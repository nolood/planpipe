# Task Breakdown

## Epic: Analytics Dashboard Performance Optimization
**Target**: Overview page load time < 2 seconds (currently 8-10s)

---

### Task 1: Establish Performance Baseline
**Priority**: P0 (blocker for all other work)
**Estimate**: 1 day
**Assignee**: Fullstack engineer

- [ ] Instrument the overview page to log timing at each layer (frontend, API, DB).
- [ ] Capture and document a representative performance trace.
- [ ] Produce a time breakdown: what percentage of the 8-10s is DB, API, network, frontend.
- [ ] Identify the slowest individual charts and queries.
- [ ] Document findings with screenshots/traces.

**Output**: A written breakdown showing where time is spent, shared with the team.

---

### Task 2: Audit ClickHouse Queries
**Priority**: P0
**Estimate**: 1 day
**Assignee**: Backend engineer
**Depends on**: Task 1

- [ ] Extract all ClickHouse queries triggered by an overview page load.
- [ ] Run `EXPLAIN` on each query; document scan sizes and execution plans.
- [ ] Check for missing partition pruning, full table scans, high-cardinality GROUP BYs.
- [ ] Identify candidates for materialized views or rollup tables.
- [ ] Document query times in isolation (via ClickHouse client).

**Output**: List of queries with execution times, plans, and optimization recommendations.

---

### Task 3: Audit GraphQL Resolvers
**Priority**: P0
**Estimate**: 1 day
**Assignee**: Backend engineer
**Depends on**: Task 1

- [ ] Count the number of ClickHouse queries per page load (N+1 check).
- [ ] Profile resolver execution time (time in resolver minus time in DB).
- [ ] Measure GraphQL response payload sizes.
- [ ] Check for existing caching (Redis, HTTP headers, etc.).
- [ ] Check if DataLoader or batching is used.

**Output**: Resolver audit report with identified bottlenecks.

---

### Task 4: Audit Frontend Rendering
**Priority**: P0
**Estimate**: 1 day
**Assignee**: Frontend engineer
**Depends on**: Task 1

- [ ] Profile the overview page in Chrome DevTools Performance tab.
- [ ] Count data points rendered per chart.
- [ ] Check if below-fold charts are eagerly loaded.
- [ ] Measure JS bundle size for the dashboard route.
- [ ] Check for unnecessary re-renders (React Profiler if applicable).
- [ ] Identify the charting library and its rendering mode (SVG vs Canvas).

**Output**: Frontend performance audit with identified bottlenecks.

---

### Task 5: Implement API Response Caching
**Priority**: P1
**Estimate**: 1-2 days
**Assignee**: Backend engineer
**Depends on**: Task 3

- [ ] Add Redis (or in-memory) cache for GraphQL responses.
- [ ] Define cache key strategy (query hash + parameters + time bucket).
- [ ] Set appropriate TTL based on data freshness requirements.
- [ ] Add cache hit/miss metrics.
- [ ] Test under concurrent load.

---

### Task 6: Optimize ClickHouse Queries
**Priority**: P1
**Estimate**: 2-3 days
**Assignee**: Backend/Data engineer
**Depends on**: Task 2

- [ ] Create materialized views for the top slow queries.
- [ ] Add rollup tables if needed (hourly/daily pre-aggregations).
- [ ] Rewrite API queries to use materialized views/rollups.
- [ ] Validate data correctness against original queries.
- [ ] Measure improvement.

---

### Task 7: Reduce Data Volume Sent to Frontend
**Priority**: P1
**Estimate**: 1 day
**Assignee**: Backend + Frontend engineer
**Depends on**: Tasks 3, 4

- [ ] Implement server-side downsampling (bucket data to appropriate granularity based on time range).
- [ ] Trim GraphQL responses to only fields the frontend consumes.
- [ ] Verify charts render correctly with reduced data.

---

### Task 8: Optimize Frontend Rendering
**Priority**: P1
**Estimate**: 1-2 days
**Assignee**: Frontend engineer
**Depends on**: Task 4

- [ ] Lazy-load below-fold charts (IntersectionObserver).
- [ ] Implement client-side downsampling if data points exceed visual resolution.
- [ ] Switch to Canvas rendering if using SVG with large datasets.
- [ ] Code-split the dashboard page if not already done.
- [ ] Fix any unnecessary re-render issues.

---

### Task 9: Parallelize Data Fetching
**Priority**: P1
**Estimate**: 0.5 days
**Assignee**: Frontend/Backend engineer
**Depends on**: Task 3

- [ ] Ensure all chart queries fire in parallel (not serialized).
- [ ] Batch multiple chart queries into a single GraphQL request if possible.
- [ ] Verify no waterfall pattern in network tab.

---

### Task 10: Load Test and Validate
**Priority**: P0
**Estimate**: 1 day
**Assignee**: Backend engineer
**Depends on**: Tasks 5-9

- [ ] Run load test simulating peak concurrent users.
- [ ] Verify < 2s target at p95 under load.
- [ ] Verify data correctness across all charts.
- [ ] Document before/after performance numbers.

---

### Task 11: Add Performance Monitoring and Alerts
**Priority**: P2
**Estimate**: 0.5 days
**Assignee**: Fullstack engineer
**Depends on**: Task 10

- [ ] Set up Real User Monitoring (RUM) for the overview page.
- [ ] Add p50/p95/p99 load time tracking.
- [ ] Create alert for p95 > 2s regression.
- [ ] Add ClickHouse slow query alerts for dashboard queries.

---

## Summary

| Phase | Tasks | Calendar Days |
|-------|-------|---------------|
| Audit & Baseline | 1, 2, 3, 4 (partially parallel) | 2-3 days |
| Implementation | 5, 6, 7, 8, 9 (partially parallel) | 3-5 days |
| Validation | 10, 11 | 1-2 days |
| **Total** | | **6-10 days** |
