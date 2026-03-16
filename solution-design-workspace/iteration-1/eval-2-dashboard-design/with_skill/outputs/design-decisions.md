# Design Decisions

> Task: Optimize analytics dashboard overview page from 8-10s to <2s P95 load time
> Total decisions: 9
> User-approved: 7 of 9

## Decision 1: Use errgroup for Backend Query Parallelization

**Decision:** Replace the sequential for-loop in `GetOverviewPage` (`service.go:30-37`) with `errgroup`-based parallel execution where each chart query runs in its own goroutine.

**Context:** The primary performance bottleneck is sequential execution of 8 ClickHouse queries. Each query takes 2-5 seconds for large tenants, making total time 16-40 seconds (sum of all queries). Parallel execution reduces this to the duration of the slowest single query (2-5 seconds).

**Reasoning:** `errgroup` from `golang.org/x/sync` is the idiomatic Go pattern for structured concurrent work. It provides automatic context cancellation on error, simple error collection, and clean goroutine lifecycle management. The codebase currently has zero concurrency patterns — errgroup introduces a well-tested, widely-understood one. The existing skip-and-continue error pattern (`service.go:33-34`) maps naturally to errgroup: each goroutine can catch its own errors and skip failed charts without canceling others.

**Alternatives considered:**
- **Raw goroutines + sync.WaitGroup:** Provides parallelism but requires manual error handling, no context cancellation, and more boilerplate. Rejected because errgroup provides these features for free.
- **Worker pool pattern:** A fixed-size pool of goroutines processing chart requests from a queue. Rejected because we have exactly 8 known tasks — a pool is over-engineered for a fixed small workload.
- **Parallelize at the GraphQL resolver layer using gqlgen dataloaders:** gqlgen supports dataloaders for deduplication, but they are designed for N+1 query resolution, not parallelizing independent queries. The service layer is the right place for this orchestration.

**Trade-offs accepted:**
- Introduces a new concurrency pattern to a codebase that has none — developers must be comfortable with errgroup
- Increases per-request ClickHouse connection usage from 1 sequential to 8 simultaneous (mitigated by connection pool sizing)
- Requires careful handling of shared state (the results slice) to avoid data races

**User approval:** approved (systematic optimization direction confirmed in Stage 3)

**Impact:** `internal/analytics/service.go`, `cmd/analytics-api/main.go` (errgroup import), ClickHouse connection pool configuration

---

## Decision 2: Enhance Existing Cache with Per-Key TTL and LRU Eviction

**Decision:** Modify the existing `internal/cache/cache.go` by adding a `SetWithTTL` method for per-key TTL support and replacing the naive eviction strategy with LRU (Least Recently Used) eviction.

**Context:** The existing cache has two problems: (1) all items share the same TTL, but different chart types need different freshness (error_rate needs 1-min, retention needs 15-min), and (2) eviction removes the first expired item found and does nothing if no items are expired — meaning the cache silently stops accepting new entries when full (`cache.go:49-57`).

**Reasoning:** Enhancing the existing code is less disruptive than replacing it. The cache structure is sound — it has mutex-based concurrency, background cleanup, and a clean Get/Set interface. The two specific problems (uniform TTL and broken eviction) are fixable by adding a `lastAccessed` field to track LRU order and adding a `SetWithTTL` method that accepts per-item TTL.

**Alternatives considered:**
- **`patrickmn/go-cache` library:** Full-featured cache with per-item TTL. Rejected because it adds an external dependency for a problem solvable with ~30 lines of code changes.
- **`hashicorp/golang-lru` library:** Provides LRU but no per-key TTL. Would need to be wrapped with TTL logic, making it no simpler than enhancing the existing code.
- **Replace with completely new cache implementation:** Unnecessary — the existing cache's core structure (map + mutex + background cleanup) is correct.

**Trade-offs accepted:**
- LRU based on a `lastAccessed` field requires iterating the map to find the LRU item during eviction, which is O(n). For a cache of 1000 items this is negligible. If cache size grows significantly, a linked-list-based LRU would be needed.

**User approval:** approved (in-memory cache confirmed in Stage 3)

**Impact:** `internal/cache/cache.go`

---

