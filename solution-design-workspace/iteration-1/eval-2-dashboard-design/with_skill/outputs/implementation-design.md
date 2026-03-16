# Implementation Design

> Task: Optimize analytics dashboard overview page from 8-10s to <2s P95 load time
> Solution direction: systematic — full-stack optimization across backend, cache, and frontend
> Design status: finalized

## Implementation Approach

### Chosen Approach
The implementation follows a three-layer systematic optimization that addresses every bottleneck in the request lifecycle: backend query parallelization with caching, and frontend progressive loading with downsampling. Each layer is designed to be independently deployable and testable.

On the backend, the sequential `GetOverviewPage` loop in `internal/analytics/service.go:30-37` is replaced with `errgroup`-based parallel execution. Each chart query runs in its own goroutine, with shared `context.Context` for cancellation. Before hitting ClickHouse, each query checks an in-memory cache keyed by `tenantID:chartID:quantizedTimeRange`. Cache misses are deduplicated using `sync/singleflight` so that concurrent requests for the same chart by the same tenant share a single ClickHouse query. The existing `internal/cache/cache.go` is enhanced with per-key TTL support and LRU eviction to replace its current naive eviction strategy.

On the frontend, the monolithic `OVERVIEW_PAGE_QUERY` in `apps/dashboard/src/pages/Overview.tsx` is replaced with per-chart `CHART_DATA_QUERY` calls (which already exist in `apps/dashboard/src/api/analytics.ts:48-65`). Each chart loads independently with its own loading/error state. Above-fold charts render immediately; below-fold charts use an IntersectionObserver for lazy loading. Data is downsampled to a maximum of 500 points before passing to Recharts, and the time range is quantized to 5-minute boundaries to enable Apollo Client cache hits.

This approach was chosen because it addresses every identified bottleneck without changing the GraphQL API contract, without ClickHouse schema changes, and using patterns that can be individually validated. Backend parallelization alone brings theoretical time from `SUM(queries)` to `MAX(queries)` (~2-5s for large tenants), and caching + singleflight further reduce that to near-zero for repeated loads. Frontend splitting eliminates the "wait for slowest chart" blocking pattern, and downsampling caps rendering overhead.

### Alternatives Considered
- **Backend-only optimization (parallelize + cache, no frontend changes):** This would achieve ~2-5s for the slowest query on a cache miss for large tenants. Rejected because the P95 target of 2s is tight, and frontend rendering of thousands of SVG elements (2-5s per Chart.tsx:42 warning) would still push total time over the target even with a fast backend.
- **GraphQL schema redesign with subscriptions for streaming:** Would allow true progressive delivery at the protocol level. Rejected because it introduces a new communication pattern (WebSocket), requires schema changes (subscription type), and is explicitly excluded from agreed scope.
- **Per-field GraphQL resolvers via gqlgen dataloaders:** Would parallelize at the GraphQL layer instead of the service layer. Rejected because gqlgen's dataloader pattern is designed for N+1 query deduplication, not for parallelizing independent queries. Parallelization in the service layer is simpler, more explicit, and doesn't require gqlgen configuration changes.
- **External cache (Redis) instead of in-memory:** Would support multi-instance deployments. Rejected because the agreed model confirmed in-memory cache is sufficient for <50k DAU and a single API instance. Redis adds operational complexity without clear benefit at this scale.

### Approach Trade-offs
This approach optimizes for safety and backward compatibility at the cost of not achieving the absolute fastest possible performance. By keeping the GraphQL API contract unchanged, we accept that the frontend must fire 8+1 individual HTTP requests instead of receiving all data in one response. By using in-memory cache, we accept that cache is not shared across API instances (single-instance deployment assumed). By downsampling on the frontend rather than the backend, we accept transferring more data over the network than strictly necessary, but avoid changing the API contract. By not adding materialized views, we accept that cache-miss queries for large tenants will still take 2-5 seconds individually — the 2s P95 target depends on caching being effective for repeated loads.

## Solution Description

