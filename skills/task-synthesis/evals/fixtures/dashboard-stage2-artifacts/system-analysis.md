# Codebase / System Analysis

## Relevant Modules

### Analytics Service (`internal/analytics/`)
- **Path:** `internal/analytics/`
- **Purpose:** Core business logic layer for the analytics dashboard. Contains the GraphQL resolver, service layer, data models, and schema. Orchestrates data loading by calling the ClickHouse client for each chart.
- **Key files:**
  - [`service.go` -- orchestrates overview page loading; contains `GetOverviewPage()` which loads 8 charts sequentially in a for-loop, plus summary data; `loadChart()` transforms ClickHouse rows into `ChartData`]
  - [`resolver.go` -- GraphQL resolver; `OverviewPage()` is the entry point called by the frontend; delegates to service layer; also exposes `ChartData()` for single chart queries and `Summary()` for summary-only queries]
  - [`models.go` -- data structures: `ChartData`, `DataPoint`, `OverviewPage`, `SummaryData`, `ChartConfig`; defines `DefaultOverviewCharts` -- the hardcoded list of 8 chart configurations with their query names and time ranges]
  - [`schema.graphql` -- GraphQL schema defining `Query.overviewPage`, `Query.chartData`, `Query.summary` operations; uses custom `DateTime` and `JSON` scalars]
- **Relevance to task:** This is where the sequential loading bottleneck lives. The `GetOverviewPage` function at `service.go:24-55` is the critical path -- it iterates over all 8 charts and calls `loadChart` for each one sequentially. No caching layer is integrated despite the existence of `internal/cache/`.

### ClickHouse Client (`internal/clickhouse/`)
- **Path:** `internal/clickhouse/`
- **Purpose:** Database access layer for ClickHouse. Contains the connection client, query execution, and predefined SQL queries for all chart types.
- **Key files:**
  - [`client.go` -- `Client` struct wrapping `clickhouse-go/v2` driver connection; `ExecuteQuery()` at line 54 runs arbitrary SQL and returns all rows loaded into memory with no result size limit; `GetSummaryData()` at line 89 runs 4 separate sequential `QueryRow` calls for summary metrics]
  - [`queries.go` -- map of named SQL queries for each chart type; 8 queries total; performance notes at lines 8-13 state queries on large tenants (>10M events) take 2-5 seconds each]
- **Relevance to task:** The queries themselves are the primary time consumers. Key bottlenecks found:
  - `user_retention_cohort` query (lines 67-85) uses a self-join with 5 `?` placeholders but `ExecuteQuery` only passes 3 parameters -- this is a **bug** that would cause a query error or incorrect results
  - `events_volume` query groups by `toStartOfHour` over 7 days with `event_type`, producing potentially thousands of rows (168 hours x N event types)
  - `error_rate_over_time` groups by `toStartOfMinute` over 24 hours, producing up to 1,440 rows
  - `top_events_by_count` has a `LIMIT 10000` -- the only query with any result cap, but 10,000 rows is still excessive for chart display
  - No queries use materialized views; all hit the raw `events` table

### Cache Utility (`internal/cache/`)
- **Path:** `internal/cache/`
- **Purpose:** Simple in-memory TTL cache with mutex-based concurrency control and background cleanup goroutine. Currently unused.
- **Key files:**
  - [`cache.go` -- `Cache` struct with `Get(key)` and `Set(key, value)` methods; TTL-based expiration; max size with simple eviction (removes first expired item found, not LRU); background cleanup on TTL interval]
- **Relevance to task:** This is explicitly marked as unused (`TODO: Consider using this for caching ClickHouse query results` at line 11). It provides the infrastructure needed to cache query results but has limitations: (1) eviction strategy is naive -- it only removes one expired item when full, not the oldest or least-used; (2) no per-key TTL -- all items share the same TTL; (3) values are stored as `any` with no serialization, which is fine for in-process caching but not distributed.

