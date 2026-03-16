# Design Decisions -- Dashboard Performance Optimization

## Decision 1: errgroup over raw goroutines for parallelization

**Context:** `GetOverviewPage` in `service.go:24-55` loads 8 charts sequentially. Parallelization requires running them concurrently.

**Options considered:**
1. **Raw goroutines + sync.WaitGroup** -- Manual goroutine management, manual error collection.
2. **errgroup** (chosen) -- Structured concurrency with context propagation, limit control, and clean error semantics.
3. **Dataloader at the GraphQL layer** -- gqlgen supports dataloaders but they are designed for batching N+1 queries, not for parallelizing independent queries. Would require schema restructuring.

**Decision:** errgroup from `golang.org/x/sync/errgroup`.

**Rationale:**
- errgroup provides `SetLimit()` for concurrency control, which prevents spawning unbounded goroutines.
- It propagates context cancellation -- if the HTTP request is cancelled, all in-flight queries abort.
- The skip-and-continue pattern is preserved by having each goroutine return `nil` on error (logging the error but not failing the group).
- errgroup is the standard Go pattern for "fan-out, fan-in" and will be recognizable to any Go developer.
- Raw goroutines would require manually implementing the same features (wait group, context propagation, concurrency limits).

**Trade-off:** errgroup's `Wait()` blocks until all goroutines complete. If one chart query is very slow (e.g., 5s), all charts wait for it. This is acceptable because the total time is still max(queries) instead of sum(queries). A streaming approach (return charts as they complete) would require WebSockets, which is out of scope.

---

## Decision 2: Skip-and-continue preserved via nil-return pattern

**Context:** The current code at `service.go:33-34` logs errors and continues (`continue` in the loop). This resilience pattern must be preserved in the parallel version.

**Options considered:**
1. **Return error from goroutine, use errgroup error aggregation** -- errgroup cancels remaining goroutines on first error. This breaks skip-and-continue.
2. **Return nil from goroutine on error** (chosen) -- Each goroutine handles its own error (logs it) and returns `nil`. Failed charts produce a `nil` entry in the results slice, which is filtered out.
3. **Custom error aggregation** -- Collect errors separately, return partial results. More complex than needed.

**Decision:** Return `nil` from goroutines on error. Filter `nil` results after `Wait()`.

**Rationale:** This is the most direct translation of the existing skip-and-continue pattern into concurrent code. It requires no custom error handling infrastructure. The goroutine writes `nil` to its slot in the results array (by not writing anything, since the array is zero-initialized with `nil` pointers).

---

## Decision 3: In-memory cache with per-key TTL over external cache (Redis)

**Context:** The cache at `internal/cache/cache.go` is an in-memory TTL cache that was never wired in. The system needs caching to reduce ClickHouse load under 50k DAU.

**Options considered:**
1. **Redis/Memcached** -- External distributed cache. Supports multiple API server instances, persistent across deploys.
2. **In-memory cache with per-key TTL** (chosen) -- Process-local cache with chart-type-specific TTLs.
3. **CDN/HTTP-level caching** -- Cache GraphQL responses at the HTTP layer. Coarse-grained, harder to invalidate per chart.

**Decision:** Upgrade the existing in-memory cache with per-key TTL and proper LRU eviction.

**Rationale:**
- The user explicitly confirmed in-memory cache is sufficient for <50k DAU (Stage 3 accepted assumptions).
- The existing cache code provides the foundation -- it just needs per-key TTL and better eviction.
- No infrastructure changes are needed (no Redis deployment, no additional failure modes).
- Per-key TTL enables differentiated freshness: operational charts (error-rate) get 1-minute TTL while slow-changing charts (retention) get 30-minute TTL.
- If the system scales to multiple API instances, migrating to Redis is a future, bounded change (swap the `cache.Cache` implementation behind the same interface).

**Trade-off:** Cold starts after deploys cause all requests to hit ClickHouse. Singleflight mitigates the thundering herd, but the first few requests will be slow. This is acceptable because deploys are infrequent and the cache warms within seconds under 50k DAU load.

---

## Decision 4: LRU eviction over the existing naive eviction

**Context:** The current cache at `cache.go:49-57` has a broken eviction strategy: when full, it iterates the map looking for an expired item. If none are expired, new entries are silently dropped.

