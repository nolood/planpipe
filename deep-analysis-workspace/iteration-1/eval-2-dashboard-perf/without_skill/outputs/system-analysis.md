# System Analysis: Dashboard Chart Load Time Optimization

## Architecture Overview

The data path for the overview page is:

```
User Browser (React + Recharts + Apollo Client)
    |
    | Single GraphQL query: overviewPage(tenantId, from, to)
    v
Go API Server (chi router + gqlgen)
    |
    | 8 sequential ClickHouse queries + 4 sequential summary queries
    v
ClickHouse (single `events` table, ~500M rows, no materialized views)
```

Entry point: `cmd/analytics-api/main.go` -- wires up ClickHouse client, analytics service, GraphQL resolver, chi router.

## Relevant Modules and Files

### Backend (Go)

| File | Role | Performance Relevance |
|------|------|-----------------------|
| `cmd/analytics-api/main.go` | Server bootstrap | No caching layer wired in; no middleware for response compression |
| `internal/analytics/resolver.go` | GraphQL resolver | `OverviewPage()` (line 31) delegates to `Service.GetOverviewPage()` |
| `internal/analytics/service.go` | Business logic | **Primary bottleneck**: sequential chart loading loop (lines 30-37) |
| `internal/analytics/models.go` | Data models | `DefaultOverviewCharts` (line 59-68) defines the 8 charts; all data points loaded without limit |
| `internal/analytics/schema.graphql` | GraphQL schema | Single `overviewPage` query returns all charts at once |
| `internal/clickhouse/client.go` | ClickHouse data access | `ExecuteQuery()` (line 54) loads entire result set into memory; `GetSummaryData()` (line 89) runs 4 sequential queries |
| `internal/clickhouse/queries.go` | SQL templates | 8 query templates; some are inherently expensive (retention cohort with self-join) |
| `internal/cache/cache.go` | In-memory TTL cache | **Exists but is completely unused** — never imported by any other module |
| `migrations/001_analytics_tables.sql` | Table schema | No materialized views; no secondary indexes on `user_id`/`session_id` |

### Frontend (TypeScript/React)

| File | Role | Performance Relevance |
|------|------|-----------------------|
| `apps/dashboard/src/pages/Overview.tsx` | Main page component | Single `useQuery()` call; all-or-nothing rendering; no progressive loading |
| `apps/dashboard/src/api/analytics.ts` | GraphQL queries | `OVERVIEW_PAGE_QUERY` fetches everything in one request; `CHART_DATA_QUERY` exists but is unused on Overview |
| `apps/dashboard/src/components/Chart.tsx` | Chart renderer | No data downsampling; renders all SVG elements for all data points |
| `apps/dashboard/src/components/ChartGrid.tsx` | Layout | All charts rendered simultaneously; no lazy loading or virtualization |
| `apps/dashboard/package.json` | Dependencies | Apollo Client 3.9, Recharts 2.12, React 18.2 |

## Identified Performance Bottlenecks

### Bottleneck 1: Sequential Chart Query Execution (Backend — Critical)

**Location:** `internal/analytics/service.go`, `GetOverviewPage()`, lines 30-37

```go
for _, cfg := range DefaultOverviewCharts {
    chartData, err := s.loadChart(ctx, tenantID, cfg, timeRange)
    ...
}
```

Each of the 8 charts is loaded one after another. If each query takes 1-2 seconds (typical for large tenants), total time is 8-16 seconds just for chart queries. Additionally, `loadSummary()` (line 39-44) is called after all charts, adding 4 more sequential queries (`client.go` lines 93-127).

**Total sequential queries per page load: 12** (8 chart queries + 4 summary queries).

### Bottleneck 2: No Caching Layer (Backend — Critical)

**Location:** `internal/cache/cache.go` exists but is never imported or used.

The `Service` struct (`service.go` line 13) only holds a ClickHouse client — no cache reference. Every overview page load by every user triggers 12 fresh ClickHouse queries, even if another user of the same tenant loaded the same page 1 second ago.

**Observation:** `internal/cache/cache.go` implements a working in-memory TTL cache with `Get`/`Set` and automatic cleanup. It was clearly built for this purpose but never integrated.

### Bottleneck 3: Unbounded Result Sets (Backend + Frontend — High)

**Location:** `internal/clickhouse/client.go`, `ExecuteQuery()`, line 52-53 (WARNING comment) and line 63 (unbounded append).

No `LIMIT` clause on most queries. The `error_rate_over_time` query groups by minute — over a 30-day range this produces ~43,200 rows. The `events_volume` query groups by hour AND event_type — with 10 event types over 7 days, that is ~1,680 rows. All rows are materialized into memory (`[]Row` slice) before being returned.

**Frontend counterpart:** `Chart.tsx` line 50-55 transforms ALL data points, and Recharts renders every point as individual SVG elements (lines 82-151). No downsampling, no windowing.

### Bottleneck 4: All-or-Nothing Frontend Rendering (Frontend — Medium)

**Location:** `apps/dashboard/src/pages/Overview.tsx`, lines 22-32

```tsx
const { data, loading, error } = useQuery(OVERVIEW_PAGE_QUERY, { ... });
if (loading) return <div className="loading">Loading dashboard...</div>;
```

The entire page waits for ALL 8 charts + summary before rendering anything. Users see "Loading dashboard..." for the entire 8-10 seconds.

### Bottleneck 5: Ineffective Apollo Client Caching (Frontend — Medium)

**Location:** `apps/dashboard/src/pages/Overview.tsx`, lines 25-32

The query variables include `new Date().toISOString()` which changes every millisecond. Apollo's default `cache-first` policy keys on variables, so the cache is never hit on subsequent loads.

