# Design Decisions

> Task: Reduce analytics dashboard load time from 8-10s to <2s (P95)
> Total decisions: 6
> User-approved: 2 of 6

## Decision 1: errgroup with Concurrency Limit of 8

**Decision:** Use `golang.org/x/sync/errgroup` to parallelize ClickHouse queries, with a concurrency limit of 8.

**Context:** The analytics service currently queries ClickHouse sequentially for each chart. With 6-8 charts, this takes 4+ seconds. Parallelization can reduce this to the duration of the slowest single query.

**Reasoning:** errgroup is the standard Go pattern for bounded parallel work with error handling. The limit of 8 matches the current maximum chart count on the overview page and prevents connection pool exhaustion.

**Alternatives considered:**
- **sync.WaitGroup:** No built-in error handling → Rejected because: need error propagation for skip-and-continue pattern
- **Worker pool library:** Over-engineered for this use case → Rejected because: errgroup is simpler and sufficient

**Trade-offs accepted:**
- Connection pool must handle 8 concurrent connections (current pool size is 20, so headroom exists)
- If chart count exceeds 8, concurrency won't scale beyond the limit (acceptable — current max is 8)

**User approval:** not required

**Impact:** Analytics service (`service.go`)

---

## Decision 2: In-Memory Cache (Not Redis)

**Decision:** Use an in-memory cache (Go map with mutex) instead of Redis for chart data caching.

**Context:** Chart data changes infrequently (most charts update on 30s-5min cadence). Caching eliminates redundant ClickHouse queries for repeated page loads.

**Reasoning:** User confirmed single-instance deployment. In-memory cache provides sub-millisecond reads without network latency or operational overhead of Redis. Cache is rebuilt on process restart (cold start handled by singleflight).

**Alternatives considered:**
- **Redis:** Survives restarts, shareable across instances → Rejected because: user confirmed single instance; network latency unnecessary
- **SQLite cache:** Persistent local cache → Rejected because: adds complexity; in-memory is sufficient

**Trade-offs accepted:**
- Cache lost on restart (singleflight prevents thundering herd)
- Not shareable across instances (single-instance deployment confirmed)

**User approval:** approved

**Impact:** New cache module (`internal/cache/`), analytics service integration

---

## Decision 3: singleflight for Cache Stampede Prevention

**Decision:** Use `golang.org/x/sync/singleflight` to deduplicate concurrent requests for the same chart data during cache misses.

**Context:** When cache is empty (cold start or TTL expiry), multiple concurrent requests for the same chart would all hit ClickHouse. singleflight ensures only one actual query runs and shares the result.

**Reasoning:** singleflight is a stdlib-adjacent package designed exactly for this pattern. Zero configuration, minimal overhead, battle-tested.

**Alternatives considered:**
- **Probabilistic early expiry:** Refresh before TTL expires → Rejected because: adds complexity, doesn't handle cold start
- **Per-key mutex:** Lock per chart ID → Rejected because: singleflight is cleaner and handles the same case

**Trade-offs accepted:**
- singleflight only deduplicates in-flight requests (not a persistent lock)
- If the single query fails, all waiters get the error (correct behavior — they should all retry)

**User approval:** not required

**Impact:** Cache module (`chart_cache.go`), analytics service

---

## Decision 4: LTTB Downsampling at 500 Points

**Decision:** Apply Largest-Triangle-Three-Buckets (LTTB) downsampling on the client side, reducing datasets to 500 points maximum.

**Context:** Some charts have thousands of data points (especially for long date ranges). Rendering all points is slow and provides no visual benefit at typical screen resolutions.

**Reasoning:** LTTB preserves the visual shape of time series data better than naive sampling. 500 points match the typical chart width in pixels (charts are ~500-600px wide). User confirmed this threshold.

**Alternatives considered:**
- **Min/max bucketing:** Preserve extremes in each bucket → Rejected because: doubles the point count and doesn't produce as clean a visual
- **Random sampling:** Simple but loses important features → Rejected because: can drop peaks/valleys
- **Server-side downsampling:** ClickHouse does the reduction → Rejected because: different users may have different screen sizes; client-side is more flexible

**Trade-offs accepted:**
- Some data fidelity loss (acceptable — zoom shows full data)
- Client-side CPU cost for downsampling (minimal — LTTB is O(n))

**User approval:** approved

**Impact:** Frontend (`downsample.ts`, `Chart.tsx`)

---

## Decision 5: Per-Chart GraphQL Queries

**Decision:** Split the single monolithic overview GraphQL query into independent per-chart queries.

**Context:** The current Overview.tsx fires one large GraphQL query that fetches all chart data. The page only renders after the entire response arrives. Per-chart queries enable progressive rendering.

**Reasoning:** Independent queries allow each chart to render as soon as its data arrives. This dramatically improves perceived performance — the user sees the first chart in <1s even if some charts take longer.

**Alternatives considered:**
- **GraphQL subscriptions:** Real-time updates via WebSocket → Rejected because: requires WebSocket infrastructure; progressive loading is simpler and sufficient
- **Server-Sent Events:** Stream chart data → Rejected because: requires new endpoint type; per-chart queries achieve the same progressive effect

**Trade-offs accepted:**
- More HTTP requests (6-8 instead of 1) — acceptable, HTTP/2 multiplexing handles this
- Slightly more complex frontend state management (each chart manages own loading state)

**User approval:** not required

**Impact:** Frontend (`Overview.tsx`, `useChartQuery.ts`), potentially API handler

---

## Decision 6: Intersection Observer for Lazy Loading

**Decision:** Use the native Intersection Observer API for lazy loading below-fold charts, rather than a scroll event listener or virtualization library.

**Context:** The overview page may have charts below the fold that don't need to load immediately. Deferring their data fetch saves ClickHouse queries and improves initial load time.

**Reasoning:** Intersection Observer is a native browser API with excellent support. No library needed. It integrates cleanly with the per-chart query pattern — each chart only fires its query when it becomes visible.

**Alternatives considered:**
- **Scroll event listener:** Polling-based, performance concerns → Rejected because: IntersectionObserver is purpose-built and more efficient
- **React virtualization library:** Full virtual scrolling → Rejected because: over-engineered for 6-8 charts; virtualization is for hundreds of items

**Trade-offs accepted:**
- Charts below fold have a brief loading state when scrolled into view (acceptable — skeleton state)

**User approval:** not required

**Impact:** Frontend (`ChartGrid.tsx`)

---

## Deferred Decisions

- **Cache warm-up strategy:** Whether to pre-populate cache on startup. Deferred because: singleflight handles cold start adequately. Can add warm-up if monitoring shows cold start is a problem.
- **Per-chart time ranges:** Allow each chart to have its own time range selector. Deferred because: user explicitly deferred this feature; not in agreed scope.
- **Materialized views:** ClickHouse materialized views for pre-aggregation. Deferred because: out of agreed scope; requires schema changes.