### Overview
When a user opens the dashboard, the frontend fires 9 parallel GraphQL requests: one `summary` query and 8 `chartData` queries (one per chart). Time range variables are quantized to 5-minute boundaries so Apollo Client can cache responses. The summary bar renders first since it's a lightweight query. Each chart renders independently as its data arrives — above-fold charts fire immediately, below-fold charts wait for IntersectionObserver visibility.

On the backend, each `chartData` resolver calls `Service.GetChartData`, which calls `loadChart`. The new `loadChart` implementation first constructs a cache key from tenantID + chartID + quantized time range, then checks the in-memory cache. On cache hit, it returns immediately. On cache miss, it goes through `singleflight.Group.Do()` to deduplicate concurrent identical requests, then executes the ClickHouse query. The result is cached with a chart-type-specific TTL (1m for error_rate, 5m for event volume, 15m for retention cohorts).

The `GetOverviewPage` resolver is preserved for backward compatibility but internally uses `errgroup` to parallelize its chart loading, so any existing or unknown consumers of the `overviewPage` query also benefit from the backend optimization.

### Data Flow
1. **Entry point:** Browser → `CHART_DATA_QUERY` GraphQL request → `/graphql` endpoint (chi router, `cmd/analytics-api/main.go:60`)
2. **Resolver:** `resolver.go:ChartData()` → `service.go:GetChartData()` → `service.go:loadChart()`
3. **Cache layer (new):** `loadChart()` → cache key construction → `cache.Get(key)` → if hit, return cached `ChartData`
4. **Singleflight (new):** If cache miss → `singleflight.Group.Do(key, func)` → deduplicates concurrent identical requests
5. **ClickHouse query:** `client.go:ExecuteQuery()` → ClickHouse SQL with tenant_id, time_from, time_to
6. **Cache store (new):** Result stored via `cache.SetWithTTL(key, result, ttl)`
7. **Response:** `ChartData` struct → GraphQL serialization → JSON response
8. **Frontend rendering:** Apollo `useQuery` → `Chart` component → `downsample(dataPoints, 500)` → Recharts SVG render

For `overviewPage` queries (backward compatibility path):
1. `resolver.go:OverviewPage()` → `service.go:GetOverviewPage()`
2. `GetOverviewPage()` → `errgroup` spawning 8 goroutines, each calling `loadChart()` (which goes through cache + singleflight)
3. All 8 results collected → `OverviewPage` struct returned

### New Entities

| Entity | Type | Location | Purpose |
|--------|------|----------|---------|
| `SetWithTTL` | method | `internal/cache/cache.go` | Allows per-key TTL for chart-type-specific cache durations |
| `chartCacheTTLs` | var (map) | `internal/analytics/service.go` | Maps chart IDs to their cache TTL durations |
| `downsample` | function | `apps/dashboard/src/utils/downsample.ts` | LTTB downsampling algorithm to reduce data points to N max |
| `useChartData` | hook | `apps/dashboard/src/hooks/useChartData.ts` | Custom React hook wrapping `CHART_DATA_QUERY` with quantized time range |
| `ChartSkeleton` | component | `apps/dashboard/src/components/ChartSkeleton.tsx` | Skeleton/loading placeholder for individual chart slots |
| `LazyChart` | component | `apps/dashboard/src/components/LazyChart.tsx` | Wrapper using IntersectionObserver for below-fold lazy loading |
| `quantizeTime` | function | `apps/dashboard/src/utils/quantizeTime.ts` | Rounds timestamps to 5-minute boundaries for cache-friendly queries |

### Modified Entities