### Dashboard Frontend (`apps/dashboard/`)
- **Path:** `apps/dashboard/`
- **Purpose:** React SPA that renders the analytics dashboard. Uses Apollo Client for GraphQL data fetching and Recharts for chart rendering.
- **Key files:**
  - [`src/pages/Overview.tsx` -- main overview page component; uses `useQuery(OVERVIEW_PAGE_QUERY)` to fetch all data in one request; shows "Loading dashboard..." until the entire response arrives; no progressive loading, no skeleton states; time range uses `new Date().toISOString()` which changes every millisecond, effectively cache-busting Apollo's cache]
  - [`src/components/Chart.tsx` -- renders a single chart using Recharts; transforms all DataPoints into Recharts format at lines 50-55; no data downsampling -- every data point becomes an SVG element; WARNING comment at line 42 states 10k+ points cause 2-5 second rendering]
  - [`src/components/ChartGrid.tsx` -- CSS grid layout; all charts render simultaneously with no lazy loading or virtualization; WARNING comment at line 13 explicitly notes this compounds the rendering performance issue]
  - [`src/api/analytics.ts` -- GraphQL query definitions; `OVERVIEW_PAGE_QUERY` fetches all charts + summary in one request with all data points (no pagination, no limit); `CHART_DATA_QUERY` exists for single chart loading but is not used by the overview page]
- **Relevance to task:** The frontend has three compounding performance issues: (1) monolithic query that blocks on the slowest chart, (2) no downsampling of data points for rendering, (3) all 8 charts rendered simultaneously.

### API Entry Point (`cmd/analytics-api/`)
- **Path:** `cmd/analytics-api/`
- **Purpose:** HTTP server entry point using chi router + gqlgen GraphQL handler.
- **Key files:**
  - [`main.go` -- wires up ClickHouse client, analytics service, resolver, and GraphQL handler; chi middleware for logging and recovery; CORS setup for dashboard frontend; health check endpoint; no cache initialization]
- **Relevance to task:** This is where cache initialization and dependency injection would happen if caching is added. Currently creates `analyticsSvc := analytics.NewService(chClient)` with no cache parameter.

### Database Schema (`migrations/`)
- **Path:** `migrations/`
- **Purpose:** ClickHouse table definitions.
- **Key files:**
  - [`001_analytics_tables.sql` -- `events` table: MergeTree engine, partitioned by `(tenant_id, toYYYYMM(timestamp))`, ordered by `(tenant_id, event_type, timestamp)`, index_granularity 8192; ~500M rows total; explicit notes: no materialized views, no secondary indexes on user_id or session_id, queries filtering by user_id require full partition scans]
- **Relevance to task:** The partition/ordering scheme means tenant-scoped queries with time range and event_type filters align well with the primary key. However, the `user_retention_cohort` and `users_by_region` queries that need `uniq(user_id)` or group by `region` must do full partition scans since there's no secondary index on these columns.

## Change Points

| Location | What Changes | Scope | Confidence |
|----------|-------------|-------|------------|
| `internal/analytics/service.go:GetOverviewPage` | Sequential chart loading loop must be parallelized using goroutines/errgroup | medium | high |
| `internal/analytics/service.go:loadChart` | Should integrate caching -- check cache before querying ClickHouse, store results after | medium | high |
| `internal/clickhouse/queries.go:user_retention_cohort` | Bug fix: query has 5 `?` placeholders but only 3 params are passed; needs restructuring or the ExecuteQuery call needs additional params | small | high |
| `internal/clickhouse/queries.go` (multiple queries) | Queries need result limits, and some may need time-bucket coarsening (e.g., error_rate grouping by 5min instead of 1min) | medium | medium |
| `internal/clickhouse/client.go:ExecuteQuery` | May need to support parameterized query variants (for queries needing more than 3 params like retention cohort) | medium | high |
| `internal/cache/cache.go` | May need improvements: per-key TTL, better eviction, or replacement with a more robust solution | small | medium |
| `cmd/analytics-api/main.go` | Wire in cache initialization and pass to analytics service | small | high |
| `apps/dashboard/src/pages/Overview.tsx` | Change from single monolithic query to per-chart queries with progressive loading | large | high |
| `apps/dashboard/src/pages/Overview.tsx:26-27` | Fix cache-busting time range: quantize `from`/`to` to nearest minute or 5 minutes | small | high |
| `apps/dashboard/src/components/Chart.tsx:50-55` | Add data downsampling before rendering -- limit data points to ~200-500 for chart display | medium | high |
| `apps/dashboard/src/components/ChartGrid.tsx` | Add lazy loading or intersection observer to defer rendering of below-fold charts | medium | medium |
| `apps/dashboard/src/api/analytics.ts` | Refactor to use per-chart queries or add field-level pagination to overview query | medium | medium |

## Dependencies

### Upstream (what affected code depends on)
- **`github.com/99designs/gqlgen v0.17.45`:** GraphQL code generator and runtime. The resolver pattern is gqlgen-generated. Adding per-field resolvers or dataloaders for parallel loading would require changes to the gqlgen configuration. Constrains the resolver architecture.
- **`github.com/ClickHouse/clickhouse-go/v2 v2.23.0`:** ClickHouse driver. Used by `internal/clickhouse/client.go` for all database operations. Connection pooling behavior and query timeout settings come from this driver.
- **`@apollo/client ^3.9.0`:** Frontend GraphQL client. Manages query caching, network requests, and loading state. The `useQuery` hook drives the data-fetching pattern. Apollo's cache normalization and cache policies are relevant for any progressive loading changes.
- **`recharts ^2.12.0`:** Chart rendering library. All SVG-based rendering. Performance characteristics (rendering time scales linearly with data points) constrain how much data can be sent to the frontend.

### Downstream (what depends on affected code)
- **Dashboard frontend:** Primary consumer of the GraphQL API. Any API contract changes (field names, response structure, pagination) directly impact the frontend.
- **Unknown API consumers:** The requirements state it is unknown whether other services consume the same GraphQL API. The `overviewPage` query is dashboard-specific and unlikely to be used by other consumers, but the `chartData` query is more generic and could be shared.

### External
- **ClickHouse cluster:** External database service. Connected via `clickhouse://localhost:9000/analytics` (configurable via `CLICKHOUSE_URL` env var). The events table has ~500M rows with MergeTree engine. Query performance depends on cluster capacity, data distribution, and partition pruning efficiency.

### Implicit
- **Time-range coupling between frontend and backend:** The frontend constructs `from`/`to` timestamps using `new Date()` at `Overview.tsx:26-27` and passes them as GraphQL variables. These become the ClickHouse query time range parameters. Because the timestamp changes every millisecond, this creates a cascade effect: Apollo cache never hits, the API always receives a fresh query, and ClickHouse always executes. The time range is an implicit coupling that affects caching at every layer.
- **Hardcoded chart configuration:** `DefaultOverviewCharts` in `models.go:59-68` defines which charts load and in what order. This is not configurable per tenant. The time range per chart (`TimeRange` field: "24h", "7d", "30d") is stored in the config but is overridden by the API-level time range from the resolver -- the per-chart time range in the config appears unused (the resolver uses a single `TimeRange` from the GraphQL arguments for all charts).
- **SummaryData type duplication:** `SummaryData` is defined both in `internal/analytics/models.go` and `internal/clickhouse/client.go:132-137`. The ClickHouse client returns its own `SummaryData` type, which is then used by the analytics service. This is a minor code smell but could cause confusion during changes.

## Existing Patterns

- **Sequential service pattern:** The codebase uses a straightforward service-layer pattern where the resolver delegates to a service, which delegates to the database client. There is no use of goroutines, channels, errgroups, or any concurrent execution patterns anywhere in the codebase. This means introducing parallelism is a new pattern for this codebase -- there are no existing examples to follow. Reference: `service.go:30-37`.
- **Error handling -- skip and continue:** Failed chart loads are logged and skipped rather than failing the entire page load (`service.go:33-36`). This is a deliberate resilience pattern that should be preserved in any parallelization approach.
- **Named query registry:** ClickHouse queries are stored as a string map in `queries.go:14` and looked up by name. Chart configurations reference these by name in `models.go:60-68`. This indirection makes it easy to modify queries without changing service logic.
- **gqlgen resolver delegation:** Resolvers are thin wrappers that delegate to the service layer (`resolver.go`). The gqlgen handler is created in `main.go:38-40`. This pattern means performance changes should happen in the service layer, not in the resolvers.
- **Apollo useQuery pattern:** The frontend uses Apollo's `useQuery` hook with default cache-first policy (`Overview.tsx:22-32`). The existing `CHART_DATA_QUERY` in `analytics.ts:48-65` shows that the codebase already has a per-chart query available -- it is just not used by the overview page.

## Technical Observations

- **Critical bug -- retention cohort query parameter mismatch:** The `user_retention_cohort` query in `queries.go:67-85` contains 5 `?` placeholders (tenant_id and time range appear twice -- once for the cohort subquery and once for the returning subquery) but `ExecuteQuery` in `client.go:57` only passes 3 arguments (`params.TenantID, params.TimeFrom, params.TimeTo`). This query would fail at runtime with a parameter count mismatch. This means the retention chart has likely never worked correctly or has always been silently skipped (the service catches and logs errors at `service.go:33`).
- **Unbounded result sets:** `ExecuteQuery` at `client.go:52-54` explicitly warns "No result size limit. Large time ranges can return 100k+ rows." Only one query (`top_events_by_count`) has a LIMIT clause, and even that is set to 10,000. For `events_volume` grouping by hour over 7 days with multiple event types, the result set could easily be thousands of rows.
- **No connection pooling configuration:** The ClickHouse client in `client.go:17-33` uses default connection settings from `clickhouse.ParseDSN`. There is no explicit configuration of max connections, idle connections, or connection timeouts. If queries are parallelized, the connection pool may need to be sized to handle 8+ concurrent queries per request.
- **Cache eviction strategy is naive:** The cache in `cache.go:49-57` attempts eviction when full by iterating the map and removing the first expired item it finds. If no items are expired, it does nothing -- meaning the cache silently stops accepting new entries when full. This is not LRU or LFU and could lead to cache ineffectiveness under load.
- **Frontend date formatting per data point:** `Chart.tsx:51` calls `format(new Date(dp.timestamp), 'MMM dd HH:mm')` for every single data point during render. For charts with thousands of points, this creates thousands of Date objects and format calls on every render.
- **Per-chart TimeRange in config is unused:** `ChartConfig.TimeRange` field in `models.go:55` stores values like "24h", "7d", "30d" but these are never read -- the resolver passes a single `TimeRange` from the GraphQL arguments to all charts. This means the error_rate chart (configured for "24h") and the retention chart (configured for "30d") both receive the same 30-day time range from the frontend default, which makes the error_rate chart return far more data than intended.
- **CORS wildcard:** `main.go:49` sets `Access-Control-Allow-Origin: *`. Not a performance issue but worth noting for security context.

## Test Coverage

| Area | Test Type | Coverage Level | Key Test Files | Notes |
|------|-----------|---------------|----------------|-------|
| `internal/analytics/` | unit/integration | none | (no test files found) | No tests for service, resolver, or models. Any changes to the loading logic have no regression safety net. |
| `internal/clickhouse/` | unit/integration | none | (no test files found) | No tests for query execution or the query definitions. The retention cohort query bug confirms lack of testing. |
| `internal/cache/` | unit | none | (no test files found) | No tests for the cache utility despite it being a standalone component. |
| `apps/dashboard/` | unit/e2e | none | (no test files found) | No frontend tests found. No `jest.config`, no `vitest.config`, no test scripts in `package.json`. |
| `cmd/analytics-api/` | integration | none | (no test files found) | No integration or smoke tests for the API server. |

## Critique Review

The critic assessed this analysis as SUFFICIENT. The analysis identifies specific file paths, line numbers, and concrete bottlenecks based on actual code reading. The retention cohort query bug (5 placeholders vs 3 parameters) is a verified finding with high confidence. The critic's minor observations: (1) the analysis could have explicitly checked whether gqlgen supports dataloaders or per-field resolvers for parallelizing chart loading, and (2) the `ChartConfig.TimeRange` unused field could have been investigated further to understand whether it was intentional or an oversight. Both observations are noted but do not block the analysis quality.

## Open Questions

- Does `clickhouse-go/v2` support configurable connection pool sizes, and what is the default? This determines whether 8 parallel queries per request are feasible without connection starvation.
- Does gqlgen support field-level resolvers or dataloaders that could enable per-chart parallel resolution at the GraphQL layer, or must parallelization happen in the service layer?
- Is the `ChartConfig.TimeRange` field intended for future use (per-chart time ranges) or is it legacy? If per-chart time ranges are intended, the error_rate chart should use 24h and retention should use 30d, which would reduce data volumes for some charts.
- Has the retention cohort chart ever worked? If it has always been silently failing, is there an expectation that this optimization will also fix it?
- What is the network transfer time for large payloads? If a response with 8 charts containing thousands of data points each is serialized to JSON, the payload could be multiple megabytes. Is gzip/compression enabled on the API?
