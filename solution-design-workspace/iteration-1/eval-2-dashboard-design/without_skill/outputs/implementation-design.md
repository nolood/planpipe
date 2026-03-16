# Implementation Design -- Dashboard Performance Optimization

## Overview

Reduce the analytics dashboard overview page load time from 8-10 seconds to under 2 seconds (P95). The optimization spans three layers: backend query execution, caching infrastructure, and frontend rendering. The GraphQL API contract remains unchanged.

**Current architecture:** React (Apollo/Recharts) -> GraphQL (gqlgen) -> Go service -> ClickHouse
**Current bottleneck chain:** Monolithic frontend query -> 8 sequential ClickHouse queries (2-5s each) -> unbounded result sets -> monolithic SVG rendering

---

## Phase 1: Backend Parallelization and Bug Fix

### 1.1 Parallelize chart queries with errgroup

**File:** `internal/analytics/service.go`
**Current behavior (lines 24-55):** `GetOverviewPage` iterates `DefaultOverviewCharts` in a for-loop, calling `loadChart` sequentially. Total time = sum of all query times (8 x 2-5s = 16-40s for large tenants).

**Design:**

Replace the sequential loop with `golang.org/x/sync/errgroup`. Each chart loads in its own goroutine. Results are collected into a thread-safe slice using index-based assignment (no mutex needed since each goroutine writes to its own index).

```go
func (s *Service) GetOverviewPage(ctx context.Context, tenantID string, timeRange TimeRange) (*OverviewPage, error) {
    start := time.Now()

    charts := make([]*ChartData, len(DefaultOverviewCharts))
    g, gctx := errgroup.WithContext(ctx)

    // Limit concurrency to avoid overwhelming ClickHouse
    g.SetLimit(8)

    for i, cfg := range DefaultOverviewCharts {
        i, cfg := i, cfg  // capture loop variables
        g.Go(func() error {
            chartData, err := s.loadChart(gctx, tenantID, cfg, timeRange)
            if err != nil {
                log.Error().Err(err).Str("chart", cfg.ID).Msg("failed to load chart")
                return nil  // skip-and-continue: don't fail the group
            }
            charts[i] = chartData
            return nil
        })
    }

    // Also load summary in parallel
    var summary *SummaryData
    var summaryErr error
    g.Go(func() error {
        summary, summaryErr = s.loadSummary(gctx, tenantID, timeRange)
        return nil
    })

    _ = g.Wait()  // errors are handled per-chart, never propagated

    // Collect non-nil results, preserving order
    result := make([]ChartData, 0, len(charts))
    for _, c := range charts {
        if c != nil {
            result = append(result, *c)
        }
    }

    if summaryErr != nil {
        log.Error().Err(summaryErr).Msg("failed to load summary")
        summary = &SummaryData{}
    }

    log.Info().Dur("total_time", time.Since(start)).Int("charts_loaded", len(result)).Msg("overview page loaded")

    return &OverviewPage{Charts: result, Summary: *summary}, nil
}
```

**Key design decisions:**
- `g.SetLimit(8)` caps concurrency to prevent unbounded goroutine creation. 8 is the exact chart count; this acts as documentation and a safety valve.
- Each goroutine returns `nil` even on error, preserving the skip-and-continue pattern. The errgroup context (`gctx`) is used so that if the parent context is cancelled, all in-flight queries abort.
- Index-based assignment (`charts[i]`) avoids mutex contention. The pre-allocated slice has one slot per chart.
- Summary data loads in parallel alongside charts, saving an additional sequential query.

**Expected impact:** Total backend time drops from sum(all queries) to max(slowest query). For a large tenant: ~5s (worst single query) instead of ~25s.

### 1.2 Fix retention cohort query bug

**File:** `internal/clickhouse/queries.go`, lines 66-85
**Bug:** The `user_retention_cohort` query contains 5 `?` placeholders (tenant_id + time range in the cohort subquery, tenant_id + time range in the returning subquery) but `ExecuteQuery` in `client.go:57` only passes 3 parameters (`TenantID, TimeFrom, TimeTo`).

