# Constraints, Risks, and Integration Analysis

## Hard Constraints

### C1: Performance Target — Sub-2-Second Page Load
The PM has set a firm target of under 2 seconds for chart load times on the main overview page, measured as wall-clock time from the user's perspective. This is the primary acceptance criterion. Any solution that does not achieve this is insufficient.

### C2: Scope Limited to Main Overview Page
Only the main overview page (`Overview.tsx`) is in scope. Other dashboard pages, mobile clients, and other API consumers are explicitly excluded. Changes to shared infrastructure (e.g., ClickHouse schema, GraphQL API contract) must not break anything outside this scope.

### C3: ~50,000 Daily Active Users
The system serves significant traffic. Any changes must be safe under concurrent load. Caching must handle stampede scenarios (multiple users requesting the same tenant's data simultaneously). Backend parallelization must not overwhelm ClickHouse with connection bursts.

### C4: ClickHouse Infrastructure Changes May Be Acceptable
The requirements note that materialized views and schema changes are "acceptable if needed" but this is flagged as an assumption requiring confirmation. Any plan involving DDL changes (materialized views, new indexes) should be treated as requiring explicit approval.

### C5: GraphQL API Contract Preservation
The requirements state that changes to the GraphQL API contract that would affect other consumers are out of scope. The existing `overviewPage`, `chartData`, and `summary` queries must continue to work as defined in `schema.graphql`. However, the frontend can choose to call different existing queries (e.g., switch from `overviewPage` to multiple `chartData` calls) without breaking the contract.

## Soft Constraints

### C6: No New External Dependencies (Implicit)
The existing stack is Go + ClickHouse + React. Adding Redis or another caching layer would be an infrastructure change. The existing `internal/cache/cache.go` in-memory cache is the path of least resistance. If Redis is needed (e.g., for multi-instance deployments), that should be flagged.

### C7: Preserve Error Tolerance Pattern
The current backend gracefully skips failed charts (`service.go` line 33-34). Any parallelization must preserve this behavior — a single slow or failing chart should not block others.

### C8: Maintain Observability
Query timing and row counts are already logged (`service.go` lines 46-49, `client.go` lines 80-83). The `metadata.queryTimeMs` field is exposed to the frontend. These must be preserved for performance monitoring.

## Risks

### R1: Cache Staleness — Users See Outdated Data (Medium Likelihood, High Impact)

**Risk:** If in-memory caching is added, users may see stale analytics data. For an analytics product, showing stale data can be worse than being slow — it erodes trust.

**Mitigation:** Use short TTLs (30-60 seconds for real-time charts like error rate, 5-10 minutes for daily aggregates). Display "last updated" timestamps. Consider stale-while-revalidate: serve cached data immediately while refreshing in the background.

**Specific concern:** The `error_rate_over_time` chart has a 24-hour time range with minute-level granularity. Users monitoring active incidents need fresh data. This chart should have a shorter cache TTL than other charts.

### R2: Parallel Query Overload on ClickHouse (Medium Likelihood, Medium Impact)

**Risk:** Switching from sequential to parallel chart loading means 8-9 concurrent ClickHouse queries per page load instead of 1 at a time. At 50,000 DAU, this could create query bursts that overwhelm ClickHouse or exhaust the connection pool.

**Mitigation:** Use bounded concurrency (e.g., `errgroup` with semaphore, max 4 concurrent queries). Combine with caching to reduce actual query volume. Monitor ClickHouse `system.query_log` for queue depth after deployment.

**Code location:** `internal/clickhouse/client.go` — the `Client` struct holds a single `driver.Conn`. Need to verify that `clickhouse-go/v2` connection handles concurrent queries safely (it does — the driver uses connection pooling internally).

### R3: Frontend Data Volume Causes Browser Performance Issues (High Likelihood, Medium Impact)

**Risk:** Even if the backend responds in under 2 seconds, rendering 10,000+ SVG elements per chart in Recharts can take 2-5 seconds in the browser (`Chart.tsx` warning at lines 42-46). The performance target could be met on the API side but missed on the rendering side.

**Mitigation:** Implement data downsampling before rendering. For time-series charts, bucket data points to a maximum of ~200-500 points per chart. This is a frontend-only change. Alternatively, the backend could accept a `maxPoints` parameter and downsample server-side.

**Quantification:** The `error_rate_over_time` query with minute granularity over 24 hours produces ~1,440 data points. The `events_volume` query over 7 days with hourly granularity and 10 event types produces ~1,680 rows. The `active_users_over_time` query over 30 days with daily granularity produces ~30 points (this one is fine).

### R4: The `user_retention_cohort` Query Has a Parameter Binding Bug (High Likelihood, High Impact for That Chart)

**Risk:** The retention cohort query in `queries.go` (lines 66-85) contains 6 parameter placeholders (`?`) across its subqueries and JOIN clause, but `ExecuteQuery()` in `client.go` only passes 3 arguments (`tenantID, TimeFrom, TimeTo`). This means the query either fails with a parameter count mismatch or binds parameters incorrectly (e.g., using `TimeFrom` as a `tenant_id`).

**Impact:** The retention chart is likely silently failing at runtime (caught by the `continue` on error in `service.go` line 33-34), meaning users never actually see this chart. Fixing this is a correctness issue that should be addressed alongside performance.

**Mitigation:** The query needs to be rewritten to either use named parameters or have `ExecuteQuery` support variable-arity parameter lists.

### R5: Cache Stampede on Popular Tenants (Low Likelihood, High Impact)

**Risk:** When the cache expires for a popular tenant, many concurrent requests could simultaneously trigger ClickHouse queries for the same data — a "thundering herd" problem.

**Mitigation:** Implement "single-flight" pattern (`golang.org/x/sync/singleflight`) to deduplicate concurrent requests for the same cache key. Only one query executes; others wait for its result.

### R6: Apollo Cache Never Hits Due to Dynamic Time Range (High Likelihood, Low-Medium Impact)

**Risk:** This is a current bug, not a new risk. The Overview page passes `new Date().toISOString()` as a query variable, making every request unique from Apollo's perspective. Even if the backend responds instantly, the frontend always makes a network request.

**Mitigation:** Round the time range to the nearest minute or 5-minute interval. This makes the Apollo cache key stable within that window. Alternatively, use `fetchPolicy: 'cache-and-network'` to show cached data immediately while refetching.

### R7: Multi-Instance Deployment Invalidates In-Memory Cache (Medium Likelihood, Medium Impact)

**Risk:** If the analytics API runs multiple instances behind a load balancer, each instance has its own `internal/cache/cache.go` cache. Cache hit rates will be lower (1/N for N instances), and users may see inconsistent data across requests.

**Mitigation:** For the initial optimization, in-memory caching is still beneficial — even per-instance caching reduces ClickHouse load. If this is a concern, Redis or a shared cache can be added later. Document this as a known limitation.

## Integration Dependencies

### ClickHouse Events Table
- **Schema:** Defined in `migrations/001_analytics_tables.sql`
- **Dependency type:** Read-only. The dashboard queries but never writes.
- **Coupling:** Tight — query templates in `queries.go` reference specific column names (`tenant_id`, `user_id`, `event_type`, `timestamp`, `region`, `session_duration_ms`, `is_error`). Any schema change requires query updates.
- **Risk:** If materialized views are added, they are additive — the raw `events` table is unchanged. Existing queries continue to work. New queries can read from materialized views instead.

### GraphQL Schema (`schema.graphql`)
- **Consumers:** At minimum, the dashboard frontend. Potentially other services (unknown per requirements).
- **Constraint:** The `overviewPage`, `chartData`, and `summary` query types must remain backward-compatible.
- **Safe changes:** Adding new optional fields or arguments (e.g., `maxPoints: Int`) is backward-compatible. Removing or renaming fields is not.
- **Approach:** The frontend can switch from `overviewPage` to individual `chartData` queries without any schema changes — both queries already exist.

### Apollo Client (Frontend)
- **Cache behavior:** Default `cache-first` policy with variable-based cache keys. Currently ineffective due to millisecond-precision timestamps in variables.
- **Integration point:** Changing from a single `useQuery(OVERVIEW_PAGE_QUERY)` to multiple `useQuery(CHART_DATA_QUERY)` calls changes the data flow significantly. Each chart becomes independently loading/errored/loaded.

### Docker Compose Deployment
- **Defined in:** `docker-compose.yml`
- **Architecture:** Three services: `analytics-api` (port 4000), `dashboard` (port 3000), `clickhouse` (ports 8123/9000).
- **Constraint:** No Redis or additional services currently defined. Adding a new service requires updating `docker-compose.yml`.
- **Note:** The frontend connects to the API via `VITE_API_URL=http://localhost:4000/graphql`. No API gateway or CDN in front.

## Backward Compatibility

### API Contract
- **Safe:** Adding optional arguments to existing queries (e.g., `maxPoints` on `chartData`).
- **Safe:** Modifying resolver implementation (parallelizing, caching) without changing the response shape.
- **Unsafe:** Changing field types, removing fields, or altering the structure of `OverviewPage`, `ChartData`, `DataPoint`, etc.
- **Assessment:** All proposed optimizations (parallelization, caching, query optimization) can be done without breaking the API contract.

### Data Accuracy
- **Concern:** Caching introduces a window where data may be stale. For an analytics product, this needs explicit acceptance from stakeholders.
- **Mitigation:** Make cache TTL configurable. Expose cache age in API responses (e.g., add a `cachedAt` field to metadata).

### Frontend Behavior
- **Current:** Users see a single loading state, then all charts appear at once.
- **Proposed:** Progressive loading — charts appear independently as their data arrives.
- **Compatibility:** This is a UX improvement, not a regression. However, it changes the visual behavior that users are accustomed to. Skeleton states should be implemented to avoid layout shifts.

## Sensitive Areas

### 1. ClickHouse Query Performance Under Changed Load Patterns
Switching to parallel queries changes the load pattern on ClickHouse. Instead of a steady stream of sequential queries, the system will produce bursts of 8-9 concurrent queries per page load. ClickHouse handles concurrent queries well, but this should be validated under load.

### 2. Cache Memory Usage
The `internal/cache/cache.go` stores values as `any` (interface{}). Caching full `ChartData` objects (which include `[]DataPoint` with potentially thousands of entries) could consume significant memory. The `maxSize` parameter limits the number of entries, not the total memory. For 100 tenants with 8 charts each at 10,000 data points per chart, memory usage could reach hundreds of MB.

### 3. Cache Eviction Strategy
The current cache implementation (`cache.go` lines 49-57) has a naive eviction strategy: when `maxSize` is reached, it iterates the map and deletes the first expired item it finds. If no items are expired, the new item is silently not cached (the Set still runs, overwriting if key exists, but new keys are blocked). This is a design flaw that could cause important entries to not be cached under load.

### 4. The `user_retention_cohort` Query Structure
This query performs a self-join on the events table. Even with fixes to the parameter binding, it will remain one of the most expensive queries. It may need a fundamentally different approach (pre-computed materialized view for cohort data) to meet the 2-second target.

### 5. `gqlgen` Code Generation
The project uses `gqlgen` for GraphQL. The resolver structure in `resolver.go` follows gqlgen conventions. Any schema changes require running the gqlgen code generator to update generated types. The generated code is not visible in the repository (likely in a `generated.go` file not included in the mock), but schema changes will need regeneration.
