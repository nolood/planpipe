# Agreed Task Model

> Agreed on: 2026-03-14
> Based on: Stage 2 analyses + user review

## Task Goal

Reduce the analytics dashboard main overview page load time from 8-10 seconds to under 2 seconds (wall-clock, from the user's perspective) for ~50,000 daily active users, by addressing compounding bottlenecks across the Go backend, ClickHouse query layer, and React frontend -- without breaking the existing GraphQL API contract.

## Problem Statement

The analytics dashboard is the primary data interface for 50,000 daily active users. Its main overview page loads 8 charts sequentially from a 500M-row ClickHouse table with no caching, no parallelization, no data downsampling, and a cache-busting frontend bug that prevents client-side caching. The performance has degraded to 4-5x slower than the target, eroding user trust and engagement with the data platform. This is a repair task triggered by data volume growth outpacing the original implementation's capacity.

## Scope

### Included
- Backend parallelization of the 8 sequential chart queries using errgroup
- Server-side caching integration using the existing cache utility (with eviction improvements)
- Fix the retention cohort query bug (5 SQL params, only 3 passed)
- Fix the frontend cache-busting time range (quantize timestamps to nearest minute)
- Frontend progressive loading (switch from monolithic query to per-chart queries)
- Frontend data downsampling before chart rendering (cap SVG elements at ~200-500 points)
- Lazy loading for below-fold charts
- Adding result limits to unbounded ClickHouse queries
- Connection pool configuration for parallel query workload
- Singleflight for cache thundering herd protection

### Excluded
- ClickHouse materialized views or schema-level infrastructure changes
- Changes to the GraphQL API contract that would break existing consumers
- Other dashboard pages beyond the main overview
- Switching the charting library (Recharts is retained)
- Per-chart time range activation (the unused `ChartConfig.TimeRange` field)
- Comprehensive test suite creation (tests added only for new parallel code paths)

## Key Scenarios

### Primary Scenario
1. User opens the analytics dashboard overview page in their browser
2. The frontend sends per-chart GraphQL queries (using the existing `chartData` query endpoint) rather than a single monolithic query
3. The backend receives each chart request, checks the server-side cache first
4. On cache miss, queries ClickHouse with bounded results; on cache hit, returns cached data immediately
5. For the overview page load (where all 8 charts are requested), backend queries execute in parallel using errgroup (latency = max of individual queries, not sum)
6. Charts appear progressively on the frontend as individual responses arrive (~500ms for first chart)
7. Below-fold charts load lazily as the user scrolls
8. Data is downsampled before rendering to keep SVG element count manageable
9. All 8 charts are visible and interactive within 2 seconds
10. Repeat visits within the cache TTL window return cached data, resulting in near-instant loads

### Mandatory Edge Cases
- **Large tenants (>10M events):** Queries take 2-5s each. Backend parallelization is mathematically required. Caching further reduces load for repeated visits.
- **Thundering herd (concurrent page loads):** At 50k DAU, many users from the same tenant will load simultaneously during peak hours. Singleflight deduplicates in-flight queries for the same cache key to prevent ClickHouse overload.
- **Cache-busting time range:** Frontend must quantize `from`/`to` timestamps to a boundary (e.g., nearest minute) to enable Apollo Client cache hits.
- **Retention cohort query bug:** The `user_retention_cohort` query has 5 SQL placeholders but only 3 parameters are passed. Must be fixed -- this chart has likely always failed silently.

### Explicitly Deferred
- **Materialized views:** Infrastructure-level ClickHouse optimization. Deferred because parallelization + caching may suffice, and schema changes have higher operational risk. User confirmed this can wait.
- **Other dashboard pages:** Not in scope. Overview page is highest priority. User confirmed.
- **Per-chart time range activation:** `ChartConfig.TimeRange` exists but is unused. Activating it would change user-facing behavior. User confirmed this can be deferred.

## System Scope

### Affected Modules
| Module | Path | Role in Task | Change Scope |
|--------|------|-------------|-------------|
| Analytics Service | `internal/analytics/` | Contains sequential bottleneck; needs parallelization and caching integration | large |
| ClickHouse Client | `internal/clickhouse/` | Executes queries; contains SQL definitions, unbounded results, retention bug | medium |
| Cache Utility | `internal/cache/` | Unused TTL cache to be wired in; needs eviction improvements | small |
| API Entry Point | `cmd/analytics-api/` | Server wiring; needs cache initialization and DI | small |
| Dashboard Frontend | `apps/dashboard/` | Monolithic query, no progressive loading, no downsampling, cache-busting | large |
| Database Schema | `migrations/` | Defines events table; no changes unless materialized views added | none (deferred) |

### Key Change Points
| Location | What Changes | Why |
|----------|-------------|-----|
| `internal/analytics/service.go:GetOverviewPage` | Parallelize 8 sequential chart queries using errgroup | Sequential execution is root cause of 8-10s latency |
| `internal/analytics/service.go:loadChart` | Integrate caching (check cache before ClickHouse, store after) | Eliminate redundant queries within TTL window |
| `internal/clickhouse/queries.go:67-85` | Fix retention cohort bug: 5 params but only 3 passed | Chart has never worked; fix is required for correctness |
| `internal/clickhouse/queries.go` (all) | Add result limits; coarsen time buckets where appropriate | Unbounded result sets waste bandwidth and memory |
| `internal/clickhouse/client.go:ExecuteQuery` | Support variable parameter counts | Currently hardcoded to 3 params; retention cohort needs 5 |
| `cmd/analytics-api/main.go` | Initialize cache, inject into service | Cache exists but is not wired in |
| `apps/dashboard/src/pages/Overview.tsx` | Switch to per-chart queries with progressive loading | Monolithic query blocks on slowest chart |
| `apps/dashboard/src/pages/Overview.tsx:26-27` | Quantize time range | Millisecond precision creates unique cache keys every load |
| `apps/dashboard/src/components/Chart.tsx:50-55` | Add data downsampling (cap at ~200-500 points) | 10k+ SVG elements cause 2-5s render times |
| `apps/dashboard/src/components/ChartGrid.tsx` | Add lazy loading for below-fold charts | All 8 charts rendering simultaneously compounds performance |

### Dependencies
- **ClickHouse cluster:** Single data source; parallel queries multiply concurrent load 8x per request. Connection pool must be explicitly sized.
- **gqlgen framework:** Resolver architecture constrains parallelization approach. Service-layer parallelization is preferred.
- **Apollo Client:** Frontend caching and query management. Per-chart queries use the existing `chartData` query endpoint.
- **Recharts:** SVG rendering where point count = render time. Data must be downsampled before passing to Recharts.
- **clickhouse-go/v2:** Connection pooling defaults are unknown; must be explicitly configured.

## Confirmed Constraints
- **GraphQL API backward compatibility:** No breaking changes. Additive only. Unknown consumers may exist. -- confirmed by user
- **2-second wall-clock target (P95):** Includes network, backend, and rendering. P95 assumed as reasonable target since percentile was undefined in requirements. -- confirmed by user
- **Zero test coverage:** All modules lack tests. New parallel code paths should have tests but comprehensive coverage is not in scope. -- confirmed by user
- **No existing concurrency patterns:** Parallelization is a new pattern for this codebase. -- confirmed by user
- **Skip-and-continue error handling:** Failed charts must not crash the page. -- confirmed by user
- **ClickHouse schema changes out of scope:** Materialized views deferred. -- confirmed by user
- **No deployment downtime:** Live system serving 50k DAU. -- confirmed by user

## Risks to Mitigate

| Risk | Likelihood | Impact | Mitigation Direction |
|------|-----------|--------|---------------------|
| Parallel queries overwhelm ClickHouse connection pool | medium | high | Configure explicit pool size; add semaphore concurrency limiter; load test |
| Cache thundering herd | high | medium | Use sync/singleflight for request coalescing |
| Concurrency bugs in untested code | medium | high | Use errgroup; add tests for parallel path; run Go race detector |
| Downsampling hides operational signals | medium | medium | Make downsampling configurable per chart type; preserve granularity for error_rate |
| Naive cache eviction fails under load | medium | medium | Replace with LRU eviction; add cache hit/miss monitoring |
| Retention cohort fix introduces expensive query | medium | low | Fix bug, monitor query cost, optimize if needed |

## Solution Direction

Systematic (full stack optimization) -- as confirmed by user. The plan should address all three layers: backend parallelization + caching with singleflight, ClickHouse query optimization (limits, bug fix), and frontend progressive loading + downsampling + lazy loading. This produces the most complete solution and best perceived performance, ensuring the 2-second target is reliably met across tenant sizes with a responsive, modern user experience.

## Accepted Assumptions
- The 2-second target is P95 (since the requirements do not specify a percentile, P95 is assumed as a reasonable and achievable target). Accepted because P95 is industry standard for user-facing performance targets.
- Backend parallelization + caching will be sufficient to meet the 2-second target without materialized views. Accepted because the analyses show individual queries take 2-5s for large tenants, and parallelizing means the total backend time becomes the max (2-5s) rather than the sum (16-40s). With caching reducing repeat queries to near-zero, the target is achievable.
- The existing `chartData` GraphQL query is sufficient for per-chart frontend loading without API contract changes. Accepted because the system analysis confirms this query already exists in the schema.
- The retention cohort chart has always failed silently and no users depend on its data. Accepted because the parameter count mismatch is a confirmed bug and the service's skip-and-continue pattern means the chart would have been silently dropped.

## Deferred Decisions
- **Cache TTL per chart type:** The appropriate staleness for each chart (error_rate: 30s-1min vs. retention: 5-10min) is a detail for the planning stage. Deferred because it requires defining freshness SLAs that are not in the requirements.
- **Connection pool sizing:** The exact pool size depends on ClickHouse cluster topology and expected concurrent users. Deferred to planning/implementation because it requires benchmarking.
- **Downsampling algorithm:** Whether to use LTTB, simple averaging, or min-max bucketing for chart data reduction. Deferred to planning because the choice depends on chart type and data characteristics.

## User Corrections Log

No corrections were made. The user confirmed all five blocks without changes.

- **Block 1 (Goal & Problem):** Confirmed as proposed
- **Block 2 (Scope):** Confirmed as proposed
- **Block 3 (Key Scenarios):** Confirmed as proposed
- **Block 4 (Constraints):** Confirmed as proposed
- **Block 5 (Solution Direction):** Confirmed -- systematic (full stack) approach selected

## Acceptance Criteria
- P95 overview page load time is under 2 seconds (wall-clock, from user's perspective, measured across tenant sizes)
- Time to first chart visible is under 500ms (progressive loading)
- All 8 charts display correct data (including the fixed retention cohort chart)
- Charts render with downsampled data (no more than 500 SVG elements per chart)
- Below-fold charts load lazily
- Server-side cache is integrated with singleflight for thundering herd protection
- Apollo Client cache produces hits on repeat visits (time range is quantized)
- ClickHouse queries have result limits (no unbounded result sets)
- Failed charts are skipped without crashing the page (existing resilience pattern preserved)
- GraphQL API contract is unchanged for existing consumers
- No deployment downtime