## Decision 3: Use singleflight for Cache-Miss Deduplication

**Decision:** Use `sync/singleflight` in `loadChart` to ensure that concurrent requests for the same chart+tenant+timeRange only execute one ClickHouse query, with other requesters waiting for and sharing the result.

**Context:** With 50,000 DAU, many users from the same tenant will load the dashboard simultaneously, especially at start of business day. Without deduplication, a cache miss generates N identical ClickHouse queries (one per concurrent request). singleflight collapses these into 1 query with N receivers.

**Reasoning:** `sync/singleflight` (from `golang.org/x/sync`) is purpose-built for this pattern. The cache key used for lookup doubles as the singleflight key. On cache miss, `singleflight.Group.Do(key, queryFunc)` ensures only one goroutine executes the ClickHouse query while others block and receive the same result. This is the standard Go pattern for preventing thundering herd on cache miss.

**Alternatives considered:**
- **No deduplication (rely on cache alone):** Cache miss window still allows thundering herd. With 50k DAU and 1-15 min TTLs, every TTL expiration would trigger a burst of identical queries. Rejected because the risk is identified as high likelihood in the agreed model.
- **Distributed lock (Redis-based):** Would work across multiple API instances but requires Redis, which is out of scope. Single-instance deployment makes in-process singleflight sufficient.

**Trade-offs accepted:**
- If the shared query fails, all waiting callers receive the error. This is correct behavior given the skip-and-continue pattern, but amplifies the impact of transient errors.
- singleflight only deduplicates in-flight requests. It does not prevent sequential duplicate queries after the first completes (cache handles that).

**User approval:** approved (singleflight explicitly in agreed scope)

**Impact:** `internal/analytics/service.go` (new singleflight.Group field)

---

## Decision 4: Quantize Frontend Time Range to 5-Minute Boundaries

**Decision:** Round the `from` and `to` timestamps in the frontend to the nearest 5-minute boundary (floor) before including them as GraphQL query variables.

**Context:** The current code at `Overview.tsx:26-27` uses `new Date().toISOString()` which changes every millisecond. This means Apollo Client's cache key is unique on every page load, and the cache-first policy never produces a hit. Quantizing to 5-minute boundaries means all requests within the same 5-minute window share a cache key.

**Reasoning:** 5-minute quantization balances cache effectiveness with data freshness. Within any 5-minute window, all requests hit Apollo's cache after the first one. The backend also benefits because quantized time ranges are more likely to hit the server-side cache. 5 minutes is short enough that the error_rate chart (operational monitoring, 1-min cache TTL) still provides near-real-time data. The quantization function is trivial: `Math.floor(timestamp / 300000) * 300000`.

**Alternatives considered:**
- **1-minute quantization:** More granular but produces more unique cache keys and more frequent cache misses. The error_rate chart has a 1-min server TTL anyway, so 1-min client quantization doesn't add much benefit.
- **15-minute quantization:** Better cache hit rate but 15-minute-stale error_rate data is too stale for operational monitoring.
- **No quantization, use Apollo fetchPolicy: 'cache-and-network':** Would show stale data immediately and refresh in background. But the cache key problem remains — every request is unique, so there's no stale data to show.

**Trade-offs accepted:**
- Data boundaries shift slightly: a request at 10:03 sees data through 10:00, not 10:03. The 3-minute maximum staleness at the boundary is acceptable for dashboard use.

**User approval:** approved

**Impact:** `apps/dashboard/src/utils/quantizeTime.ts` (new), `apps/dashboard/src/hooks/useChartData.ts` (new), `apps/dashboard/src/pages/Overview.tsx` (uses new hook)

---

## Decision 5: Downsample Data on Frontend, Not Backend

**Decision:** Apply LTTB downsampling in the `Chart` component (frontend) before passing data to Recharts, rather than downsampling in the Go service or SQL queries.

**Context:** Charts with >500 data points cause significant Recharts rendering overhead (2-5s per the warning at `Chart.tsx:42`). Downsampling is needed to cap rendering work. The question is where in the stack to downsample.