**Fix approach:**

Option A (chosen): Modify `ExecuteQuery` to accept variadic params, and have the service layer pass the correct parameters per query. This requires knowing which queries need extra parameters.

Option B (simpler, chosen for minimal blast radius): Restructure the retention query to reuse parameters using CTEs or to reduce placeholder count.

**Chosen approach -- restructure the query:**

```sql
user_retention_cohort: `
    WITH cohort AS (
        SELECT user_id, min(timestamp) as first_seen
        FROM events
        WHERE tenant_id = ?
            AND timestamp BETWEEN ? AND ?
        GROUP BY user_id
    )
    SELECT
        toStartOfWeek(cohort.first_seen) as ts,
        map('retention_rate',
            countDistinct(returning.user_id) / countDistinct(cohort.user_id)) as values,
        map('cohort_week', toString(toStartOfWeek(cohort.first_seen))) as labels
    FROM cohort
    LEFT JOIN (
        SELECT DISTINCT user_id, toStartOfWeek(timestamp) as active_week
        FROM events
        WHERE tenant_id = ?
            AND timestamp BETWEEN ? AND ?
    ) as returning ON cohort.user_id = returning.user_id
    GROUP BY ts
    ORDER BY ts`
```

This still has 6 placeholders (`?` x 6: tenant_id, from, to for cohort; tenant_id, from, to for returning). The fix requires extending `ExecuteQuery` to accept the correct number of parameters.

**Actual fix -- extend ExecuteQuery:**

```go
// client.go -- add a new method for queries with custom param counts
func (c *Client) ExecuteQueryWithParams(ctx context.Context, query string, args ...any) ([]Row, error) {
    // Same implementation as ExecuteQuery but uses variadic args
    rows, err := c.conn.Query(ctx, query, args...)
    // ... rest is identical
}
```

Then in `service.go:loadChart`, detect the retention query and pass doubled params:

```go
var rows []Row
var err error
if cfg.Query == "user_retention_cohort" {
    rows, err = s.ch.ExecuteQueryWithParams(ctx, query,
        tenantID, tr.From, tr.To, tenantID, tr.From, tr.To)
} else {
    rows, err = s.ch.ExecuteQuery(ctx, query, clickhouse.QueryParams{
        TenantID: tenantID, TimeFrom: tr.From, TimeTo: tr.To,
    })
}
```

**Alternative (cleaner, recommended):** Extend `QueryParams` with an `ExtraArgs []any` field, and have `ExecuteQuery` append them. But the simplest approach is to add a `paramCount` field to `ChartConfig` in `models.go` and have the query registry declare how many params each query needs:

```go
type ChartConfig struct {
    ID        string
    Title     string
    ChartType string
    Query     string
    TimeRange string
    // DuplicateParams indicates the query needs tenant_id+from+to repeated
    DuplicateParams bool
}
```

Then in `loadChart`:
```go
params := []any{tenantID, tr.From, tr.To}
if cfg.DuplicateParams {
    params = append(params, tenantID, tr.From, tr.To)
}
rows, err := s.ch.ExecuteQueryWithParams(ctx, query, params...)
```

### 1.3 Reduce ClickHouse result set sizes

**File:** `internal/clickhouse/queries.go`

**Changes per query:**

| Query | Current Issue | Fix |
|-------|--------------|-----|
| `events_volume` | `toStartOfHour` over 7d = 168 hours x N event types | Change to `toStartOfFourHour` (42 buckets) or `toStartOfDay` (7 buckets). Add `LIMIT 500`. |
| `error_rate_over_time` | `toStartOfMinute` over default 30d = 43,200 rows | Change to `toStartOfFiveMinute` for ranges >24h, keep `toStartOfMinute` for 24h. Add `LIMIT 1000`. |
| `top_events_by_count` | `LIMIT 10000` | Reduce to `LIMIT 100` (bar chart only needs top entries). |
| `active_users_over_time` | `toStartOfDay` over 30d = 30 rows | Already reasonable. No change needed. |
| `avg_session_duration` | `toStartOfHour` over 7d = 168 rows | Already reasonable. No change needed. |
| `users_by_region` | Groups by region, no limit | Add `LIMIT 50`. |
| `conversion_funnel` | `toStartOfDay` over 30d = 30 rows | Already reasonable. No change needed. |
| `user_retention_cohort` | `toStartOfWeek` over 30d = ~4 rows | Already reasonable (after bug fix). |

