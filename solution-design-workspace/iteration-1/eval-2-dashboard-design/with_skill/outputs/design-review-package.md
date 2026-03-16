# Design Review Package

> Task: Optimize analytics dashboard overview page from 8-10s to <2s P95 load time
> Solution direction: systematic — full-stack optimization
> Changes: 7 files modified, 5 new, 0 deleted across 5 modules

## Proposed Solution Summary

The dashboard overview page is optimized across three layers. On the backend, the 8 sequential ClickHouse queries are executed in parallel using Go's errgroup, reducing server-side time from the sum of all queries (~16-40s for large tenants) to the duration of the slowest single query (~2-5s). Results are cached in-memory with per-chart TTLs (1-15 minutes depending on freshness needs), and concurrent identical requests are deduplicated using singleflight to prevent thundering herd on cache misses. On the frontend, the single monolithic GraphQL request is split into 9 independent requests (1 summary + 8 charts), each loading and rendering independently. Above-fold charts appear immediately while below-fold charts lazy-load on scroll. Data is downsampled to 500 points maximum before chart rendering to eliminate SVG rendering bottlenecks. The retention cohort chart bug (parameter count mismatch) is fixed as part of the query layer changes.

## Key Changes

### Backend: Parallel Query Execution
The `GetOverviewPage` function currently runs 8 ClickHouse queries one after another. After the change, all 8 run simultaneously via errgroup. The existing error handling (skip failed charts, continue) is preserved. The `overviewPage` GraphQL query continues to work with the same response shape — it's just faster.

### Backend: In-Memory Caching with Singleflight
A cache layer is added between the analytics service and ClickHouse. Each chart query result is cached with a chart-type-specific TTL: error_rate (1 min), event volume (5 min), retention cohorts (15 min). When the cache is empty (cold start or TTL expired), singleflight ensures that concurrent requests from multiple users of the same tenant share a single ClickHouse query rather than each triggering their own.

### Backend: Bug Fix and Query Optimization
The retention cohort query has a parameter mismatch (5 SQL placeholders but only 3 values passed) — this is fixed. The error_rate query granularity is coarsened from 1-minute to 5-minute intervals (reducing rows from 1440 to 288 for a 24-hour range). The top_events query limit is reduced from 10,000 to 500.

### Frontend: Per-Chart Progressive Loading
Instead of waiting for all 8 charts to load before showing anything, each chart loads independently. Users see individual charts appearing as their data arrives. The summary bar (total users, active users, etc.) loads via its own lightweight query and appears first.

### Frontend: Cache Fix and Downsampling
The time range in GraphQL requests is quantized to 5-minute boundaries so Apollo Client's cache produces hits on repeat loads (currently, millisecond-precision timestamps make every request unique). Charts with more than 500 data points are downsampled using the LTTB algorithm before rendering, eliminating the 2-5 second SVG rendering overhead for large datasets.

## Approval Points

### Point 1: Cache Staleness — Per-Chart TTLs

**Context:** Adding caching means dashboard data may not be perfectly real-time. Different charts have different freshness needs.

**Options:**
- **Option A: Per-chart TTLs (1m for error_rate, 5m for volume, 15m for retention)** — Balances freshness with performance. Error monitoring stays near-real-time; slow-changing charts benefit from longer caching.
- **Option B: Uniform 5m TTL for all charts** — Simpler but makes error_rate data 5 minutes stale (risky for incident monitoring).
- **Option C: No caching (singleflight only)** — Every request hits ClickHouse on cache miss. Safest for freshness but unlikely to meet 2s P95 for large tenants consistently.

**Recommendation:** Option A. Per-chart TTLs match the natural update frequency of each chart type. The error_rate chart at 1-minute TTL is only slightly delayed, while retention cohorts at 15 minutes benefit greatly from caching without any user-visible staleness.

**Question:** Are the proposed TTLs acceptable? Specifically, is 1-minute staleness OK for the error_rate chart?

---

### Point 2: Error Rate Chart Granularity Change