**Reasoning:** Frontend downsampling keeps the GraphQL API contract unchanged — the `dataPoints` array in the response contains all data, and the frontend decides how to visualize it. This is safer because: (1) unknown API consumers may need full-resolution data, (2) the agreed scope explicitly excludes GraphQL API changes, and (3) the frontend knows the chart dimensions and can choose the optimal sample count based on pixel width.

**Alternatives considered:**
- **Backend downsampling in `loadChart`:** Would reduce network payload size. Rejected because it changes the API response (fewer data points than the query produced), which could affect unknown consumers and violates the API-unchanged constraint.
- **SQL-level downsampling (e.g., WITH FILL, GROUP BY larger intervals):** Would reduce data at the source. Rejected because different chart types need different downsampling strategies, and SQL-level downsampling is harder to make configurable.

**Trade-offs accepted:**
- Network payload is larger than necessary (full data transferred, downsampled on client). For 8 charts with ~500-1000 points each, the JSON payload is ~200-500KB — acceptable.
- CPU work happens on the client (downsampling computation). LTTB is O(n) and fast even for 10k points.

**User approval:** approved (downsampling to 500 points confirmed in agreed scope)

**Impact:** `apps/dashboard/src/utils/downsample.ts` (new), `apps/dashboard/src/components/Chart.tsx` (modified)

---

## Decision 6: Use LTTB Algorithm for Downsampling

**Decision:** Use the Largest-Triangle-Three-Buckets (LTTB) algorithm for downsampling time-series data points to a maximum of 500.

**Context:** Multiple downsampling algorithms exist. The choice affects visual fidelity of charts.

**Reasoning:** LTTB is specifically designed for visual downsampling of time-series data. It preserves the visual shape of the chart by keeping points that contribute most to the overall visual impression — peaks, valleys, and trend changes are preserved while flat regions are simplified. It runs in O(n) time and is straightforward to implement (~40 lines of code). It's widely used in charting libraries and has published academic backing.

**Alternatives considered:**
- **Every-Nth-point sampling:** Simple but can miss peaks and valleys entirely. A brief error spike could be completely invisible if it falls between sampled points.
- **Min-max bucketing:** Preserves extremes within each bucket but produces a jagged visual. Better for operational charts but worse for trend visualization.

**Trade-offs accepted:**
- LTTB is designed for line charts. For bar charts (top_events, conversion_funnel), it may not be optimal. However, these charts have relatively few data points (30 days of daily buckets = 30 points), so downsampling rarely triggers for them.

**User approval:** not required (implementation detail, does not affect user-visible behavior beyond the agreed 500-point cap)

**Impact:** `apps/dashboard/src/utils/downsample.ts`

---

## Decision 7: Change ExecuteQuery to Accept Variadic Parameters

**Decision:** Change `ExecuteQuery` from accepting a fixed `QueryParams` struct to accepting variadic `args ...any`, allowing queries with any number of parameters.

**Context:** The `user_retention_cohort` query has 5 `?` placeholders (tenant_id and time range appear twice — once for each subquery) but `ExecuteQuery` at `client.go:57` only passes 3 arguments from `QueryParams`. This is a confirmed bug: the query always fails with a parameter count mismatch.

**Reasoning:** Making params variadic is the simplest fix that handles both the bug and any future queries that might need different parameter counts. The calling code in `loadChart` constructs the argument list based on the query being executed. For the retention cohort query, it passes `tenantID, timeFrom, timeTo, tenantID, timeFrom, timeTo` (6 args for 5 placeholders — one extra tenantID for the subquery).

**Alternatives considered:**
- **Add `ExtraArgs []any` field to `QueryParams`:** Works but is an awkward API — callers must know which params go in the struct vs. the extra slice.
- **Create a separate `ExecuteQueryWithArgs` method:** Duplicates the query execution logic. The existing method can simply be made variadic.
- **Restructure the retention query to use CTEs instead of subqueries:** Would reduce the parameter count but changes the SQL semantics and adds complexity.

**Trade-offs accepted:**
- Variadic `args` loses the type safety of `QueryParams`. Callers must ensure they pass the right number of arguments in the right order. Since there's no test coverage, this is mitigated by careful code review and adding tests.

**User approval:** not required (internal implementation detail, bug fix)

**Impact:** `internal/clickhouse/client.go`, `internal/analytics/service.go` (calling code)