**Specific query modifications:**

```sql
-- events_volume: coarsen from hourly to 4-hourly
"events_volume": `
    SELECT
        toStartOfInterval(timestamp, INTERVAL 4 HOUR) as ts,
        map('count', count()) as values,
        map('event_type', event_type) as labels
    FROM events
    WHERE tenant_id = ?
        AND timestamp BETWEEN ? AND ?
    GROUP BY ts, event_type
    ORDER BY ts
    LIMIT 500`,

-- error_rate_over_time: coarsen from per-minute to per-5-minutes
"error_rate_over_time": `
    SELECT
        toStartOfFiveMinute(timestamp) as ts,
        map(
            'error_rate', countIf(is_error = 1) / count(),
            'total', count()
        ) as values,
        map() as labels
    FROM events
    WHERE tenant_id = ?
        AND timestamp BETWEEN ? AND ?
    GROUP BY ts
    ORDER BY ts
    LIMIT 1000`,

-- top_events_by_count: drastically reduce limit
"top_events_by_count": `
    SELECT
        toStartOfDay(timestamp) as ts,
        map('count', count()) as values,
        map('event_type', event_type) as labels
    FROM events
    WHERE tenant_id = ?
        AND timestamp BETWEEN ? AND ?
    GROUP BY ts, event_type
    ORDER BY count() DESC
    LIMIT 100`,
```

**Expected impact:** Reduces data transfer from ClickHouse and memory allocation in Go. The `events_volume` query drops from potentially 1000+ rows to ~200. The `error_rate_over_time` drops from 43,200 to ~8,600 rows (for 30d range).

### 1.4 Configure ClickHouse connection pool

**File:** `internal/clickhouse/client.go`

**Current state (lines 17-32):** Uses `clickhouse.ParseDSN` with defaults. No explicit connection pool configuration.

**Change:** Configure the connection pool to handle parallel queries:

```go
func NewClient(dsn string) (*Client, error) {
    opts, err := clickhouse.ParseDSN(dsn)
    if err != nil {
        return nil, fmt.Errorf("invalid ClickHouse DSN: %w", err)
    }

    // Configure pool for parallel query execution
    // 8 charts + 1 summary + headroom = 12 max connections
    opts.MaxOpenConns = 12
    opts.MaxIdleConns = 6
    opts.ConnMaxLifetime = 10 * time.Minute
    opts.DialTimeout = 5 * time.Second

    conn, err := clickhouse.Open(opts)
    // ... rest unchanged
}
```

**Rationale:** With 8 parallel chart queries + 1 summary query per request, and potentially multiple concurrent users, the pool needs at least 9 connections per request. Setting `MaxOpenConns = 12` provides headroom. `MaxIdleConns = 6` keeps half the pool warm for subsequent requests.

---

## Phase 2: Caching Layer

### 2.1 Upgrade the cache implementation

**File:** `internal/cache/cache.go`

**Current limitations:**
1. Single global TTL for all items
2. Naive eviction (removes first expired item, not LRU)
3. Silently drops new entries when full and nothing is expired

**Design -- per-key TTL with proper LRU eviction:**

