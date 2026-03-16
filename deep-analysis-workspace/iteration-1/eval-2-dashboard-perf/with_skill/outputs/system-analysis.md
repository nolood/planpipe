# Codebase / System Analysis

## Relevant Modules

### Analytics API Entry Point
- **Path:** `cmd/analytics-api/main.go`
- **Purpose:** HTTP server setup. Creates the ClickHouse client, wires it into the analytics service, and mounts the GraphQL handler on `/graphql`. Uses chi router with Logger and Recoverer middleware, plus a basic CORS handler (allows all origins).
- **Key files:** [`main.go` -- server bootstrap, dependency wiring]
- **Relevance to task:** This is where the cache would need to be instantiated and injected into the service layer. Currently, the `internal/cache` package is not imported here at all. The service is constructed as `analytics.NewService(chClient)` with no cache parameter.

### Analytics Service Layer
- **Path:** `internal/analytics/`
- **Purpose:** Core business logic for the analytics dashboard. Contains the GraphQL resolvers, service layer, data models, and GraphQL schema.
- **Key files:**
  - [`service.go` -- `GetOverviewPage()` method: the primary bottleneck. Iterates `DefaultOverviewCharts` sequentially in a `for` loop (line 30), calling `loadChart()` for each. Also calls `loadSummary()` sequentially after all charts. No parallelism, no caching.]
  - [`resolver.go` -- GraphQL resolver. `OverviewPage()` resolver delegates directly to `svc.GetOverviewPage()`. Also exposes `ChartData()` for single-chart loading and `Summary()` for summary-only queries -- but the frontend doesn't use these for the overview page.]
  - [`models.go` -- Data types: `ChartData`, `DataPoint`, `OverviewPage`, `SummaryData`, `ChartConfig`. Defines `DefaultOverviewCharts` -- a hardcoded slice of 8 `ChartConfig` structs, each with an ID, title, chart type, query name, and time range.]
  - [`schema.graphql` -- GraphQL schema defining `Query.overviewPage`, `Query.chartData`, `Query.summary` operations and their types.]
- **Relevance to task:** This is the central module where most backend changes would occur. The sequential loop in `GetOverviewPage()` is the primary backend bottleneck.

### ClickHouse Data Layer
- **Path:** `internal/clickhouse/`
- **Purpose:** ClickHouse database client and query definitions.
- **Key files:**
  - [`client.go` -- `Client` struct wrapping a ClickHouse connection. `ExecuteQuery()` method (line 54): runs a named query and returns all rows as `[]Row`. WARNING comment on line 52: "No result size limit. Large time ranges can return 100k+ rows." All rows are loaded into memory. `GetSummaryData()` method (line 89): runs 4 separate sequential queries for summary stats (total users, active users, total events, conversion rate) -- each is an independent `QueryRow` call.]
  - [`queries.go` -- Map of 8 named SQL queries, one per chart type. Key performance observations per query:]
    - `active_users_over_time`: Groups by day, uses `uniq(user_id)` -- moderate cost
    - `events_volume`: Groups by hour AND event_type -- produces many rows (hours * event_types)
    - `top_events_by_count`: Has `LIMIT 10000` -- can return up to 10,000 rows
    - `conversion_funnel`: Groups by day with 4 conditional counts -- moderate
    - `user_retention_cohort`: Self-join subquery pattern -- most expensive query. Joins a "cohort" subquery (min timestamp per user) with a "returning" subquery (distinct user+week). Uses the ClickHouse placeholder `?` for tenant_id and time range in BOTH subqueries, meaning it passes 5 parameters but the current `ExecuteQuery` call only passes 3 (`tenantID, timeFrom, timeTo`). **This query will fail or produce wrong results at runtime** because the `LEFT JOIN` subquery also needs tenant_id and time range parameters but only receives the first 3.
    - `users_by_region`: Groups by region -- moderate, but `uniq(user_id)` appears both in SELECT and ORDER BY
    - `avg_session_duration`: Filters by `event_type = 'session_end'` -- efficient due to primary key including event_type
    - `error_rate_over_time`: Groups by minute -- can produce many rows (1440 per day)
- **Relevance to task:** Contains the actual queries that dominate load time. Optimizations here (adding LIMIT, server-side downsampling, query rewriting) directly reduce ClickHouse query time.

### Cache Utility
- **Path:** `internal/cache/cache.go`
- **Purpose:** In-memory TTL cache with `sync.RWMutex` for concurrency safety. Supports `Get(key)` and `Set(key, value)`. Has a simple eviction strategy: when at `maxSize`, removes the first expired item found. Background goroutine cleans up expired items on a ticker matching the TTL.
- **Key files:** [`cache.go` -- complete implementation, ~79 lines]
- **Relevance to task:** This is an existing, unused cache that was explicitly built for this purpose (TODO comment: "Consider using this for caching ClickHouse query results"). It is functional and ready to wire into the service layer. However, its eviction strategy is naive (only removes one expired item when full, doesn't handle the case where no items are expired but cache is full).

### Database Schema
- **Path:** `migrations/001_analytics_tables.sql`
- **Purpose:** ClickHouse table definition for the `events` table.
- **Key details:**
  - Engine: `MergeTree()`
  - Partition key: `(tenant_id, toYYYYMM(timestamp))`
  - Order key (primary key): `(tenant_id, event_type, timestamp)`
  - `event_type` is `LowCardinality(String)` -- good for ClickHouse filtering
  - `region` is `LowCardinality(String)` -- good
  - No secondary indexes on `user_id` or `session_id` -- queries filtering by `user_id` require full partition scans
  - No materialized views for pre-aggregated data
  - ~500M rows total across all tenants
- **Relevance to task:** The schema design directly impacts query performance. The primary key supports queries that filter by `tenant_id` + `event_type` + `timestamp` (which most chart queries do). Missing materialized views mean every query scans raw data.

### Dashboard Frontend
- **Path:** `apps/dashboard/`
- **Purpose:** React + Recharts dashboard application, built with Vite.
- **Key files:**
  - [`src/pages/Overview.tsx` -- Main overview page component. Uses a single `useQuery(OVERVIEW_PAGE_QUERY)` call that fetches all 8 charts + summary at once. Shows a loading spinner ("Loading dashboard...") until ALL data arrives. No progressive loading, no skeleton states. Time range variables use `new Date()` which means the cache key changes on every call (line 26-27), making Apollo's default cache-first strategy ineffective.]
  - [`src/components/Chart.tsx` -- Renders a single chart using Recharts. Transforms all data points into Recharts format (line 50-55), extracting value keys from the first data point. WARNING comment (line 42): "No data downsampling or virtualization. For charts with 10k+ data points, recharts renders every single SVG element, causing significant rendering time (2-5 seconds for large datasets)." Dots are disabled (`dot={false}`) as a minor optimization.]
  - [`src/components/ChartGrid.tsx` -- CSS grid layout (2 columns on desktop). NOTE comment (line 13): "All charts are rendered simultaneously -- no lazy loading or virtualization."]
  - [`src/api/analytics.ts` -- GraphQL query definitions. `OVERVIEW_PAGE_QUERY` fetches everything. `CHART_DATA_QUERY` exists for single-chart loading but is not used by the Overview page.]
- **Relevance to task:** Frontend changes are needed for progressive loading and data downsampling. The existing `CHART_DATA_QUERY` provides an alternative approach -- the frontend could fetch charts individually rather than all at once.

## Change Points

| Location | What Changes | Scope | Confidence |
|----------|-------------|-------|------------|
| `internal/analytics/service.go:GetOverviewPage` | Sequential `for` loop (line 30) must be changed to parallel execution using goroutines + `sync.WaitGroup` or `errgroup.Group`. This is the single highest-impact change. | medium | high -- read the code |
| `internal/analytics/service.go:loadChart` | May need to accept a result limit parameter to cap rows returned per chart. Currently passes all rows through without limiting. | small | high -- read the code |
| `internal/clickhouse/client.go:ExecuteQuery` | Needs a row limit parameter or result truncation to prevent 100k+ row results. The function currently loads everything into memory. | small | high -- read the code |
| `internal/clickhouse/client.go:GetSummaryData` | 4 sequential queries should be parallelized. Each is independent. | medium | high -- read the code |
| `internal/clickhouse/queries.go:queries["user_retention_cohort"]` | Has a parameter binding bug -- uses 5 `?` placeholders but `ExecuteQuery` only passes 3 params. Needs either query restructuring or a different parameter passing approach. | medium | high -- read the code |
| `internal/clickhouse/queries.go:queries["top_events_by_count"]` | `LIMIT 10000` is too high for dashboard display. Should be reduced or server-side aggregation added. | small | high -- read the code |
| `internal/clickhouse/queries.go:queries["events_volume"]` | Grouping by hour produces excessive rows for 7-day ranges (168 rows * N event types). Consider grouping by day or limiting event types. | small | medium -- inferred from query structure |
| `internal/clickhouse/queries.go:queries["error_rate_over_time"]` | Grouping by minute for 24h produces up to 1440 rows. Consider coarser granularity or downsampling. | small | medium -- inferred from query structure |
| `internal/analytics/service.go` (new method) | Cache integration: wrap `loadChart` with cache lookup/store using `internal/cache`. Cache key would be `tenantID:chartID:timeRange`. | medium | high -- cache module exists and is ready |
| `cmd/analytics-api/main.go` | Instantiate cache, inject into service constructor. Service constructor signature changes. | small | high -- read the code |
| `apps/dashboard/src/pages/Overview.tsx` | Change from single monolithic query to per-chart queries with progressive loading. Use `CHART_DATA_QUERY` (already defined) for each chart individually. Add skeleton/placeholder states. | large | high -- read the code |
| `apps/dashboard/src/components/Chart.tsx` | Add data downsampling before passing to Recharts. If dataset has >500 points, downsample to ~200-300 points using LTTB or simple interval sampling. | medium | high -- read the code |
| `apps/dashboard/src/components/ChartGrid.tsx` | Add lazy loading for below-the-fold charts (intersection observer or similar). | medium | medium -- depends on product decision |

## Dependencies

### Upstream (what affected code depends on)
- **ClickHouse clickhouse-go/v2 driver (v2.23.0):** Used in `internal/clickhouse/client.go`. The `driver.Conn` interface provides `Query()` and `QueryRow()`. Supports parameterized queries with `?` placeholders. No connection pooling configuration visible -- uses library defaults.
- **gqlgen (v0.17.45):** GraphQL code generation framework. The `analytics.NewExecutableSchema` and `analytics.Config` types are generated. Changes to `schema.graphql` require regenerating Go code.
- **Apollo Client (@apollo/client ^3.9.0):** Frontend GraphQL client. Default caching policy is `cache-first`, but dynamic time-range variables effectively disable cache hits on the overview page.
- **Recharts (^2.12.0):** SVG-based charting library. Performance degrades with large datasets because it renders individual SVG elements per data point. Known limitation -- no built-in virtualization or downsampling.
- **chi router (v5.0.12):** HTTP routing. Only relevant if new endpoints are added.

### Downstream (what depends on affected code)
- **Dashboard frontend (`apps/dashboard/`):** Primary consumer of the GraphQL API. Uses `OVERVIEW_PAGE_QUERY` and `CHART_DATA_QUERY` queries.
- **Unknown external API consumers:** The requirements flag this as an unknown. The CORS configuration allows all origins (`Access-Control-Allow-Origin: *`), which suggests the API may be consumed by other frontends or services. Changes to the GraphQL schema or response format could break unknown consumers.

### External
- **ClickHouse database:** Stores the events table (~500M rows). Connection via `CLICKHOUSE_URL` environment variable. Default: `clickhouse://localhost:9000/analytics`. Docker compose uses ClickHouse 24.2.
- **Vite dev server:** Dashboard frontend build tool. API URL configured via `VITE_API_URL` environment variable.

### Implicit
- **GraphQL schema code generation:** `schema.graphql` changes require running gqlgen code generation to update resolver interfaces. The generated code is not visible in the repository but `analytics.NewExecutableSchema` and `analytics.Config` are generated types.
- **Time-based cache invalidation:** The overview page uses `new Date()` for both `from` and `to`, which means the exact millisecond timestamp changes on every page load. This makes Apollo Client's cache useless for the overview query because the cache key includes the variables.

## Existing Patterns

- **Sequential query execution:** The current pattern in `service.go` is simple sequential iteration. Go's standard library provides `sync.WaitGroup` and `golang.org/x/sync/errgroup` for parallel execution. The codebase does not currently use either, but `context.Context` is already threaded through all calls, which is the prerequisite for `errgroup`.

- **Error handling with graceful degradation:** In `GetOverviewPage`, individual chart failures are logged and skipped (`continue`), not propagated. The summary failure returns an empty `SummaryData`. This pattern should be preserved in any optimization -- partial results are better than total failure.

- **Structured logging with zerolog:** All components use `github.com/rs/zerolog/log` for logging. Performance metrics are already logged: `query_time` and `rows` in `ExecuteQuery`, `total_time` and `charts_loaded` in `GetOverviewPage`. Any new code should follow this pattern.

- **Environment-based configuration:** Server port and ClickHouse URL are configured via environment variables with sensible defaults. Cache configuration (TTL, max size) should follow this pattern.

- **GraphQL resolver delegation:** Resolvers are thin -- they parse parameters and delegate to the service layer. This clean separation means the service layer can be optimized without changing the resolver interface.

- **Unused single-chart query:** `CHART_DATA_QUERY` in `analytics.ts` and `ChartData` resolver in `resolver.go` already support loading individual charts. This is an existing pattern that the frontend could use for progressive loading without any backend API changes.

## Technical Observations

- **The user_retention_cohort query has a parameter binding bug.** The query template in `queries.go` (lines 66-85) contains 5 `?` placeholders (tenant_id and time range in both the outer subquery and the LEFT JOIN subquery), but `ExecuteQuery` in `client.go` only passes 3 parameters (`params.TenantID, params.TimeFrom, params.TimeTo`). This means the query will either fail at runtime or silently bind incorrect values. This is likely causing either errors (the chart is skipped due to the `continue` on error in `GetOverviewPage`) or incorrect retention data.

- **No test files exist in the repository.** There are no `_test.go` files in any Go package and no test files in the frontend. This means there are no regression safeguards for any changes. Any optimization carries regression risk that cannot be caught by existing tests.

- **The cache eviction strategy is incomplete.** `internal/cache/cache.go` line 49-57: when the cache is at `maxSize` and a new item is set, it only removes one expired item. If no items are expired, the new item is still added (exceeding `maxSize`). This is a minor bug but could cause unbounded memory growth under heavy load.

- **The CORS handler allows all origins.** `main.go` line 49: `Access-Control-Allow-Origin: *`. This is permissive and suggests the API might be accessed from multiple frontends. This reinforces the risk that unknown consumers exist.

- **Docker compose defines the complete stack.** `docker-compose.yml` shows three services: `analytics-api`, `dashboard`, and `clickhouse`. The ClickHouse service uses version 24.2 and persists data in a named volume. This setup supports local development and testing of any changes.

- **No connection pooling configuration.** The ClickHouse client in `client.go` uses `clickhouse.Open(opts)` with default options parsed from the DSN. There's no explicit configuration of max connections, idle connections, or connection timeouts. When parallelizing queries, this could become a bottleneck if the default pool size is too small.

- **Data transformation is simple but wasteful.** In `service.go:loadChart` (lines 80-88), the `Row` type from the ClickHouse layer is converted to `DataPoint` one by one. The fields are identical (`Timestamp`, `Values`, `Labels`), so this is essentially a type alias copy. Not a performance bottleneck, but worth noting as unnecessary allocation.

## Test Coverage

| Area | Test Type | Coverage Level | Key Test Files | Notes |
|------|-----------|---------------|----------------|-------|
| `internal/analytics/` | unit/integration | none | (no test files found) | No tests for service, resolver, or models |
| `internal/clickhouse/` | unit/integration | none | (no test files found) | No tests for client or queries |
| `internal/cache/` | unit | none | (no test files found) | No tests for cache logic |
| `apps/dashboard/` | unit/e2e | none | (no test files found) | No frontend tests |
| `cmd/analytics-api/` | integration | none | (no test files found) | No server integration tests |

## Self-Critique Notes

- I could not verify runtime query performance because this is a static code analysis. The comments in the code suggest 2-5 seconds per query for large tenants, but actual execution times depend on data distribution, ClickHouse cluster configuration, and hardware -- none of which are visible from the code.

- I flagged the `user_retention_cohort` parameter binding bug with high confidence based on counting `?` placeholders vs. parameters passed to `ExecuteQuery`. However, ClickHouse's `clickhouse-go` driver might handle parameter binding differently than expected (e.g., reusing parameters). This should be verified at runtime.

- I did not find any generated Go code from gqlgen (no `generated.go` files). The `analytics.NewExecutableSchema` and `analytics.Config` references in `main.go` suggest generated code exists but was not included in the repository. Schema changes would require regeneration, but I cannot verify the exact generated interfaces.

- The "unknown external consumers" risk is based on the permissive CORS configuration and the requirements' own flagging of this unknown. I could not verify whether other consumers exist from the codebase alone.

- Connection pooling behavior under parallel query load is uncertain. The default pool settings for `clickhouse-go/v2` would need to be checked in the library documentation to determine whether parallelizing 8+ queries simultaneously would be constrained by connection limits.