---

## Decision 8: Preserve overviewPage Query with Parallel Execution

**Decision:** Keep the `overviewPage` GraphQL query working and enhance it with internal errgroup parallelization, rather than removing or deprecating it.

**Context:** The frontend is switching from `overviewPage` (monolithic) to per-chart `chartData` queries. The question is what to do with the old `overviewPage` query.

**Reasoning:** The agreed scope states "GraphQL API contract unchanged" and the constraints analysis notes "unknown API consumers" may exist. Removing or deprecating `overviewPage` risks breaking unknown consumers. By keeping it and adding internal parallelization, any existing consumers automatically benefit from the performance improvement. The cost is minimal — `GetOverviewPage` is a thin wrapper that calls `loadChart` 8 times via errgroup.

**Alternatives considered:**
- **Remove `overviewPage` query:** Would simplify the API but breaks the backward compatibility constraint.
- **Deprecate with a warning header:** Adds unnecessary complexity. The query still works and is now fast — no reason to deprecate it.

**Trade-offs accepted:**
- The `overviewPage` query still returns all charts in a single response, which means the client still waits for the slowest chart. This is fine for non-dashboard consumers who may prefer a single request.

**User approval:** approved (backward compatibility confirmed in agreed scope)

**Impact:** `internal/analytics/service.go` (GetOverviewPage rewritten with errgroup but same signature), `internal/analytics/resolver.go` (no changes)

---

## Decision 9: Per-Chart-Type Cache TTLs

**Decision:** Configure different cache TTL durations for different chart types based on their data freshness requirements.

**Context:** The 8 charts have different update frequencies and freshness needs. Error rate is operational (needs near-real-time). Retention cohorts are backward-looking (can tolerate minutes of staleness). Using a single TTL either makes error_rate too stale or causes too many cache misses for retention.

**Reasoning:** Per-chart TTLs optimize cache hit rate while respecting freshness requirements for each chart type. The `chartCacheTTLs` map in the service defines:
- `error-rate`: 1 minute (operational monitoring, needs freshness)
- `events-volume`, `active-users`, `session-duration`: 5 minutes (moderate freshness)
- `top-events`, `conversion-funnel`, `geo-distribution`: 10 minutes (aggregate data, changes slowly)
- `user-retention`: 15 minutes (backward-looking cohort analysis, very slow to change)

**Alternatives considered:**
- **Uniform 5-minute TTL for all charts:** Simpler configuration but error_rate data would be 5 minutes stale (too much for incident monitoring) and retention data would cause unnecessary cache misses every 5 minutes.
- **No caching, rely only on singleflight:** Would still hit ClickHouse on every unique request. Without caching, the 2s P95 target is unlikely for large tenants on cache-cold requests.

**Trade-offs accepted:**
- More configuration complexity — 8 TTL values to maintain
- If a new chart type is added, a TTL must be configured (should default to a reasonable value like 5m)

**User approval:** approved (cache strategy confirmed in Stage 3; per-chart TTLs are a natural extension)

**Impact:** `internal/analytics/service.go` (new `chartCacheTTLs` map)

---

## Deferred Decisions

- **ClickHouse connection pool exact sizing:** Deferred to implementation/load testing. The design recommends `MaxOpenConns=20, MaxIdleConns=10` as a starting point, but the optimal values depend on ClickHouse cluster capacity and actual concurrent user patterns. Should be revisited after initial deployment with monitoring.
- **Cache max size tuning:** Set to 1000 entries initially. With 8 charts per tenant and TTLs of 1-15 minutes, the number of active cache entries depends on the number of active tenants. Should be monitored and adjusted based on cache hit/miss rates and memory usage.
- **SummaryData type duplication cleanup:** `SummaryData` is defined in both `internal/analytics/models.go:37-42` and `internal/clickhouse/client.go:132-137`. This is a code smell but out of scope for this task. Should be addressed in a future cleanup.
- **HTTP/2 for the API server:** The design notes that 9 parallel frontend requests may hit HTTP/1.1's per-origin connection limit. Whether to enable HTTP/2 depends on the deployment infrastructure (load balancer, TLS termination). Deferred to ops team.