```go
type Cache struct {
    mu       sync.RWMutex
    items    map[string]*cacheItem
    order    []string          // LRU order tracking (most recent at end)
    maxSize  int
}

type cacheItem struct {
    value     any
    expiresAt time.Time
}

func New(maxSize int) *Cache {
    c := &Cache{
        items:   make(map[string]*cacheItem, maxSize),
        order:   make([]string, 0, maxSize),
        maxSize: maxSize,
    }
    go c.cleanup()
    return c
}

// Set stores a value with a specific TTL for this key
func (c *Cache) Set(key string, value any, ttl time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()

    // If key already exists, update it
    if _, exists := c.items[key]; exists {
        c.items[key] = &cacheItem{value: value, expiresAt: time.Now().Add(ttl)}
        c.touchLocked(key)
        return
    }

    // Evict if at capacity
    for len(c.items) >= c.maxSize && len(c.order) > 0 {
        // Remove least recently used (front of order slice)
        evictKey := c.order[0]
        c.order = c.order[1:]
        delete(c.items, evictKey)
    }

    c.items[key] = &cacheItem{value: value, expiresAt: time.Now().Add(ttl)}
    c.order = append(c.order, key)
}
```

**Per-chart TTL configuration:**

```go
// Defined in analytics/service.go or a new config file
var chartTTLs = map[string]time.Duration{
    "active-users":       5 * time.Minute,    // Daily aggregation, slow-changing
    "events-volume":      2 * time.Minute,    // Hourly buckets, moderate freshness need
    "top-events":         5 * time.Minute,    // Ranking, slow-changing
    "conversion-funnel":  10 * time.Minute,   // Daily aggregation, very slow-changing
    "user-retention":     30 * time.Minute,   // Weekly cohorts, rarely changes
    "geo-distribution":   10 * time.Minute,   // Regional data, slow-changing
    "session-duration":   5 * time.Minute,    // Hourly aggregation, moderate
    "error-rate":         1 * time.Minute,    // Operational chart, needs freshest data
}
```

**Rationale for TTL choices:**
- Operational charts (error-rate) get short TTLs (1 min) because operators need near-real-time data.
- Trend charts (active-users, conversion-funnel) get longer TTLs (5-10 min) because daily/weekly aggregations change slowly.
- Retention cohort gets the longest TTL (30 min) because it is a weekly aggregation and the most expensive query.

### 2.2 Integrate singleflight for cache miss deduplication

**File:** `internal/analytics/service.go`

**Design:** Wrap cache miss handling with `sync/singleflight` to prevent thundering herd when many concurrent requests for the same tenant hit an empty cache.

```go
type Service struct {
    ch    *clickhouse.Client
    cache *cache.Cache
    sf    singleflight.Group
}

func NewService(ch *clickhouse.Client, cache *cache.Cache) *Service {
    return &Service{ch: ch, cache: cache}
}

func (s *Service) loadChart(ctx context.Context, tenantID string, cfg ChartConfig, tr TimeRange) (*ChartData, error) {
    // Build cache key from tenant + chart + quantized time range
    cacheKey := fmt.Sprintf("chart:%s:%s:%d:%d",
        tenantID, cfg.ID,
        tr.From.Unix()/60,   // Quantize to minute
        tr.To.Unix()/60)

    // Check cache first
    if cached, ok := s.cache.Get(cacheKey); ok {
        return cached.(*ChartData), nil
    }

    // singleflight: if another goroutine is already loading this exact chart
    // for this tenant and time range, wait for its result instead of querying again
    result, err, _ := s.sf.Do(cacheKey, func() (any, error) {
        // Double-check cache (another goroutine may have populated it)
        if cached, ok := s.cache.Get(cacheKey); ok {
            return cached, nil
        }

        chartData, err := s.loadChartFromDB(ctx, tenantID, cfg, tr)
        if err != nil {
            return nil, err
        }

        // Cache the result with chart-specific TTL
        ttl := chartTTLs[cfg.ID]
        if ttl == 0 {
            ttl = 5 * time.Minute  // default
        }
        s.cache.Set(cacheKey, chartData, ttl)

        return chartData, nil
    })

    if err != nil {
        return nil, err
    }
    return result.(*ChartData), nil
}

// loadChartFromDB is the extracted original loadChart logic
func (s *Service) loadChartFromDB(ctx context.Context, tenantID string, cfg ChartConfig, tr TimeRange) (*ChartData, error) {
    // ... existing loadChart implementation (query execution + row transformation)
}
```

