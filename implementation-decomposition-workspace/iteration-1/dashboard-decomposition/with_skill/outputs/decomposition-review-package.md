# Decomposition Review Package

> Task: Reduce analytics dashboard overview page load time from 8-10s to <2s (P95) through full-stack optimization
> Total subtasks: 10
> Execution waves: 4
> Estimated parallel efficiency: 4 subtasks can run simultaneously at peak (Waves 1 and 2)

## Decomposition Summary

The full-stack dashboard optimization is decomposed into 10 subtasks across 4 waves. The decomposition follows the natural layer boundaries: ClickHouse query fixes, cache module, and frontend utilities form an independent foundation (Wave 1, 4 parallel subtasks). Backend parallelization, frontend hooks, lazy loading, and Chart.tsx downsampling build on that foundation in Wave 2 (4 parallel subtasks). Overview.tsx integration follows in Wave 3 once the per-chart hook is ready. API wiring completes the backend in Wave 4. The key design choice is maximizing parallelism by isolating foundation work into independent, zero-overlap subtasks, while keeping integration subtasks sequential where they must consume outputs of earlier work.

## Subtask Overview

| # | Subtask | Type | Wave | Scope | Key Dependencies |
|---|---------|------|------|-------|-----------------|
| ST-1 | Fix retention cohort bug | foundation | 1 | small | none |
| ST-2 | Add ClickHouse query limits and coarser time buckets | foundation | 1 | small | none |
| ST-3 | Create ChartCache with per-chart TTL and singleflight | foundation | 1 | medium | none |
| ST-4 | Implement LTTB downsampling utility | foundation | 1 | small | none |
| ST-5 | Parallelize analytics service with errgroup and cache integration | implementation | 2 | large | ST-1 (soft), ST-2 (soft), ST-3 (blocking) |
| ST-6 | Create useChartQuery hook | implementation | 2 | medium | none |
| ST-7 | Add Intersection Observer lazy loading to ChartGrid | implementation | 2 | small | none |
| ST-9 | Integrate downsampling into Chart.tsx | integration | 2 | small | ST-4 (blocking) |
| ST-8 | Refactor Overview.tsx for per-chart queries and progressive loading | integration | 3 | medium | ST-6 (blocking) |
| ST-10 | Wire cache initialization in API server entry point | integration | 4 | small | ST-3 (blocking), ST-5 (blocking) |

## Execution Waves

### Wave 1: Foundation
**Subtasks:** ST-1, ST-2, ST-3, ST-4
**Parallel:** all 4 subtasks can run simultaneously — zero file overlap between them
**Goal:** Establish all independent building blocks: ClickHouse bug fix, query limits, cache module, LTTB downsampling utility

### Wave 2: Core Implementation
**Subtasks:** ST-5, ST-6, ST-7, ST-9
**Parallel:** all 4 subtasks can run simultaneously. ST-5 waits for ST-3 (cache). ST-9 waits for ST-4 (downsample). ST-6 and ST-7 have no Wave 1 dependencies.
**Goal:** Build the core optimization: backend parallelization with cache, per-chart query hook, lazy loading, Chart.tsx downsampling integration

### Wave 3: Frontend Integration
**Subtasks:** ST-8
**Sequential:** ST-8 depends on ST-6 (useChartQuery hook)
**Goal:** Refactor the Overview page to use per-chart queries with progressive loading

### Wave 4: Convergence
**Subtasks:** ST-10
**Sequential:** ST-10 depends on ST-3 (cache) and ST-5 (parallelized service)
**Goal:** Wire cache initialization in the API server entry point, completing backend integration

## Dependency Highlights

- **ST-3 (cache) is the critical foundation for the backend:** ST-5 (service parallelization) and ST-10 (API wiring) both depend on it. Starting ST-3 early is essential for the backend critical path.
- **ST-6 (useChartQuery) gates the frontend integration:** ST-8 (Overview.tsx refactor) cannot begin until ST-6 delivers the per-chart query hook. Frontend critical path runs through ST-6 -> ST-8.
- **Backend and frontend critical paths are independent:** The backend path (ST-3 -> ST-5 -> ST-10) and frontend path (ST-6 -> ST-8) can run in parallel with no cross-dependencies.
- **ST-1 and ST-2 are soft dependencies for ST-5:** The service parallelization can begin before the bug fix and query limits are complete, but correct results require them.

## Conflict Zones

- **`queries.go` and `queries_test.go` (ST-1 + ST-2):** Both modify the same files in Wave 1, but at different locations (ST-1 targets lines 67-85 for the retention bug; ST-2 adds LIMIT and GROUP BY changes to other query methods). Low severity — merge conflicts are trivial since changes are non-overlapping.

No other significant conflict zones detected.

## Coverage Assessment
- **Coverage:** COVERAGE_OK
- **Confidence:** high
- **Key finding:** All 8 acceptance criteria from the agreed task model map to specific subtask completion criteria. All 13 files from the change map (8 modified, 5 new) are assigned to subtasks. All 6 design decisions are reflected in the relevant subtasks. No gaps, no over-coverage.

## Review Points

### Point 1: Wave Ordering of ST-9 (Downsampling Integration)
**Context:** ST-9 (integrating downsampling into Chart.tsx) was moved from Wave 3 to Wave 2 during critique review because its only dependency is ST-4 (downsample utility, Wave 1), not ST-8 (Overview refactor).
**Current approach:** ST-9 runs in Wave 2 alongside ST-5, ST-6, and ST-7.
**Question:** Is this ordering correct, or should Chart.tsx downsampling integration wait until after Overview.tsx is refactored?

### Point 2: Soft Dependencies on ST-1 and ST-2
**Context:** ST-5 (service parallelization) has soft dependencies on ST-1 (bug fix) and ST-2 (query limits). This means ST-5 can start before those are done.
**Current approach:** ST-5 in Wave 2 can begin once ST-3 (cache, blocking dependency) is complete, even if ST-1/ST-2 are still in progress from Wave 1.
**Question:** Is the soft dependency approach acceptable, or should ST-5 wait for ST-1 and ST-2 to fully complete?

## Scope Confirmation

**All agreed requirements covered:**
- P95 load <2s -> ST-5 + ST-8 + ST-7 + ST-9 + ST-10
- All 8 charts correct data -> ST-1, ST-5, ST-8
- Retention cohort works -> ST-1
- GraphQL contract unchanged -> ST-6, ST-8
- Progressive loading works -> ST-8
- Apollo cache hits -> ST-6
- ClickHouse load reduced -> ST-3, ST-5
- Race detector passes -> ST-5

**Question:** Does this decomposition cover everything you need? Any subtasks that should be split, merged, reordered, or removed?