**Context:** The error_rate chart currently groups data by 1-minute intervals, producing up to 1,440 data points for a 24-hour range. The design changes this to 5-minute intervals (288 points). This is a data resolution change.

**Options:**
- **Option A: Change to 5-minute intervals** — Fewer data points, still shows trends. A 5-minute error spike is visible; a 1-minute spike is averaged into a 5-minute bucket.
- **Option B: Keep 1-minute intervals** — Preserves full resolution. With the 500-point downsampling cap, 1,440 points would be downsampled to 500 for rendering, but the full data is still transferred.

**Recommendation:** Option A. With the backend coarsened to 5 minutes, the query returns fewer rows (less ClickHouse work, less network transfer), and the 288 points are below the 500-point downsampling threshold so the chart renders at full 5-minute resolution. Option B works too but transfers more data for marginal benefit.

**Question:** Is 5-minute granularity acceptable for error rate monitoring on this dashboard? (Note: this is the overview dashboard, not a dedicated monitoring tool.)

---

### Point 3: Frontend Request Pattern — 9 Parallel Requests

**Context:** The design changes from 1 large GraphQL request to 9 smaller ones (1 summary + 8 charts). This improves perceived performance (charts appear independently) but changes the network pattern.

**Options:**
- **Option A: 9 parallel requests with lazy loading** — Above-fold charts (first 4) fire immediately; below-fold charts (last 4) fire on scroll. Maximum 5 concurrent requests initially.
- **Option B: 9 parallel requests, all at once** — All charts fire immediately. Fastest total load but may hit browser connection limits on HTTP/1.1.

**Recommendation:** Option A. Lazy loading reduces initial request count and means below-fold charts never load if the user doesn't scroll. This also reduces ClickHouse load for users who only check the top charts.

**Question:** Is lazy loading for below-fold charts acceptable, or should all charts load eagerly?

---

### Point 4: Backward Compatibility — Keeping overviewPage Query

**Context:** The frontend is switching to per-chart queries, but the `overviewPage` GraphQL query is preserved and enhanced with parallel execution.

**Recommendation:** Keep it. The performance improvement benefits any existing consumers automatically. No action needed from the user.

**Question:** Confirming: no known consumers besides the dashboard frontend use the `overviewPage` query?

## Risk Zones

- **ClickHouse connection pool under parallel load:** 8 concurrent queries per request multiplied by concurrent users could exhaust connections. Mitigated by explicit connection pool sizing (20 max connections) and the fact that caching reduces the frequency of ClickHouse queries significantly.
- **Race conditions in new concurrent code:** The codebase has no existing concurrency patterns. Introducing errgroup requires careful handling of shared state. Mitigated by using pre-allocated result slices (no concurrent append) and running `go test -race` in CI.
- **Cache cold start after deployment:** Immediately after deploy, all caches are empty. The first wave of requests all hit ClickHouse. Mitigated by singleflight (deduplicates concurrent cache misses) and the fact that the backend is now parallel (even uncached requests complete in 2-5s instead of 16-40s).
- **9 parallel frontend requests on HTTP/1.1:** Browsers limit concurrent connections per origin to 6 on HTTP/1.1. The lazy loading pattern mitigates this (only 5 requests fire initially: 1 summary + 4 above-fold charts).

## Scope Confirmation

**In scope (per agreed model):**
- Backend errgroup parallelization
- In-memory caching with per-chart TTLs
- singleflight for cache miss deduplication
- Retention cohort bug fix
- Frontend per-chart queries with progressive loading
- Apollo cache-busting fix (time range quantization)
- Data downsampling (LTTB, 500 point cap)
- Lazy loading for below-fold charts

**Not in scope (confirmed):**
- ClickHouse schema changes (materialized views, secondary indexes)
- Chart library switch (Recharts stays)
- GraphQL API breaking changes
- Real-time streaming / WebSocket updates
- Per-chart time range configuration

**Question:** Does this implementation scope match what you agreed to? Anything missing or extra?