| Entity | Location | Current Behavior | New Behavior | Breaking? |
|--------|----------|-----------------|-------------|-----------|
| `Service` struct | `internal/analytics/service.go:13-15` | Holds only `ch *clickhouse.Client` | Adds `cache *cache.Cache` and `sfGroup *singleflight.Group` fields | no |
| `NewService` | `internal/analytics/service.go:17-19` | Takes only `ch *clickhouse.Client` | Takes `ch *clickhouse.Client` and `cache *cache.Cache` | no (internal) |
| `GetOverviewPage` | `internal/analytics/service.go:24-55` | Sequential for-loop over charts | `errgroup`-based parallel execution with context | no |
| `loadChart` | `internal/analytics/service.go:58-103` | Direct ClickHouse query | Cache check → singleflight → ClickHouse query → cache store | no |
| `Cache.Set` | `internal/cache/cache.go:45-63` | Uniform TTL, naive eviction | Preserved as-is; new `SetWithTTL` method added | no |
| Cache eviction | `internal/cache/cache.go:49-57` | Removes first expired item only | LRU eviction: track access time, remove least-recently-used when full | no |
| `main` | `cmd/analytics-api/main.go:20-84` | No cache init | Creates cache, passes to `NewService`, registers cache metrics | no |
| `Overview` component | `apps/dashboard/src/pages/Overview.tsx` | Single `useQuery(OVERVIEW_PAGE_QUERY)` | Per-chart `useChartData` hooks + separate `useSummary` + lazy loading | no |
| `Chart` component | `apps/dashboard/src/components/Chart.tsx:50-55` | Passes all data points to Recharts | Calls `downsample(dataPoints, 500)` before transforming | no |
| `ChartGrid` | `apps/dashboard/src/components/ChartGrid.tsx` | Renders all children immediately | Wraps each child in `LazyChart` for below-fold lazy loading | no |
| `ExecuteQuery` | `internal/clickhouse/client.go:54-86` | Passes 3 params (tenantID, timeFrom, timeTo) | Accept variadic params to support queries with more than 3 placeholders | no |
| `user_retention_cohort` query | `internal/clickhouse/queries.go:66-85` | 5 placeholders, 3 params passed (bug) | 5 placeholders, 5 params passed correctly | no (bug fix) |
| `error_rate_over_time` query | `internal/clickhouse/queries.go:112-124` | Groups by `toStartOfMinute` | Groups by `toStartOfFiveMinute` (reduces max rows from 1440 to 288 for 24h) | no |
| `top_events_by_count` query | `internal/clickhouse/queries.go:38-48` | `LIMIT 10000` | `LIMIT 500` | no |

## Change Details

### Module: Analytics Service

**Path:** `internal/analytics/`
**Role in changes:** Core orchestration — parallelization, caching, singleflight integration

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `service.go` | modify | Add cache + singleflight fields to Service struct; change NewService signature; rewrite GetOverviewPage with errgroup; wrap loadChart with cache-check + singleflight | large |
| `resolver.go` | no change | No changes needed — resolvers already delegate to service; ChartData resolver is already implemented | none |
| `models.go` | no change | Types remain the same; no structural changes | none |
| `schema.graphql` | no change | GraphQL API contract unchanged | none |

**Interfaces affected:**
- `NewService(ch *clickhouse.Client)` changes to `NewService(ch *clickhouse.Client, c *cache.Cache)` — only called from `main.go`

**Tests needed:**
- `GetOverviewPage` parallel execution: verify all 8 charts returned, verify skip-and-continue on individual failures
- `loadChart` with cache: verify cache hit returns cached data, verify cache miss queries ClickHouse, verify singleflight dedup
- Race condition testing with `go test -race`

### Module: ClickHouse Client

**Path:** `internal/clickhouse/`
**Role in changes:** Bug fix, query optimization, parameter flexibility

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `client.go` | modify | Change `ExecuteQuery` to accept variadic params instead of fixed 3 params | small |
| `queries.go` | modify | Fix retention cohort param bug; reduce error_rate granularity to 5-min; reduce top_events LIMIT to 500 | medium |

**Interfaces affected:**
- `ExecuteQuery(ctx, query, params QueryParams)` changes to `ExecuteQuery(ctx, query string, args ...any)` — called from `service.go:loadChart`
- Alternative: keep `QueryParams` but add an `ExtraArgs []any` field — less invasive

**Tests needed:**
- `ExecuteQuery` with 3 params (standard queries)
- `ExecuteQuery` with 5 params (retention cohort)
- Retention cohort query returns valid data (verifies bug fix)

### Module: Cache Utility

**Path:** `internal/cache/`
**Role in changes:** Enhanced eviction, per-key TTL support

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `cache.go` | modify | Add `SetWithTTL(key, value, ttl)` method; implement LRU eviction by tracking last-access time in `cacheItem`; add `lastAccessed` field to `cacheItem` | medium |