**Options considered:**
1. **Keep naive eviction, increase cache size** -- Band-aid. Eviction still silently fails when the cache fills with unexpired items.
2. **LRU eviction** (chosen) -- Track access order, evict least-recently-used item when full.
3. **LFU (Least Frequently Used)** -- More complex, better for workloads with popular items. Overkill for this use case.
4. **Use a battle-tested library (e.g., github.com/hashicorp/golang-lru)** -- Adds a dependency but is well-tested.

**Decision:** Implement simple LRU using an order slice. Consider switching to `hashicorp/golang-lru` if the implementation proves buggy.

**Rationale:**
- LRU is the right fit because recently-viewed dashboards are likely to be viewed again soon.
- A slice-based LRU is simple to implement (move accessed key to end, evict from front).
- The cache size is bounded (1000 entries) so the O(n) operations on the order slice are negligible.
- Adding `hashicorp/golang-lru` is a viable alternative that could be swapped in later, but starting with a custom implementation keeps dependencies minimal and is more educational for the team.

**Trade-off:** The slice-based LRU has O(n) complexity for the "touch" operation (moving a key to the end). For 1000 entries this is microseconds. A proper doubly-linked-list + map implementation would be O(1) but more complex. The custom implementation may have subtle bugs; thorough testing in `cache_test.go` is critical.

---

## Decision 5: singleflight for cache miss deduplication

**Context:** With 50k DAU, many concurrent requests for the same tenant's dashboard will arrive simultaneously. On cache miss (cold start or TTL expiry), all concurrent requests would query ClickHouse independently.

**Options considered:**
1. **No deduplication** -- Let all concurrent misses hit ClickHouse. Simple, but wastes resources.
2. **singleflight** (chosen) -- Standard Go pattern for deduplicating in-flight calls. If a query for key K is already running, subsequent requests for K wait for the first result.
3. **Cache pre-warming** -- Proactively populate cache before TTL expires. More complex, requires knowing which keys to warm.

**Decision:** Use `sync/singleflight` in `loadChart` to deduplicate concurrent cache misses for the same cache key.

**Rationale:**
- singleflight is a stdlib-adjacent pattern (`golang.org/x/sync/singleflight`) that is well-understood in the Go ecosystem.
- It perfectly addresses the thundering herd risk identified in the Stage 3 risk analysis.
- The cache key already encodes tenant + chart + quantized time range, so singleflight naturally deduplicates at the right granularity.
- The "double-check" pattern (check cache inside the singleflight callback) handles the race between the first request populating the cache and subsequent requests entering singleflight.

**Trade-off:** If the first request for a key fails, all waiting requests also fail. This is acceptable because:
1. The skip-and-continue pattern means a failed chart is logged and skipped, not retried.
2. Subsequent requests (next page load) will attempt a fresh query.

---

## Decision 6: Per-chart frontend queries over monolithic query

**Context:** The frontend currently uses `OVERVIEW_PAGE_QUERY` which fetches all 8 charts in one GraphQL request. The backend resolves this into 8 sequential ClickHouse queries (now parallel with the backend changes). The user sees nothing until all 8 charts are ready.

**Options considered:**
1. **Keep monolithic query, rely on backend parallelization** -- Backend returns all charts in ~5s. Frontend still shows nothing for 5s.
2. **Per-chart queries** (chosen) -- Frontend fires 8 independent `CHART_DATA_QUERY` requests. Each chart renders as soon as its data arrives.
3. **GraphQL subscriptions** -- Stream chart data as it becomes available. Requires WebSocket infrastructure, which is out of scope.
4. **GraphQL @defer directive** -- Incrementally deliver parts of a response. gqlgen supports @defer experimentally (v0.17.45), but Apollo Client 3.9 support is limited. Too risky.

**Decision:** Switch the frontend to 8 independent `CHART_DATA_QUERY` calls, one per chart.

