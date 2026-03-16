# Analysis: Analytics Dashboard Performance

## Problem Decomposition

The 8-10 second load time has three potential bottleneck zones. Each must be investigated independently before optimizing, because fixing the wrong layer wastes effort.

### Zone 1: ClickHouse Query Layer
**Symptoms that would point here:**
- Slow query log in ClickHouse shows queries taking > 2s.
- Queries scan large amounts of data (full table scans, missing indexes/projections).
- Materialized views or pre-aggregations are absent.

**Common causes in ClickHouse dashboards:**
- Missing `ORDER BY` key alignment with query filters (ClickHouse is column-oriented; the primary key determines data locality).
- Querying raw event tables instead of pre-aggregated rollups.
- Unbounded time ranges or missing partition pruning.
- Large `GROUP BY` cardinality without `LIMIT`.

**Investigation steps:**
1. Enable ClickHouse `system.query_log` analysis -- find the exact queries the API sends for the overview page.
2. Run `EXPLAIN PIPELINE` / `EXPLAIN` on each query.
3. Check if materialized views or projections exist for the dashboard's aggregation patterns.
4. Measure query time in isolation (CLI or ClickHouse client) to separate DB time from API/network time.

### Zone 2: GraphQL API Layer (`services/analytics-api/`)
**Symptoms that would point here:**
- ClickHouse queries are fast (< 500ms) but API response time is still slow.
- N+1 resolver pattern: one GraphQL query triggers many sequential ClickHouse queries.
- Heavy post-processing/transformation in resolvers.
- No caching layer.

**Common causes:**
- GraphQL resolvers that issue one query per chart series instead of batching.
- Lack of DataLoader or equivalent batching mechanism.
- No response caching (Redis, in-memory, or CDN-level).
- Serialization overhead on large result sets (hundreds of thousands of rows returned to the client).

**Investigation steps:**
1. Add tracing/timing to each resolver (or check if OpenTelemetry/APM is already in place).
2. Count the number of ClickHouse queries per single page load.
3. Check if any caching exists (Redis, in-memory, HTTP cache headers).
4. Measure payload size of the GraphQL response.

### Zone 3: Frontend Rendering (`apps/dashboard/`)
**Symptoms that would point here:**
- API responds in < 1s but the page still takes 8-10s to become interactive.
- Browser DevTools shows long "Scripting" or "Rendering" blocks.
- Charts render tens of thousands of data points without downsampling.

**Common causes:**
- Rendering all data points in a charting library (e.g., Recharts, Chart.js, D3) without aggregation/downsampling.
- All charts fetch data simultaneously, causing a waterfall or memory pressure.
- No virtualization or lazy loading of off-screen charts.
- Large JavaScript bundle blocking initial render.
- Re-renders caused by state management issues (e.g., Redux store updates triggering full tree re-renders).

**Investigation steps:**
1. Profile in Chrome DevTools Performance tab -- identify long tasks.
2. Check how many data points each chart receives and renders.
3. Check if charts below the fold are loaded eagerly.
4. Measure JS bundle size and check for code splitting.
5. Check for unnecessary re-renders using React Profiler (if React).

## Likely Root Causes (Ranked by Probability)

Based on the symptom profile (8-10 seconds, ClickHouse + GraphQL, "large datasets"):

1. **High probability**: ClickHouse queries are scanning too much data -- missing materialized views or improper primary key usage for the dashboard's access patterns.
2. **High probability**: Too many data points sent to the frontend -- the API returns raw granularity data (e.g., per-minute for a 30-day range) instead of pre-bucketed aggregations.
3. **Medium probability**: N+1 query pattern in GraphQL resolvers -- each chart issues its own query serially.
4. **Medium probability**: Frontend renders all data points without downsampling -- charting library chokes on 100k+ points.
5. **Lower probability**: No caching at all -- every page load hits ClickHouse directly with no intermediate cache.

## Estimated Impact by Fix

| Fix | Expected Improvement | Effort |
|-----|---------------------|--------|
| Add materialized views / rollup tables in ClickHouse | 3-5x query speedup | Medium |
| Batch/parallelize resolver queries | 2-4x API speedup | Low-Medium |
| Add API-level caching (Redis, 30-60s TTL) | 5-10x for cache hits | Low |
| Downsample data before sending to frontend | 2-3x render speedup | Low |
| Lazy-load below-fold charts | Perceived 2-3x improvement | Low |
| Frontend data point downsampling/aggregation | 2-5x render speedup | Low-Medium |