**Interfaces affected:**
- New method `SetWithTTL(key string, value any, ttl time.Duration)` — additive, no breaking changes
- `cacheItem` struct gains `lastAccessed time.Time` field — internal, no external consumers

**Tests needed:**
- `SetWithTTL` with varying TTLs; verify items expire at correct times
- LRU eviction: verify least-recently-used item is evicted when cache is full
- Concurrent Get/Set with race detector

### Module: API Entry Point

**Path:** `cmd/analytics-api/`
**Role in changes:** Dependency wiring

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `main.go` | modify | Import cache package; create cache instance with default TTL + max size; pass cache to `NewService`; optionally expose cache hit/miss counters via health endpoint | small |

**Interfaces affected:**
- None — `main.go` is the composition root, not consumed by other code

**Tests needed:**
- Integration test: verify server starts with cache initialized

### Module: Dashboard Frontend

**Path:** `apps/dashboard/`
**Role in changes:** Per-chart queries, progressive loading, downsampling, cache fix

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `src/pages/Overview.tsx` | modify | Replace single `useQuery` with per-chart `useChartData` hooks + `useSummary` hook; add skeleton states per chart; split above-fold and below-fold rendering | large |
| `src/components/Chart.tsx` | modify | Add `downsample()` call before data transformation at line 50; import from `utils/downsample` | small |
| `src/components/ChartGrid.tsx` | modify | Wrap children with `LazyChart` component for below-fold lazy loading | small |
| `src/api/analytics.ts` | modify | Add `SUMMARY_QUERY` for standalone summary loading; existing `CHART_DATA_QUERY` is already defined and usable | small |
| `src/utils/downsample.ts` | create | LTTB (Largest-Triangle-Three-Buckets) downsampling function | small |
| `src/utils/quantizeTime.ts` | create | Time range quantization to 5-minute boundaries | small |
| `src/hooks/useChartData.ts` | create | Custom hook wrapping `useQuery(CHART_DATA_QUERY)` with quantized time variables | small |
| `src/components/ChartSkeleton.tsx` | create | Skeleton placeholder matching chart container dimensions | small |
| `src/components/LazyChart.tsx` | create | IntersectionObserver wrapper for deferred rendering | small |

**Interfaces affected:**
- `Overview` component API unchanged (still accepts `tenantId` prop)
- `Chart` component API unchanged (still accepts same props)
- `ChartGrid` component API unchanged (still accepts `children` prop)

**Tests needed:**
- `Overview` renders charts progressively (fast chart appears before slow chart)
- `downsample` preserves min/max values and returns <= N points
- `quantizeTime` rounds to 5-minute boundaries correctly
- `useChartData` uses quantized time range (verify cache key stability)
- `LazyChart` defers rendering until intersection

## Key Technical Decisions