**Cache key design:**
- Format: `chart:{tenantID}:{chartID}:{fromMinute}:{toMinute}`
- Time is quantized to the minute, matching the frontend quantization (see Phase 3).
- This ensures that requests within the same minute share cache entries.

### 2.3 Wire cache into the application

**File:** `cmd/analytics-api/main.go`

```go
import "github.com/acme/analytics/internal/cache"

func main() {
    // ... existing setup ...

    // Initialize cache: 1000 entries max
    // Individual items have per-chart TTLs set during insertion
    queryCache := cache.New(1000)

    analyticsSvc := analytics.NewService(chClient, queryCache)
    // ... rest unchanged
}
```

**Cache sizing rationale:** 1000 entries supports ~125 tenants with 8 charts each (125 x 8 = 1000). For 50k DAU, the number of distinct tenants actively viewing dashboards simultaneously is likely much smaller. If needed, this can be increased. Each entry is a `*ChartData` struct; memory footprint depends on data point count but with downsampled results (max 500 points), each entry is roughly 50-100KB, so total cache memory is ~50-100MB.

---

## Phase 3: Frontend Optimization

### 3.1 Fix Apollo cache-busting bug

**File:** `apps/dashboard/src/pages/Overview.tsx`, lines 26-27

**Current code:**
```typescript
from: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString(),
to: new Date().toISOString(),
```

This creates a unique timestamp every millisecond, so Apollo's cache key (which includes query variables) never matches a previous request.

**Fix -- quantize to 5-minute boundaries:**

```typescript
function quantizeTime(date: Date, intervalMs: number): string {
    const quantized = new Date(Math.floor(date.getTime() / intervalMs) * intervalMs);
    return quantized.toISOString();
}

const FIVE_MINUTES = 5 * 60 * 1000;

// In the component:
const from = useMemo(
    () => quantizeTime(new Date(Date.now() - 30 * 24 * 60 * 60 * 1000), FIVE_MINUTES),
    [Math.floor(Date.now() / FIVE_MINUTES)]  // recalculate every 5 minutes
);
const to = useMemo(
    () => quantizeTime(new Date(), FIVE_MINUTES),
    [Math.floor(Date.now() / FIVE_MINUTES)]
);
```

**Impact:** Apollo cache hits for subsequent renders within the same 5-minute window. Combined with backend caching (which also quantizes to minutes), this creates a coherent caching strategy across the stack.

### 3.2 Split monolithic query into per-chart queries

**File:** `apps/dashboard/src/pages/Overview.tsx`
**File:** `apps/dashboard/src/api/analytics.ts`

**Design:** Replace the single `OVERVIEW_PAGE_QUERY` with individual `CHART_DATA_QUERY` calls per chart, plus a separate `SUMMARY_QUERY` call. This enables progressive loading -- each chart renders as soon as its data arrives.

**New Overview.tsx structure:**

```typescript
import { useQuery } from '@apollo/client';
import { CHART_DATA_QUERY, SUMMARY_QUERY } from '../api/analytics';
import { ChartGrid } from '../components/ChartGrid';
import { ChartWithLoading } from '../components/ChartWithLoading';

const CHART_IDS = [
    'active-users', 'events-volume', 'top-events', 'conversion-funnel',
    'user-retention', 'geo-distribution', 'session-duration', 'error-rate',
];

// Above-fold charts (first row of 2-column grid)
const ABOVE_FOLD = new Set(['active-users', 'events-volume', 'top-events', 'conversion-funnel']);

export function Overview({ tenantId }: OverviewProps) {
    const from = useMemo(
        () => quantizeTime(new Date(Date.now() - 30 * 24 * 60 * 60 * 1000), FIVE_MINUTES),
        [Math.floor(Date.now() / FIVE_MINUTES)]
    );
    const to = useMemo(
        () => quantizeTime(new Date(), FIVE_MINUTES),
        [Math.floor(Date.now() / FIVE_MINUTES)]
    );

    return (
        <div className="overview-page">
            <SummaryBar tenantId={tenantId} from={from} to={to} />
            <ChartGrid>
                {CHART_IDS.map(chartId => (
                    <ChartWithLoading
                        key={chartId}
                        tenantId={tenantId}
                        chartId={chartId}
                        from={from}
                        to={to}
                        lazy={!ABOVE_FOLD.has(chartId)}
                    />
                ))}
            </ChartGrid>
        </div>
    );
}

function SummaryBar({ tenantId, from, to }: { tenantId: string; from: string; to: string }) {
    const { data, loading } = useQuery(SUMMARY_QUERY, {
        variables: { tenantId, from, to },
    });

    if (loading) return <div className="summary-bar skeleton" />;

    const { summary } = data;
    return (
        <div className="summary-bar">
            {/* ... same summary rendering as before ... */}
        </div>
    );
}
```