**Rationale:**
- The `chartData` query already exists in `schema.graphql` line 3 and has a working resolver at `resolver.go:40-46`. No backend changes are needed.
- Progressive loading transforms the perceived performance: the first chart appears in <1s, while the monolithic query would show nothing for 5s even after backend parallelization.
- Each chart has independent loading/error states, improving UX resilience (one failed chart doesn't block others).
- 8 parallel HTTP requests is well within browser limits (6-8 concurrent connections per domain for HTTP/1.1, unlimited for HTTP/2).

**Trade-off:**
- 8 requests instead of 1 increases total HTTP overhead (~400 bytes per request header x 8 = ~3.2KB extra). Negligible.
- The backend's `GetOverviewPage` parallelization becomes less critical for the frontend flow (each `GetChartData` call is a single query). However, `GetOverviewPage` may still be used by other consumers, so the parallelization is still valuable.
- Apollo Client will manage 8 independent cache entries instead of 1 monolithic one. This is actually better for cache invalidation granularity.

---

## Decision 7: Time range quantization at 5-minute intervals

**Context:** `Overview.tsx:26-27` uses `new Date().toISOString()` for query variables, producing millisecond-precision timestamps that create unique cache keys on every request.

**Options considered:**
1. **Quantize to 1 minute** -- Minimal data staleness, but still frequent cache misses.
2. **Quantize to 5 minutes** (chosen) -- Balances data freshness with cache effectiveness.
3. **Quantize to 15 minutes** -- Very effective caching, but data can be up to 15 minutes stale.
4. **Use server-generated timestamps** -- Backend controls the time range. Breaks the frontend's time range picker UX.

**Decision:** Quantize to 5-minute boundaries using `Math.floor(timestamp / (5 * 60 * 1000)) * (5 * 60 * 1000)`.

**Rationale:**
- 5-minute quantization means the dashboard shows data that is at most 5 minutes old. For a dashboard that is already querying 7-30 day windows, 5 minutes of staleness is imperceptible.
- It aligns with the backend cache TTLs (shortest is 1 minute for error-rate). The frontend and backend caching strategies are coherent.
- `useMemo` with the quantized value as a dependency prevents unnecessary re-renders within the same 5-minute window.

**Trade-off:** Users who rapidly reload the page within 5 minutes will see the same data. This is the intended behavior -- it turns unnecessary ClickHouse queries into Apollo cache hits.

---

## Decision 8: Bucket averaging for downsampling over LTTB

**Context:** Charts can receive 1,000-10,000+ data points from the backend. Recharts renders each as an SVG element, causing 2-5 second rendering for large datasets.

**Options considered:**
1. **Largest-Triangle-Three-Buckets (LTTB)** -- Produces visually optimal downsampled line charts by preserving visual extrema. Requires an external library or ~60 lines of implementation.
2. **Bucket averaging** (chosen) -- Divide data into N buckets, average values within each bucket. Simple, chart-type-agnostic.
3. **Min-max bucketing** -- Keep min and max per bucket to preserve spikes. Doubles the output size vs. averaging.
4. **Server-side downsampling** -- Add downsampling to the Go service layer. Reduces network transfer but adds backend complexity.

**Decision:** Client-side bucket averaging, capped at 500 points for line/area charts and 100 for bar charts.

**Rationale:**
- Bucket averaging is simple to implement (~25 lines), works for all chart types, and produces visually acceptable results for trend charts.
- 500 points for a line chart rendered at ~1000px width means ~2 pixels per point, which is at or below the visual resolution limit. More points would be invisible.
- Client-side downsampling means the backend code and API contract are unchanged. The frontend receives all data and decides how much to render.
- The decision to not use LTTB is pragmatic: it adds complexity for marginal visual improvement at 500 points. If users report visual quality issues, LTTB can be swapped in behind the same interface.

**Trade-off:**
- Bucket averaging can hide short-duration spikes. For the error-rate chart (operational), this is mitigated by the backend query coarsening (5-minute buckets, max 1000 rows), which already limits the data to a displayable volume.
- Downloading large datasets only to discard most of them on the client wastes network bandwidth. Server-side downsampling would be more efficient but requires backend changes and couples the API to the UI's viewport. This can be added later as an optimization.

---

## Decision 9: IntersectionObserver for lazy loading over react-virtualized

**Context:** The `ChartGrid` renders all 8 charts simultaneously. Below-fold charts (charts 5-8 in a 2-column grid) are not visible on initial load.

**Options considered:**
1. **IntersectionObserver** (chosen) -- Native browser API. Observe placeholder elements, load chart when it enters the viewport.
2. **react-virtualized / react-window** -- Renders only visible items. Designed for long lists (100+ items), overkill for 8 charts.
3. **Manual scroll listener** -- Debounced scroll event handler. Less efficient than IntersectionObserver, more code.
4. **requestIdleCallback** -- Load below-fold charts when browser is idle. Non-deterministic timing.

**Decision:** IntersectionObserver with a 200px rootMargin.

**Rationale:**
- 8 charts is too few for virtualization libraries (designed for hundreds of items).
- IntersectionObserver is a native API supported in all modern browsers, requires no dependencies.
- The 200px rootMargin starts loading before the chart scrolls into view, hiding the loading latency.
- Implementation is ~15 lines of code in the `ChartWithLoading` component.

**Trade-off:** Below-fold charts will show a brief skeleton state when the user scrolls to them, even if the data would have been available had it been pre-fetched. The 200px rootMargin minimizes this, and on subsequent visits the Apollo cache eliminates it entirely.

---

## Decision 10: Keep the `overviewPage` GraphQL query for backward compatibility

**Context:** The frontend switches from `OVERVIEW_PAGE_QUERY` to per-chart `CHART_DATA_QUERY` calls. The `overviewPage` query in `schema.graphql` line 2 could be removed or deprecated.

**Options considered:**
1. **Remove the `overviewPage` query** -- Clean, but risky if unknown consumers use it.
2. **Keep and optimize it** (chosen) -- The backend parallelization improvements benefit any `overviewPage` callers. Mark it as deprecated in the code, but don't remove it.
3. **Keep as-is (no optimization)** -- Waste of the backend work.

**Decision:** Keep `overviewPage` in the schema and apply the backend optimizations (errgroup parallelization) to it. Mark the frontend import as deprecated.

**Rationale:**
- The Stage 2 analysis notes "Unknown API consumers: It is unknown whether other services consume the same GraphQL API." Removing the query could break unknown consumers.
- The backend parallelization benefits all callers of `GetOverviewPage`, including any unknown consumers.
- Marking it deprecated in the frontend code is a signal to the team, not a breaking change.

---

## Decision 11: Summary data loaded as a separate query

**Context:** Summary data (total users, active users, total events, conversion rate) is currently fetched as part of the monolithic `overviewPage` query. In the new per-chart pattern, it needs its own query.

**Options considered:**
1. **Include summary in the first chart query** -- Hacky, couples chart and summary data.
2. **Separate `SUMMARY_QUERY`** (chosen) -- Uses the existing `summary` query in `schema.graphql` line 4.
3. **Inline summary into the page without a query** -- Would require pre-computed data, which doesn't exist.

**Decision:** Use the existing `summary` GraphQL query as a separate frontend call.

**Rationale:**
- The `summary` query already exists in `schema.graphql` line 4 and has a resolver at `resolver.go:49-55`.
- Summary data is lightweight (4 scalar values) and loads fast (~200ms for 4 `QueryRow` calls).
- Having it as a separate query means the summary bar can render independently of chart data, improving perceived performance.

---

## Decision 12: Connection pool sized at 12 max connections

**Context:** Parallelizing 8 chart queries + 1 summary query per request requires adequate connection pool sizing.

**Options considered:**
1. **Default pool size** -- Unknown, likely insufficient for 8 concurrent queries per request.
2. **Pool size = 12** (chosen) -- 9 queries per request + 3 headroom for concurrent requests.
3. **Pool size = 50** -- Supports ~5 concurrent page loads with full parallelism. May overwhelm a single ClickHouse node.

**Decision:** `MaxOpenConns = 12`, `MaxIdleConns = 6`.

**Rationale:**
- With 8 parallel chart queries + 1 summary + headroom, 12 connections supports one fully parallel page load with room for concurrent requests to share connections.
- Setting it too high (50+) could overwhelm a single ClickHouse node. The docker-compose.yml shows a single `clickhouse-server` container.
- `MaxIdleConns = 6` keeps half the pool warm to avoid reconnection overhead on subsequent requests.
- If load testing shows connection starvation, the value can be increased. If ClickHouse shows resource pressure, it should be decreased.

**Trade-off:** With only 12 connections, a second concurrent page load will partially serialize. This is acceptable because:
1. The cache reduces ClickHouse load for popular tenants.
2. Singleflight deduplicates concurrent requests for the same tenant.
3. Most requests will hit the cache after initial warm-up.