| # | Decision | Reasoning | Alternatives Rejected | User Approved? |
|---|----------|-----------|----------------------|----------------|
| 1 | Use `errgroup` for backend parallelization | errgroup provides structured concurrency with context cancellation and error collection. It's the idiomatic Go pattern for "run N things in parallel, collect errors." The codebase has no existing concurrency patterns, so errgroup introduces a clean, well-tested one. | Raw goroutines + WaitGroup (less error handling), worker pool (over-engineered for fixed 8 queries) | yes |
| 2 | Enhance existing cache rather than replace | The existing `internal/cache/cache.go` provides the basic structure. Adding per-key TTL and LRU eviction is less disruptive than introducing a third-party cache library. | `patrickmn/go-cache` (additional dependency), `hashicorp/golang-lru` (no per-key TTL built-in), custom cache from scratch (unnecessary) | yes |
| 3 | Use singleflight for cache-miss deduplication | `sync/singleflight` is a stdlib-adjacent package specifically designed for this pattern. Under 50k DAU, concurrent requests for the same tenant's dashboard will generate many identical ClickHouse queries. singleflight ensures only one in-flight query per unique key. | Distributed lock via Redis (out of scope, over-engineered), no deduplication (thundering herd risk) | yes |
| 4 | Quantize frontend time range to 5-minute boundaries | Current `new Date().toISOString()` changes every millisecond, making Apollo cache keys unique on every call. Quantizing to 5-minute boundaries means requests within the same 5-minute window share a cache key. 5 minutes balances freshness vs. cache hit rate. | 1-minute quantization (too frequent cache misses), 15-minute (too stale for error monitoring) | yes |
| 5 | Downsample on frontend, not backend | Keeps the GraphQL API contract unchanged — the `dataPoints` array in the response is unmodified. Downsampling on the frontend is simpler to implement and doesn't risk affecting unknown API consumers. | Backend downsampling in `loadChart` (changes API response, may affect unknown consumers), SQL-level downsampling (complex, harder to make configurable per chart type) | yes |
| 6 | Use LTTB algorithm for downsampling | LTTB (Largest-Triangle-Three-Buckets) preserves visual shape better than naive every-Nth-point sampling. It keeps peaks and valleys that are visually significant. Well-established algorithm with simple implementation. | Every-Nth-point (loses peaks/valleys), min-max bucketing (more complex, less smooth) | not required |
| 7 | Change `ExecuteQuery` to accept variadic params | The retention cohort query needs 5 params but `ExecuteQuery` currently hardcodes 3. Rather than creating a separate method, making params variadic handles this query and any future queries that need different param counts. | Add `ExtraArgs` field to `QueryParams` (works but awkward API), create separate `ExecuteQueryWithParams` method (duplicates logic) | not required |
| 8 | Keep `overviewPage` GraphQL query working (backward compatibility) | Unknown API consumers may use this query. Internally it now uses errgroup for parallel execution, so it's faster but returns the same response shape. | Remove `overviewPage` query (breaks unknown consumers), deprecate with warning (unnecessary complexity) | yes |
| 9 | Per-chart-type cache TTLs | Different charts have different freshness needs. Error rate (operational monitoring) needs short TTL (1 min). Retention cohorts (backward-looking) can tolerate longer TTL (15 min). Event volume is in between (5 min). | Uniform TTL for all charts (either too stale for error_rate or too frequent cache misses for retention) | yes |

## Dependencies

### Internal Dependencies
- **Analytics Service → Cache:** Service depends on cache for `Get`/`SetWithTTL`. Cache must be initialized before service.
- **Analytics Service → singleflight:** `golang.org/x/sync/singleflight` — standard library extension, needs `go get`.
- **Analytics Service → errgroup:** `golang.org/x/sync/errgroup` — standard library extension, needs `go get`.
- **Analytics Service → ClickHouse Client:** Existing dependency. `ExecuteQuery` signature changes — service calling code must be updated to match.
- **main.go → Cache + Service:** Composition root creates cache, passes to service constructor.
- **Frontend Overview → useChartData + LazyChart + ChartSkeleton:** New components/hooks consumed by the rewritten Overview page.
- **Frontend Chart → downsample:** Chart component imports the new utility function.

### External Dependencies
- **`golang.org/x/sync`:** Provides `errgroup` and `singleflight`. Well-maintained Go extended standard library. No version constraints.
- **ClickHouse cluster:** Parallel queries increase per-request concurrency from 1 to 8 concurrent connections. The `clickhouse-go/v2` driver's default connection pool needs to be sized appropriately. The DSN can include `MaxOpenConns` and `MaxIdleConns` parameters.
- **No new frontend dependencies:** `IntersectionObserver` is a native browser API. LTTB downsampling is a simple function (no library needed).

### Migration Dependencies
No migrations required. No ClickHouse schema changes. No data migrations.

## Implementation Sequence

