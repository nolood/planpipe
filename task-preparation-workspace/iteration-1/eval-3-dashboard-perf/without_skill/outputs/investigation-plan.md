# Investigation Plan: Dashboard Performance

## Phase 1: Instrument and Measure (1-2 days)

**Goal**: Establish a precise baseline and identify which zone (DB / API / Frontend) accounts for the majority of the 8-10 seconds.

### Steps

1. **Add end-to-end timing to the overview page load.**
   - Frontend: Record timestamps at navigation start, first GraphQL request sent, last GraphQL response received, last chart rendered.
   - API: Log wall-clock time per resolver and per ClickHouse query.
   - ClickHouse: Pull `system.query_log` for the relevant queries.

2. **Capture a representative trace.**
   - Load the overview page with browser DevTools Network + Performance tabs open.
   - Record: total GraphQL request count, payload sizes, JS execution time, render time.
   - Screenshot the waterfall and performance flame chart.

3. **Categorize the time.**
   - DB time: sum of ClickHouse query execution times.
   - API overhead: total API response time minus DB time.
   - Network: time from request sent to response received minus API processing.
   - Frontend: time from response received to chart rendered.

**Exit criteria**: A breakdown like "DB: 5s, API overhead: 1s, Network: 0.5s, Frontend: 3s" that tells us where to focus.

## Phase 2: Quick Wins (2-3 days)

Based on the most common patterns for this type of issue, these are likely applicable regardless of Phase 1 findings:

1. **Parallelize chart data fetching.**
   - Ensure all chart queries fire concurrently, not sequentially.
   - If using Apollo Client, check that queries aren't accidentally serialized.

2. **Add API-level response caching.**
   - Redis or in-memory cache with a short TTL (30-60s).
   - For 50k DAU, even a 30s cache dramatically reduces ClickHouse load.
   - Cache key: query hash + time range + user segment.

3. **Lazy-load below-fold charts.**
   - Use IntersectionObserver or equivalent to defer loading charts not visible on initial viewport.
   - Immediately reduces perceived load time.

4. **Reduce payload size.**
   - If the API returns more fields than the charts need, trim the GraphQL selection.
   - If returning per-minute data for a 30-day range (43,200 points), pre-bucket to hourly (720 points) or daily (30 points) depending on chart resolution.

## Phase 3: Targeted Optimization (3-5 days)

Depending on Phase 1 findings, pursue the relevant track:

### Track A: ClickHouse is the bottleneck (> 3s in DB)
- Analyze query plans with `EXPLAIN`.
- Create materialized views for the dashboard's most common aggregation patterns.
- Ensure queries leverage ClickHouse partition pruning (filter by date partition).
- Consider pre-computed rollup tables (hourly/daily aggregates) populated by ClickHouse's built-in materialized view engine.
- Review `ORDER BY` key of main tables -- ensure it aligns with dashboard filter patterns.

### Track B: API is the bottleneck (> 2s in API overhead)
- Profile resolver execution -- look for N+1 patterns, unnecessary data transformation.
- Implement DataLoader for batching if multiple resolvers hit ClickHouse.
- Move heavy aggregation logic into ClickHouse (SQL) instead of doing it in application code.
- Consider query complexity limits to prevent expensive GraphQL queries.

### Track C: Frontend is the bottleneck (> 2s in rendering)
- Profile with React Profiler / Chrome Performance tab.
- Downsample data points before rendering (e.g., Largest Triangle Three Buckets algorithm).
- Switch to a canvas-based renderer if using SVG with many data points (e.g., ECharts canvas mode).
- Implement virtualization for table/list components if present.
- Code-split the dashboard page and lazy-load chart library chunks.
- Check for unnecessary re-renders triggered by global state changes.

## Phase 4: Validate and Harden (1-2 days)

1. **Load test at 50k DAU equivalent.**
   - Simulate concurrent overview page loads.
   - Verify < 2s target holds under load, not just in dev.

2. **Add performance monitoring.**
   - Set up Real User Monitoring (RUM) for the overview page.
   - Alert if p95 load time exceeds 2s.
   - Track ClickHouse query times in dashboards (meta!).

3. **Document the optimizations.**
   - What was changed and why.
   - Cache invalidation strategy.
   - Rollup table maintenance.

## Estimated Total Timeline: 7-12 days