**New analytics.ts additions:**

```typescript
export const SUMMARY_QUERY = gql`
    query Summary($tenantId: ID!, $from: DateTime, $to: DateTime) {
        summary(tenantId: $tenantId, from: $from, to: $to) {
            totalUsers
            activeUsers
            totalEvents
            conversionRate
        }
    }
`;
```

**Note:** `CHART_DATA_QUERY` already exists in `analytics.ts` (lines 48-65) and does not need to be created.

### 3.3 Create ChartWithLoading component for progressive rendering

**New component: `apps/dashboard/src/components/ChartWithLoading.tsx`**

This wrapper component handles per-chart data fetching, loading states, error states, and lazy loading via IntersectionObserver.

```typescript
import { useQuery } from '@apollo/client';
import { useRef, useState, useEffect } from 'react';
import { CHART_DATA_QUERY } from '../api/analytics';
import { Chart } from './Chart';

interface ChartWithLoadingProps {
    tenantId: string;
    chartId: string;
    from: string;
    to: string;
    lazy: boolean;
}

export function ChartWithLoading({ tenantId, chartId, from, to, lazy }: ChartWithLoadingProps) {
    const [isVisible, setIsVisible] = useState(!lazy);
    const containerRef = useRef<HTMLDivElement>(null);

    // IntersectionObserver for lazy loading
    useEffect(() => {
        if (!lazy || !containerRef.current) return;

        const observer = new IntersectionObserver(
            ([entry]) => {
                if (entry.isIntersecting) {
                    setIsVisible(true);
                    observer.disconnect();
                }
            },
            { rootMargin: '200px' }  // Start loading 200px before visible
        );

        observer.observe(containerRef.current);
        return () => observer.disconnect();
    }, [lazy]);

    if (!isVisible) {
        return (
            <div ref={containerRef} className="chart-container chart-placeholder">
                <div className="chart-skeleton" style={{ height: 350 }} />
            </div>
        );
    }

    return <ChartFetcher tenantId={tenantId} chartId={chartId} from={from} to={to} />;
}

function ChartFetcher({ tenantId, chartId, from, to }: Omit<ChartWithLoadingProps, 'lazy'>) {
    const { data, loading, error } = useQuery(CHART_DATA_QUERY, {
        variables: { tenantId, chartId, from, to },
    });

    if (loading) {
        return (
            <div className="chart-container">
                <div className="chart-skeleton" style={{ height: 350 }}>
                    Loading chart...
                </div>
            </div>
        );
    }

    if (error) {
        return (
            <div className="chart-container chart-error">
                <p>Failed to load chart: {error.message}</p>
            </div>
        );
    }

    const { chartData } = data;
    return (
        <Chart
            chartId={chartData.chartId}
            title={chartData.title}
            type={chartData.chartType}
            dataPoints={chartData.dataPoints}
            metadata={chartData.metadata}
        />
    );
}
```

**Key design points:**
- `lazy={true}` charts use IntersectionObserver with a 200px rootMargin to start loading before the user scrolls to them.
- Above-fold charts (`lazy={false}`) start loading immediately.
- Each chart has independent loading/error states -- a failed chart does not block others.
- Skeleton placeholders maintain layout stability during loading.