| Step | What | Why This Order | Validates |
|------|------|----------------|-----------|
| 1 | Fix retention cohort query bug (`queries.go`) + change `ExecuteQuery` to variadic params (`client.go`) | Foundation fix — unblocks the retention chart. Variadic params are needed by the bug fix and are a prerequisite for the service layer changes. | Retention chart query executes without parameter error; all existing queries still work |
| 2 | Enhance cache: add `SetWithTTL`, add LRU eviction, add `lastAccessed` tracking (`cache.go`) | Cache must exist before service can use it. Independent of query changes. | Cache stores and retrieves with per-key TTL; eviction works under load; race detector passes |
| 3 | Add `errgroup` + `singleflight` to service; modify `NewService` to accept cache; wire cache into `loadChart` (`service.go`) | Core backend optimization. Depends on steps 1-2. | `GetOverviewPage` executes charts in parallel (verified by timing); cache hit/miss works; singleflight deduplicates; skip-and-continue preserved |
| 4 | Update `main.go` to create cache and pass to service; configure ClickHouse connection pool | Wires everything together. Depends on step 3. | Server starts, serves requests with caching and parallelization active |
| 5 | Optimize ClickHouse queries: coarsen error_rate to 5-min, reduce top_events LIMIT (`queries.go`) | Reduces data volume. Independent of caching but best done after parallel execution is verified. | Fewer rows returned per query; charts still display correctly |
| 6 | Create frontend utilities: `downsample.ts`, `quantizeTime.ts` | Foundation for frontend changes. No dependencies on backend changes. | Unit tests: downsample reduces points correctly, quantizeTime rounds correctly |
| 7 | Create frontend components: `ChartSkeleton.tsx`, `LazyChart.tsx`, `useChartData.ts` | Building blocks for the Overview rewrite. Depends on step 6. | Components render in isolation; hook fires correct GraphQL query |
| 8 | Rewrite `Overview.tsx` to use per-chart queries + progressive loading; update `Chart.tsx` with downsampling; update `ChartGrid.tsx` with lazy loading | Full frontend integration. Depends on steps 6-7. | Charts load progressively; above-fold appear first; downsampled data renders fast; Apollo cache hits on repeat load |

## Risk Zones

| Risk Zone | Location | What Could Go Wrong | Mitigation | Severity |
|-----------|----------|-------------------|------------|----------|
| Parallel ClickHouse queries exhaust connection pool | `internal/analytics/service.go` (errgroup goroutines) + `internal/clickhouse/client.go` (connection) | 8 concurrent queries per request x N concurrent users = 8N simultaneous connections. Default pool size unknown. | Configure `MaxOpenConns=20, MaxIdleConns=10` in ClickHouse DSN. Add a semaphore (buffered channel of size 8) in the service to cap per-request concurrency. | high |
| Race conditions in new concurrent code | `internal/analytics/service.go:GetOverviewPage` | errgroup goroutines share `charts` slice. Concurrent append without synchronization causes data races. | Use a pre-allocated slice indexed by chart position (not append). Alternatively, collect results via channel. Always run `go test -race`. | high |
| Cache fills up and stops accepting entries | `internal/cache/cache.go` | If LRU eviction is implemented incorrectly, cache could degrade to the current behavior where it silently stops accepting new entries when full. | Comprehensive tests for cache-full scenarios. Monitor cache size and hit/miss rates. | medium |
| Singleflight amplifies errors | `internal/analytics/service.go:loadChart` | If a ClickHouse query fails, all waiters in the singleflight group get the same error. This is actually correct behavior (skip-and-continue), but it means one transient error affects multiple requests. | The existing skip-and-continue pattern handles this — failed charts are logged and omitted. No change needed, but worth monitoring error rates. | low |
| Frontend fires 9 parallel requests | `apps/dashboard/src/pages/Overview.tsx` | 9 concurrent GraphQL requests may hit browser's per-origin connection limit (6 for HTTP/1.1). If the API serves over HTTP/1.1, only 6 requests proceed in parallel, rest are queued. | Ensure API is served over HTTP/2 (multiplexing). Alternatively, batch first 4 (above-fold) then lazy-load rest. The lazy loading already mitigates this — only visible charts fire requests. | medium |
| Downsampling hides error spikes | `apps/dashboard/src/components/Chart.tsx` | LTTB at 500 points over 24h error_rate data (originally 1440 points at 1-min intervals, now 288 at 5-min) — visual spikes may be smoothed out. | 288 points at 5-min granularity is below the 500-point cap, so error_rate chart will NOT be downsampled. Only charts with >500 points (events_volume, top_events) will be downsampled. This is acceptable. | low |
| Apollo cache key stability | `apps/dashboard/src/pages/Overview.tsx` | If `quantizeTime` implementation has edge cases (e.g., midnight boundary, timezone issues), cache keys may still be unique per request. | Simple implementation: `Math.floor(timestamp / (5 * 60 * 1000)) * (5 * 60 * 1000)`. Test with known timestamps. | low |