### Bottleneck 6: Expensive SQL Queries Without Pre-aggregation (Database — High)

**Location:** `internal/clickhouse/queries.go` and `migrations/001_analytics_tables.sql`

Several queries are inherently expensive on raw data:

- **`user_retention_cohort`** (lines 66-85): Self-join with subquery. Scans the events table twice. Additionally, this query has a **parameter binding bug** — it references `tenant_id = ?` and time range `BETWEEN ? AND ?` in both the subquery and the JOIN clause (4 `?` for tenant, 4 `?` for time range), but `ExecuteQuery` only passes 3 arguments. This query likely fails at runtime.

- **`active_users_over_time`** (lines 16-25): `uniq(user_id)` over 30 days requires scanning all events for the tenant and computing HyperLogLog.

- **`users_by_region`** (lines 87-97): `uniq(user_id)` grouped by region — another full scan.

The table has no materialized views (noted in `001_analytics_tables.sql` lines 20-21). All queries hit the raw `events` table with ~500M total rows.

### Bottleneck 7: Sequential Summary Queries (Backend — Medium)

**Location:** `internal/clickhouse/client.go`, `GetSummaryData()`, lines 89-130

Four separate `QueryRow` calls executed sequentially: `uniq(user_id)`, `uniq(user_id)` for active, `count()`, and `countIf/countIf` ratio. These could be combined into a single query or run in parallel.

## Change Points (Where Code Must Be Modified)

### Must Change

1. **`internal/analytics/service.go` — `GetOverviewPage()`**: Replace sequential loop with parallel execution using goroutines and `errgroup`.

2. **`internal/analytics/service.go` — `NewService()`**: Accept and use a cache instance. Wrap `loadChart()` with cache-check-then-query pattern.

3. **`cmd/analytics-api/main.go`**: Instantiate `cache.Cache` and pass it to `analytics.NewService()`.

4. **`internal/clickhouse/client.go` — `GetSummaryData()`**: Combine 4 queries into 1, or parallelize.

### Should Change

5. **`internal/clickhouse/queries.go`**: Add appropriate `LIMIT` clauses. Fix the `user_retention_cohort` parameter binding bug. Consider query-level aggregation changes (coarser time granularity for large ranges).

6. **`apps/dashboard/src/pages/Overview.tsx`**: Split single `overviewPage` query into per-chart queries (`CHART_DATA_QUERY` already exists). Render charts independently as each resolves. Add skeleton loading states.

7. **`apps/dashboard/src/components/Chart.tsx`**: Add data downsampling before passing to Recharts (e.g., LTTB algorithm or simple bucketing for datasets > 500 points).

8. **`apps/dashboard/src/api/analytics.ts`**: Adjust query variables to use rounded time ranges (e.g., floor to nearest 5-minute interval) for better cache hit rates.

### Could Change (if needed to hit target)

9. **`migrations/` — new migration**: Add ClickHouse materialized views for pre-aggregated daily/hourly metrics per tenant. This would reduce query times from seconds to milliseconds.

10. **`internal/analytics/schema.graphql`**: Add pagination support to `dataPoints` (limit/offset) or add a `maxPoints` argument.

## Dependencies

| Dependency | Version | Relevance |
|------------|---------|-----------|
| `gqlgen` | v0.17.45 | GraphQL code generation; resolver structure is generated. Schema changes require regeneration. |
| `clickhouse-go/v2` | v2.23.0 | ClickHouse driver; connection pooling configured at DSN level. |
| `chi/v5` | v5.0.12 | HTTP router; no response compression middleware currently used. |
| `zerolog` | v1.32.0 | Structured logging; performance metrics already logged (query time, row count). |
| `@apollo/client` | ^3.9.0 | GraphQL client with built-in caching; cache policy is misconfigured for this use case. |
| `recharts` | ^2.12.0 | SVG-based charting; known to struggle with >1000 data points per chart. |

## Existing Patterns Worth Noting

1. **Error tolerance pattern**: `service.go` skips failed charts with `continue` (line 33-34) rather than failing the whole page. This pattern should be preserved.

2. **Time range defaulting**: `resolver.go` defaults to 30 days if no time range specified (lines 57-68). This default is important context for query performance — 30 days is a large scan window.

3. **Query metadata**: Each chart already carries `QueryTimeMs` and `TotalRows` in its metadata. This is useful for observability and debugging — existing instrumentation that should be preserved.

4. **Logging**: Both the service layer (`service.go` line 46-49) and ClickHouse client (`client.go` line 80-83) log query timing. This existing instrumentation can be used to validate performance improvements.

5. **The `CHART_DATA_QUERY` already exists** in `analytics.ts` (line 48-65) for loading individual charts. It is not used on the Overview page but provides the GraphQL contract needed for per-chart loading.

## Technical Observations

1. **No connection pooling configuration visible**: The ClickHouse client (`client.go` line 17-33) uses default connection settings from `clickhouse.Open(opts)`. Under concurrent query load (after parallelization), connection pool sizing may need tuning.

2. **No response compression**: The chi middleware stack (`main.go` lines 42-44) uses only `Logger` and `Recoverer`. Adding gzip/brotli middleware would reduce payload transfer time, especially for large chart datasets.

3. **GraphQL schema is rigid**: The `overviewPage` query returns `[ChartData!]!` — all charts bundled. Switching to per-chart queries on the frontend is possible without schema changes (using the existing `chartData` query), but would require frontend refactoring.

4. **The `DashboardConfig` and `ChartConfig` types** (`models.go` lines 45-56) suggest a design where dashboard configurations were intended to be customizable, but currently `DefaultOverviewCharts` is hardcoded. This means any optimization can safely assume a fixed, known set of 8 charts.