### 3.4 Data downsampling before chart rendering

**File:** `apps/dashboard/src/components/Chart.tsx`

**Design:** Add a `downsample` function that reduces data points to a configurable maximum (default 500) using the Largest-Triangle-Three-Buckets (LTTB) algorithm for time-series data, or simple bucket averaging for other chart types.

```typescript
const MAX_POINTS: Record<string, number> = {
    line: 500,
    area: 500,
    bar: 100,
    pie: 50,
    default: 500,
};

function downsampleData(dataPoints: DataPoint[], chartType: string): DataPoint[] {
    const maxPoints = MAX_POINTS[chartType] || MAX_POINTS.default;

    if (dataPoints.length <= maxPoints) {
        return dataPoints;
    }

    // For time-series charts (line, area): use bucket averaging
    // to preserve visual shape while reducing point count
    const bucketSize = Math.ceil(dataPoints.length / maxPoints);
    const downsampled: DataPoint[] = [];

    for (let i = 0; i < dataPoints.length; i += bucketSize) {
        const bucket = dataPoints.slice(i, i + bucketSize);

        // Use the middle timestamp of the bucket
        const midIndex = Math.floor(bucket.length / 2);
        const midPoint = bucket[midIndex];

        // Average the numeric values across the bucket
        const avgValues: Record<string, number> = {};
        const firstValues = bucket[0].values;
        for (const key of Object.keys(firstValues)) {
            const sum = bucket.reduce((acc, dp) => acc + (dp.values[key] as number || 0), 0);
            avgValues[key] = sum / bucket.length;
        }

        downsampled.push({
            timestamp: midPoint.timestamp,
            values: avgValues,
            labels: midPoint.labels,
        });
    }

    return downsampled;
}
```

**Integration into Chart component:**

```typescript
export function Chart({ chartId, title, type, dataPoints, metadata }: ChartProps) {
    // Downsample before transforming for Recharts
    const sampledPoints = useMemo(
        () => downsampleData(dataPoints, type),
        [dataPoints, type]
    );

    const chartData = useMemo(
        () => sampledPoints.map((dp) => ({
            time: format(new Date(dp.timestamp), 'MMM dd HH:mm'),
            timestamp: dp.timestamp,
            ...dp.values,
            ...(dp.labels || {}),
        })),
        [sampledPoints]
    );

    // ... rest unchanged, but now operates on sampledPoints/chartData
}
```

**Key design points:**
- `useMemo` prevents re-downsampling on every render.
- Different max points per chart type: bar charts need fewer points than line charts.
- Bucket averaging preserves the visual trend while being computationally cheap.
- The metadata still shows `totalRows` from the server, so users can see the actual data volume.

### 3.5 Optimize date formatting in Chart component

**File:** `apps/dashboard/src/components/Chart.tsx`, line 51

**Current issue:** `format(new Date(dp.timestamp), 'MMM dd HH:mm')` is called for every data point on every render.

**Fix:** The `useMemo` from 3.4 already addresses the re-render issue. Additionally, for XAxis tick formatting, delegate to Recharts' `tickFormatter` instead of pre-formatting all points:

```typescript
<XAxis
    dataKey="timestamp"
    tickFormatter={(ts: string) => format(new Date(ts), 'MMM dd HH:mm')}
    interval="preserveStartEnd"
/>
```

This way, only visible tick labels are formatted (typically 5-10 ticks), not all data points.

---

## Phase 4: Testing

### 4.1 Backend tests

**New files:**
- `internal/analytics/service_test.go` -- Test parallel execution, skip-and-continue behavior, cache integration, singleflight deduplication.
- `internal/clickhouse/queries_test.go` -- Test query parameter counts match placeholder counts for all queries.
- `internal/cache/cache_test.go` -- Test per-key TTL, LRU eviction, concurrent access, cleanup goroutine.

**Critical test cases:**

