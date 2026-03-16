# Stage 3 Handoff — Task Synthesis & Agreement Complete

## Task Summary

The analytics dashboard's main overview page takes 8-10 seconds to load 8 charts for ~50,000 daily active users. The target is under 2 seconds. The root cause is compounding bottlenecks across three layers: sequential ClickHouse queries on the backend (latency = sum of all queries), no caching at any layer, and the frontend rendering all charts simultaneously with unbounded data as SVG. The agreed approach is a systematic full-stack optimization: backend parallelization + caching with singleflight, query optimization with result limits, and frontend progressive loading + downsampling + lazy rendering.

## Classification
- **Type:** refactor (performance optimization of existing functionality)
- **Complexity:** high — multiple bottlenecks across 3 layers (database, API, frontend) requiring coordinated changes with no existing test coverage
- **Primary risk area:** technical — introducing concurrency and caching into untested code with no regression safety net
- **Solution direction:** systematic — full stack optimization across all three layers, as agreed with user

## Agreed Goal

Reduce the analytics dashboard main overview page load time from 8-10 seconds to under 2 seconds (wall-clock, P95, from the user's perspective) by addressing compounding bottlenecks across the Go backend, ClickHouse query layer, and React frontend -- without breaking the existing GraphQL API contract.

## Agreed Problem Statement

The dashboard is the primary data interface for 50,000 daily active users. Performance has degraded to 4-5x slower than target due to data volume growth (500M rows) outpacing the original implementation's capacity. The lack of parallelization, caching, and frontend optimization means every visit is a cold, sequential, full-data load. This erodes user trust and engagement with the data platform.

## Agreed Scope

### Included
- Backend parallelization of 8 sequential chart queries using errgroup
- Server-side caching integration with the existing cache utility (improved eviction)
- Singleflight for cache thundering herd protection
- Fix the retention cohort query bug (5 SQL params, only 3 passed)
- Fix the frontend cache-busting time range (quantize timestamps)
- Frontend progressive loading (per-chart queries using existing `chartData` endpoint)
- Frontend data downsampling before chart rendering (~200-500 points max)
- Lazy loading for below-fold charts
- Adding result limits to unbounded ClickHouse queries
- Connection pool configuration for parallel query workload

### Excluded
- ClickHouse materialized views or schema-level infrastructure changes
- Changes to the GraphQL API contract that would break existing consumers
- Other dashboard pages beyond the main overview
- Switching the charting library (Recharts retained)
- Per-chart time range activation (unused `ChartConfig.TimeRange` field)
- Comprehensive test suite creation (tests only for new parallel code paths)

## Key Scenarios for Planning

### Primary Scenario
1. User opens the analytics dashboard overview page
2. Frontend sends per-chart GraphQL queries (using existing `chartData` endpoint) rather than a monolithic query
3. Backend checks server-side cache for each chart request
4. On cache miss: queries ClickHouse with bounded results; on cache hit: returns cached data immediately
5. For overview page loads, backend queries execute in parallel using errgroup (latency = max, not sum)
6. Charts appear progressively as individual responses arrive (~500ms for first chart)
7. Below-fold charts load lazily as user scrolls
8. Data is downsampled before rendering (max ~200-500 SVG elements per chart)
9. All 8 charts visible and interactive within 2 seconds
10. Repeat visits within cache TTL return cached data for near-instant loads

### Mandatory Edge Cases
- Large tenants (>10M events): queries take 2-5s each; parallelization is mathematically required
- Thundering herd: singleflight must deduplicate concurrent cache-miss queries for the same key
- Cache-busting time range: frontend must quantize timestamps to enable Apollo cache hits
- Retention cohort bug: 5 SQL params / 3 passed -- must be fixed; chart has never worked

## System Map for Planning

### Modules to Change
| Module | Path | What Changes | Scope |
|--------|------|-------------|-------|
| Analytics Service | `internal/analytics/` | Parallelize GetOverviewPage; integrate caching into loadChart; add singleflight | large |
| ClickHouse Client | `internal/clickhouse/` | Fix retention bug; add result limits; support variable param counts | medium |
| Cache Utility | `internal/cache/` | Improve eviction (LRU or size-bounded); wire into service | small |
| API Entry Point | `cmd/analytics-api/` | Initialize cache; inject into analytics service | small |
| Dashboard Frontend | `apps/dashboard/` | Per-chart queries; progressive loading; downsampling; lazy loading; time range quantization | large |

### Key Change Points
| Location | What Changes | Why |
|----------|-------------|-----|
| `internal/analytics/service.go:GetOverviewPage` | Parallelize 8 sequential queries using errgroup | Sequential execution is root cause (sum vs max latency) |
| `internal/analytics/service.go:loadChart` | Check cache before ClickHouse; store after; wrap with singleflight | Eliminate redundant queries; protect against thundering herd |
| `internal/clickhouse/queries.go:67-85` | Fix retention cohort bug (5 params, 3 passed) | Chart has never worked; fix required for correctness |
| `internal/clickhouse/queries.go` (all) | Add LIMIT clauses; coarsen time buckets where appropriate | Unbounded results waste bandwidth, memory, and render time |
| `internal/clickhouse/client.go:ExecuteQuery` | Support variable parameter counts | Currently hardcoded to 3; retention needs 5 |
| `cmd/analytics-api/main.go` | Initialize cache; inject into service constructor | Cache utility exists but is not wired in |
| `apps/dashboard/src/pages/Overview.tsx` | Switch to per-chart queries with progressive loading | Monolithic query blocks on slowest chart |
| `apps/dashboard/src/pages/Overview.tsx:26-27` | Quantize from/to timestamps to nearest minute | Millisecond precision creates unique cache keys every load |
| `apps/dashboard/src/components/Chart.tsx:50-55` | Downsample data before rendering (cap ~200-500 points) | 10k+ SVG elements cause 2-5s render times |
| `apps/dashboard/src/components/ChartGrid.tsx` | Add intersection observer for lazy loading below-fold charts | All 8 charts rendering simultaneously compounds delay |

### Critical Dependencies
- **ClickHouse cluster:** Single data source; parallel queries multiply load 8x per request. Pool must be sized explicitly. Topology (single node vs. cluster) is unknown -- plan should account for both.
- **gqlgen framework:** Resolver-delegates-to-service pattern. Parallelization belongs in the service layer, not resolvers.
- **Apollo Client:** Frontend cache and query management. Per-chart queries use the existing `chartData` query. Time range quantization enables cache hits.
- **Recharts:** SVG rendering where point count = render time. Data must be downsampled before rendering.
- **clickhouse-go/v2:** Driver defaults for connection pooling are unknown; must be explicitly configured.

## Constraints the Plan Must Respect
- GraphQL API backward compatibility: no breaking changes, additive only -- user confirmed
- 2-second wall-clock target at P95: includes network, backend, and rendering -- user confirmed
- Zero existing test coverage: new parallel paths should have tests, but comprehensive suite is not in scope -- user confirmed
- No existing concurrency patterns: parallelization is a new pattern for this codebase -- user confirmed
- Skip-and-continue error handling: failed charts must not crash the page -- user confirmed
- ClickHouse schema changes out of scope: no materialized views -- user confirmed
- No deployment downtime: live system, 50k DAU -- user confirmed

## Risks the Plan Must Mitigate

| Risk | Likelihood | Impact | Mitigation Direction |
|------|-----------|--------|---------------------|
| Parallel queries overwhelm ClickHouse connection pool | medium | high | Configure explicit pool size; add semaphore; load test |
| Cache thundering herd | high | medium | sync/singleflight for request coalescing |
| Concurrency bugs in untested code | medium | high | errgroup for structured concurrency; tests; race detector |
| Downsampling hides operational signals | medium | medium | Configurable per chart type; preserve granularity for error_rate |
| Naive cache eviction fails under load | medium | medium | LRU eviction; cache monitoring |
| Retention cohort fix introduces expensive query | medium | low | Fix bug; monitor cost; optimize if needed |

## Product Requirements for Planning
- **Primary scenario:** User opens overview page and sees charts progressively within 2 seconds, with first chart visible in ~500ms
- **Success signals:** P95 page load < 2s; time to first chart < 500ms; dashboard engagement rate stable or increasing; ClickHouse query volume reduced
- **Minimum viable outcome:** Backend parallelization of chart queries (without this, the 2-second target is mathematically unachievable for large tenants)
- **Backward compatibility:** GraphQL API contract unchanged; visual chart output unchanged; data accuracy preserved

## Solution Direction

Systematic (full stack optimization) as agreed with user. The plan should address all three layers in coordinated phases:

1. **Backend (highest priority):** Parallelize queries with errgroup, integrate caching with singleflight, fix retention bug, configure connection pool
2. **Query layer:** Add result limits, coarsen time buckets, support variable param counts
3. **Frontend:** Switch to per-chart queries with progressive loading, downsample data before rendering, lazy-load below-fold charts, quantize time range for cache hits

The rationale: any single layer's optimization alone is insufficient. Backend parallelization is the mathematical requirement, but without caching the ClickHouse load multiplies dangerously under concurrent use. Without frontend changes, large result sets still cause slow rendering even with fast API responses.

## Accepted Assumptions
- The 2-second target is P95 (requirements do not specify percentile; P95 is industry standard)
- Backend parallelization + caching suffices without materialized views (queries at 2-5s parallelize to 2-5s total; caching eliminates repeats)
- The existing `chartData` GraphQL query is sufficient for per-chart frontend loading without API changes
- The retention cohort chart has always failed silently and no users depend on its data
- The cache utility's basic structure is sound; only the eviction strategy needs improvement

## Deferred Items
- Cache TTL per chart type: appropriate staleness per chart is a planning/implementation detail
- Connection pool sizing: requires benchmarking against actual ClickHouse cluster topology
- Downsampling algorithm choice: depends on chart type and data characteristics (LTTB, averaging, min-max)
- ClickHouse materialized views: deferred as infrastructure-level change out of current scope

## User Corrections from Synthesis

No corrections were made. The user confirmed all five agreement blocks without changes, selecting the systematic (full stack) solution direction.

## Acceptance Criteria
- P95 overview page load time under 2 seconds (wall-clock, across tenant sizes)
- Time to first chart visible under 500ms (progressive loading working)
- All 8 charts display correct data, including fixed retention cohort chart
- Charts render with downsampled data (max ~500 SVG elements per chart)
- Below-fold charts load lazily via intersection observer
- Server-side cache integrated with singleflight thundering herd protection
- Apollo Client cache produces hits on repeat visits (quantized time range)
- ClickHouse queries have result limits (no unbounded result sets)
- Failed charts skipped without crashing the page (resilience pattern preserved)
- GraphQL API contract unchanged for existing consumers
- No deployment downtime

## Detailed References
These files contain the full analysis and agreed model:
- `analysis.md` — synthesized task analysis
- `agreement-package.md` — agreement blocks presented to user
- `agreed-task-model.md` — full agreed task model with correction log
- `product-analysis.md` — detailed product/business analysis (Stage 2)
- `system-analysis.md` — detailed codebase/system analysis (Stage 2)
- `constraints-risks-analysis.md` — detailed constraints/risks analysis (Stage 2)
