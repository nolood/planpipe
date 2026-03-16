# Change Map

> Task: Optimize analytics dashboard overview page from 8-10s to <2s P95 load time
> Total files affected: 7 modified, 5 new, 0 deleted

## Files to Modify

| File | Module | Change Description | Scope | Dependencies |
|------|--------|-------------------|-------|-------------|
| `internal/analytics/service.go` | Analytics Service | Add `cache` and `sfGroup` fields to `Service` struct; update `NewService` to accept cache param; rewrite `GetOverviewPage` with errgroup parallel execution; wrap `loadChart` with cache-check + singleflight dedup + cache-store; add `chartCacheTTLs` map for per-chart TTLs | large | `internal/cache/cache.go` (must be enhanced first), `internal/clickhouse/client.go` (variadic params must be done first) |
| `internal/clickhouse/client.go` | ClickHouse Client | Change `ExecuteQuery` to accept variadic `args ...any` instead of fixed `QueryParams` struct for query parameters. Update the `conn.Query` call at line 57 to pass variadic args. | small | none |
| `internal/clickhouse/queries.go` | ClickHouse Client | Fix `user_retention_cohort` query: update calling code to pass 5 params (tenantID, timeFrom, timeTo, tenantID, timeFrom, timeTo → the subquery needs its own set). Change `error_rate_over_time` from `toStartOfMinute` to `toStartOfFiveMinute`. Reduce `top_events_by_count` LIMIT from 10000 to 500. | medium | none |
| `internal/cache/cache.go` | Cache Utility | Add `lastAccessed time.Time` field to `cacheItem` struct. Update `Get` to set `lastAccessed`. Add `SetWithTTL(key string, value any, ttl time.Duration)` method. Replace naive eviction (remove first expired) with LRU eviction (remove item with oldest `lastAccessed`). | medium | none |
| `cmd/analytics-api/main.go` | API Entry Point | Import `internal/cache` package. Create cache instance: `cache.New(5*time.Minute, 1000)`. Pass cache to `analytics.NewService(chClient, analyticsCache)`. Configure ClickHouse connection pool via DSN parameters. | small | `internal/cache/cache.go`, `internal/analytics/service.go` |
| `apps/dashboard/src/pages/Overview.tsx` | Dashboard Frontend | Replace single `useQuery(OVERVIEW_PAGE_QUERY)` with: (1) `useSummary` hook for summary data, (2) per-chart `useChartData` hooks for above-fold charts, (3) `LazyChart` wrappers for below-fold charts. Add `ChartSkeleton` for loading states per chart. Remove the monolithic loading/error blocks. | large | `src/hooks/useChartData.ts`, `src/components/LazyChart.tsx`, `src/components/ChartSkeleton.tsx`, `src/utils/quantizeTime.ts` |
| `apps/dashboard/src/components/Chart.tsx` | Dashboard Frontend | Import `downsample` from `utils/downsample`. At line 50, before the `.map()` transformation, call `downsample(dataPoints, 500)` to reduce data points. Pass downsampled data to Recharts. | small | `src/utils/downsample.ts` |

## Files to Create

| File | Module | Purpose | Template/Pattern |
|------|--------|---------|-----------------|
| `apps/dashboard/src/utils/downsample.ts` | Dashboard Frontend | LTTB (Largest-Triangle-Three-Buckets) downsampling function. Takes array of DataPoints and max count, returns reduced array preserving visual shape. | Self-contained utility function, no existing pattern to follow |
| `apps/dashboard/src/utils/quantizeTime.ts` | Dashboard Frontend | Quantizes Date timestamps to 5-minute boundaries. Used by `useChartData` to create stable Apollo cache keys. `quantize(date: Date, intervalMs: number) => Date` | Self-contained utility function |
| `apps/dashboard/src/hooks/useChartData.ts` | Dashboard Frontend | Custom React hook wrapping Apollo `useQuery(CHART_DATA_QUERY)` with quantized time range variables and per-chart loading/error state. | Follows the existing `useQuery` pattern in `Overview.tsx:22-32` but uses `CHART_DATA_QUERY` from `analytics.ts:48-65` |
| `apps/dashboard/src/components/ChartSkeleton.tsx` | Dashboard Frontend | Skeleton/placeholder component for chart loading state. Renders a grey pulsing rectangle matching chart container dimensions (width: 100%, height: 300px). | Follows existing component structure in `Chart.tsx` (functional component with props) |
| `apps/dashboard/src/components/LazyChart.tsx` | Dashboard Frontend | IntersectionObserver wrapper component that defers rendering its children until the container scrolls into viewport. Uses `useRef` + `useEffect` with `IntersectionObserver` API. | No existing pattern — new component. Uses native browser IntersectionObserver API. |

## Files to Delete

No files to delete.

## Interfaces Changed

| Interface | Location | Current Signature | New Signature | Consumers |
|-----------|----------|------------------|---------------|-----------|
| `NewService` | `internal/analytics/service.go:17` | `func NewService(ch *clickhouse.Client) *Service` | `func NewService(ch *clickhouse.Client, c *cache.Cache) *Service` | `cmd/analytics-api/main.go:35` |
| `ExecuteQuery` | `internal/clickhouse/client.go:54` | `func (c *Client) ExecuteQuery(ctx context.Context, query string, params QueryParams) ([]Row, error)` | `func (c *Client) ExecuteQuery(ctx context.Context, query string, args ...any) ([]Row, error)` | `internal/analytics/service.go:68-72` (loadChart) |

## Data / Schema Changes

| What | Type | Description | Migration Needed? |
|------|------|-------------|-------------------|
| `error_rate_over_time` query granularity | modify (query only) | Time bucket changes from 1-minute to 5-minute intervals. Reduces max rows from 1440 to 288 for a 24h range. | no |
| `top_events_by_count` result limit | modify (query only) | LIMIT reduced from 10,000 to 500. Frontend can only display ~500 bars meaningfully. | no |
| In-memory cache data | add (runtime only) | Cached `ChartData` objects stored in memory. No persistence, lost on restart. Max 1000 entries, LRU eviction. | no |

No ClickHouse table schema changes. No persistent data migrations.

## Configuration Changes

| What | Location | Description |
|------|----------|-------------|
| ClickHouse connection pool | `cmd/analytics-api/main.go` (DSN params) | Add `?max_open_conns=20&max_idle_conns=10` to ClickHouse DSN to handle parallel query load |
| Cache settings | `cmd/analytics-api/main.go` | New cache creation: default TTL 5m, max size 1000 entries. Consider making configurable via env vars (`CACHE_TTL`, `CACHE_MAX_SIZE`) |
| Chart cache TTLs | `internal/analytics/service.go` | New `chartCacheTTLs` map defining per-chart TTLs: error_rate=1m, events_volume=5m, active_users=5m, retention=15m, etc. |

## Change Dependency Order

```
[client.go: variadic params] → [queries.go: bug fix + query optimization] → [service.go: errgroup + cache + singleflight]
[cache.go: LRU + SetWithTTL] → [service.go: errgroup + cache + singleflight]
[service.go] → [main.go: wire cache + pool config]

[downsample.ts + quantizeTime.ts] → [useChartData.ts + ChartSkeleton.tsx + LazyChart.tsx] → [Overview.tsx + Chart.tsx]

Backend and frontend tracks are independent — can proceed in parallel.
```