```go
// service_test.go
func TestGetOverviewPage_ParallelExecution(t *testing.T) {
    // Verify all 8 charts load, total time is ~max(individual) not sum
}

func TestGetOverviewPage_SkipFailedCharts(t *testing.T) {
    // One chart query fails -> other 7 still returned
}

func TestLoadChart_CacheHit(t *testing.T) {
    // Second call returns cached result, no ClickHouse query
}

func TestLoadChart_Singleflight(t *testing.T) {
    // 10 concurrent calls for same key -> 1 ClickHouse query
}

func TestGetOverviewPage_RaceDetector(t *testing.T) {
    // Run with -race flag, verify no data races
}

// queries_test.go
func TestQueryParamCounts(t *testing.T) {
    // Count ? placeholders in each query, verify they match expected param count
    // This catches the retention cohort bug class
}
```

### 4.2 Frontend tests

**New files:**
- `apps/dashboard/src/__tests__/Overview.test.tsx` -- Test progressive loading behavior, skeleton states, error handling.
- `apps/dashboard/src/__tests__/Chart.test.tsx` -- Test downsampling output, edge cases (empty data, single point).

---

## Execution Order

### Sprint 1 (Backend foundation -- 3-4 days)
1. Fix retention cohort query bug (1.2) -- unblocks the chart for the first time
2. Add `ExecuteQueryWithParams` to ClickHouse client
3. Configure connection pool (1.4)
4. Parallelize `GetOverviewPage` with errgroup (1.1)
5. Write backend tests for parallelization
6. **Checkpoint:** Backend serves overview page in ~5s instead of ~25s for large tenants

### Sprint 2 (Caching layer -- 2-3 days)
1. Upgrade cache implementation with per-key TTL and LRU (2.1)
2. Integrate singleflight (2.2)
3. Wire cache into application (2.3)
4. Write cache tests
5. **Checkpoint:** Cached responses return in <100ms; first load ~5s, subsequent loads ~100ms

### Sprint 3 (Query optimization -- 1-2 days)
1. Coarsen time buckets in ClickHouse queries (1.3)
2. Add LIMIT clauses to unbounded queries
3. Validate query correctness
4. **Checkpoint:** First-load time drops to ~2-3s due to reduced data volume

### Sprint 4 (Frontend optimization -- 3-4 days)
1. Fix Apollo cache-busting (3.1)
2. Split monolithic query into per-chart queries (3.2)
3. Create ChartWithLoading component (3.3)
4. Add data downsampling (3.4)
5. Optimize date formatting (3.5)
6. Write frontend tests
7. **Checkpoint:** First chart visible in <1s; all charts in <2s; subsequent loads instant

### Sprint 5 (Validation -- 1-2 days)
1. End-to-end load testing against representative data
2. Verify P95 < 2s target
3. Verify race detector passes
4. Monitor cache hit rates, ClickHouse query load

---

## Performance Budget

| Phase | Estimated Time Contribution | Notes |
|-------|---------------------------|-------|
| ClickHouse query (parallel, cached miss) | ~2-5s (max single query) -> 0.5-2s with query optimization | Parallelization converts sum to max; query changes reduce individual times |
| Cache hit | ~1ms | In-memory lookup |
| Network (backend to frontend) | ~50-100ms | Reduced payload with downsampled data |
| Frontend rendering (per chart) | ~50-100ms | 500 points max instead of 10k+ |
| Total (cache miss, P95) | < 2s | Target met |
| Total (cache hit) | < 500ms | Significant improvement for repeat loads |

---

## Rollback Strategy

Each phase is independently rollbackable:

1. **Backend parallelization:** Revert `GetOverviewPage` to sequential loop. The API contract is unchanged.
2. **Caching:** Remove cache from `NewService` constructor. The service falls back to direct ClickHouse queries.
3. **Query changes:** Revert SQL strings in `queries.go`. No schema changes involved.
4. **Frontend changes:** Revert `Overview.tsx` to use `OVERVIEW_PAGE_QUERY`. The `overviewPage` GraphQL query remains functional.

Feature flags can be added for cache enable/disable and parallel vs. sequential execution, but given the zero-test-coverage baseline, a phased deployment with monitoring is preferred over feature flags.