## Backward Compatibility

### API Changes
No API changes. The GraphQL schema (`schema.graphql`) remains identical. The `overviewPage` query still works — it returns the same response shape but executes faster internally via errgroup parallelization. The `chartData` query (already in the schema) is now used by the frontend but was always available. No new fields, no removed fields, no type changes.

### Data Changes
No data schema changes. ClickHouse table structure is unchanged. The data returned by queries may differ slightly:
- `error_rate_over_time`: groups by 5-minute intervals instead of 1-minute (fewer rows, coarser granularity)
- `top_events_by_count`: returns max 500 rows instead of 10,000
- Cached responses may be up to TTL-duration stale (1-15 minutes depending on chart type)

These are behavior changes in data granularity, not schema changes.

### Behavioral Changes
- **Progressive loading:** Users see individual charts appearing as they load, instead of the entire page appearing at once. This is a UX improvement, not a regression.
- **Cache staleness:** Dashboard data may be up to 1-15 minutes old (depending on chart type TTL). Previously, every load was real-time from ClickHouse. For the error_rate chart (1-min TTL), staleness is minimal. For retention cohorts (15-min TTL), the data is inherently backward-looking, so staleness is acceptable.
- **Downsampled charts:** Charts with >500 data points will display a downsampled visual representation. The LTTB algorithm preserves the visual shape, but individual data points may not be present. This affects events_volume and top_events charts most.

## Critique Review

The design critic assessed this design as **DESIGN_APPROVED** with all criteria scoring PASS.

Key findings from the critique:
- **Feasibility:** PASS. All changes are grounded in actual code paths verified by reading the codebase. The approach uses well-established patterns (errgroup, singleflight, LTTB) and introduces no exotic dependencies.
- **Scope discipline:** PASS. All changes map directly to the agreed scope. The LRU eviction enhancement to the cache is the closest thing to scope extension, but it's necessary for the cache to function correctly under load — the current eviction strategy is broken (silently stops accepting entries when full with no expired items).
- **Change map completeness:** PASS. All 16 source files in the codebase were read. Every file that needs changes is identified with specific change descriptions.
- **Risk coverage:** PASS. Risks are specific: connection pool exhaustion with quantified concern (8N connections), race conditions with specific mitigation (pre-allocated slice), cache eviction failure with specific failure mode.

Minor observations incorporated:
- Added HTTP/2 consideration for frontend parallel request risk
- Clarified that error_rate chart at 5-minute granularity produces 288 points (below 500 cap), so it will NOT be downsampled
- Added explicit note about `SummaryData` type duplication between `analytics/models.go` and `clickhouse/client.go` (not addressed — out of scope, minor code smell)

## User Approval Log
- **Full-stack optimization:** Confirmed in Stage 3. User chose systematic approach over backend-only.
- **In-memory cache over Redis:** Confirmed in Stage 3. Sufficient for <50k DAU single-instance deployment.
- **P95 target (not P50):** Confirmed in Stage 3. Design targets P95 <2s.
- **Per-chart time ranges rejected:** Confirmed in Stage 3. All charts use the same time range.
- **Cache staleness acceptable:** Confirmed in Stage 3 (implicit). Per-chart-type TTLs (1m-15m) balance freshness vs. performance.
- **GraphQL API unchanged:** Confirmed in Stage 3. No breaking changes to the schema.
- **errgroup for parallelization:** Design decision — approved (no user objection, standard Go pattern).
- **5-minute time range quantization:** Design decision — approved (minimal data accuracy impact, major cache benefit).
- **LTTB downsampling at 500 points:** Design decision — approved (preserves visual accuracy per agreed model).
