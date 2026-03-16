# Execution Backlog

> Task: Reduce analytics dashboard overview page load time from 8-10s to <2s (P95) through full-stack optimization
> Implementation approach: Three-layer systematic optimization — backend parallelization with errgroup + in-memory caching with singleflight, ClickHouse query fixes and limits, frontend per-chart queries with progressive loading and client-side downsampling
> Total subtasks: 10
> Execution waves: 4
> Decomposition status: finalized

## Execution Overview

The implementation is decomposed into 10 subtasks across 4 execution waves. Wave 1 establishes foundations: the ClickHouse bug fix, query limits, cache module, and frontend downsampling utility — all independent and fully parallelizable. Wave 2 builds core functionality: backend service parallelization with cache integration, the per-chart query hook, lazy loading, and Chart.tsx downsampling integration — up to 4 subtasks in parallel with declared dependencies on Wave 1. Wave 3 integrates the frontend: refactoring Overview.tsx to use per-chart queries and progressive loading. Wave 4 converges with API wiring. The structure maximizes parallel execution — up to 4 subtasks can run simultaneously at peak (Waves 1 and 2).

## Execution Waves

### Wave 1 — Foundation
Establishes all independent building blocks: bug fix, query optimization, cache module, and frontend utilities. All subtasks in this wave are fully independent with zero file overlap.

| Subtask | Title | Type | Scope | Can Parallel With |
|---------|-------|------|-------|-------------------|
| ST-1 | Fix retention cohort bug | foundation | small | ST-2, ST-3, ST-4 |
| ST-2 | Add ClickHouse query limits and coarser time buckets | foundation | small | ST-1, ST-3, ST-4 |
| ST-3 | Create ChartCache with per-chart TTL and singleflight | foundation | medium | ST-1, ST-2, ST-4 |
| ST-4 | Implement LTTB downsampling utility | foundation | small | ST-1, ST-2, ST-3 |

### Wave 2 — Core Implementation
Builds the core optimization layer on both backend and frontend. Backend parallelization depends on cache (ST-3) and query fixes (ST-1, ST-2). Frontend hook and lazy loading are independent of backend. Chart.tsx downsampling depends only on the downsample utility (ST-4).

| Subtask | Title | Type | Scope | Can Parallel With |
|---------|-------|------|-------|-------------------|
| ST-5 | Parallelize analytics service with errgroup and cache integration | implementation | large | ST-6, ST-7, ST-9 |
| ST-6 | Create useChartQuery hook | implementation | medium | ST-5, ST-7, ST-9 |
| ST-7 | Add Intersection Observer lazy loading to ChartGrid | implementation | small | ST-5, ST-6, ST-9 |
| ST-9 | Integrate downsampling into Chart.tsx | integration | small | ST-5, ST-6, ST-7 |

### Wave 3 — Frontend Integration
Refactors the Overview page to use per-chart queries with progressive loading. Depends on the useChartQuery hook (ST-6).

| Subtask | Title | Type | Scope | Can Parallel With |
|---------|-------|------|-------|-------------------|
| ST-8 | Refactor Overview.tsx for per-chart queries and progressive loading | integration | medium | — |

### Wave 4 — Convergence
API wiring. Depends on backend service (ST-5) and cache (ST-3).

| Subtask | Title | Type | Scope | Can Parallel With |
|---------|-------|------|-------|-------------------|
| ST-10 | Wire cache initialization in API server entry point | integration | small | — |

## Dependency Graph

```
ST-1 (fix retention bug) ──→ ST-5 (parallelize service)
ST-2 (query limits) ──→ ST-5 (parallelize service)
ST-3 (cache module) ──→ ST-5 (parallelize service)
ST-3 (cache module) ──→ ST-10 (API wiring)
ST-4 (downsample util) ──→ ST-9 (Chart.tsx integration)
ST-5 (parallelize service) ──→ ST-10 (API wiring)
ST-6 (useChartQuery hook) ──→ ST-8 (Overview.tsx refactor)
ST-7 (lazy loading) — no downstream dependencies
ST-8 (Overview.tsx refactor) — no downstream dependencies
ST-9 (Chart.tsx downsampling) — no downstream dependencies
ST-10 (API wiring) — no downstream dependencies
```

## Conflict Zones

| # | Zone | Subtasks Involved | Conflict Type | Severity | Resolution |
|---|------|-------------------|---------------|----------|------------|
| 1 | `internal/clickhouse/queries.go` | ST-1, ST-2 | file collision | low | Both modify `queries.go` but at different locations (ST-1: lines 67-85 retention bug; ST-2: adding LIMIT and GROUP BY changes). Changes are non-overlapping. Both are in Wave 1 and can proceed in parallel — merge conflicts are trivial to resolve since changes target different query functions. |
| 2 | `internal/clickhouse/queries_test.go` | ST-1, ST-2 | file collision | low | Both add tests to the same test file. Test additions are typically non-conflicting (additive, different test functions). Both are in Wave 1 and can proceed in parallel — merge is straightforward. |

---

## Subtasks

### ST-1: Fix Retention Cohort Bug

**ID:** ST-1
**Type:** foundation
**Wave:** 1
**Priority:** critical-path
**Estimated scope:** small

#### Purpose
The retention cohort query at `queries.go:67-85` has a bug: a missing WHERE clause causes full-table scans instead of filtered queries. This is both a correctness bug and a performance issue. Fixing it is independent of all other optimization work and provides immediate value.

#### Goal
The retention cohort query includes correct WHERE clause filtering by tenant_id and date range, eliminating unnecessary full-table scans.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| ClickHouse | `internal/clickhouse/queries.go` | modify | Fix WHERE clause at lines 67-85 to include tenant_id and date range filter |
| ClickHouse | `internal/clickhouse/queries_test.go` | modify | Add tests verifying correct filtered results for retention cohort |

#### Boundaries

**In scope:**
- Fix the retention cohort WHERE clause at `queries.go:67-85`
- Add test coverage for the fixed query
- Verify query returns correctly filtered data

**Out of scope:**
- Adding LIMIT clauses or coarser time buckets (ST-2)
- Any other query modifications beyond the retention cohort bug
- Cache integration (ST-3, ST-5)

#### Context

**Related design decisions:**
- No specific design decision — this is a bug fix identified during analysis

**Applicable constraints:**
- Skip-and-continue pattern must be preserved (failing chart doesn't block others)
- No ClickHouse schema changes allowed

**Key scenarios covered:**
- Mandatory edge case: "Retention cohort -> bug fix makes it work"

#### Dependencies

No dependencies — can start immediately.

#### Completion Criteria
- [ ] WHERE clause at `queries.go:67-85` includes tenant_id and date range filter
- [ ] Retention cohort query returns correctly filtered results (not full-table scan)
- [ ] Test coverage added for the fixed retention query
- [ ] Existing tests continue to pass

---

### ST-2: Add ClickHouse Query Limits and Coarser Time Buckets

**ID:** ST-2
**Type:** foundation
**Wave:** 1
**Priority:** high
**Estimated scope:** small

#### Purpose
Large tenants can return unbounded result sets from ClickHouse, causing slow queries and excessive memory usage. This subtask adds LIMIT clauses and coarser GROUP BY time buckets for date ranges exceeding 90 days, bounding query results.

#### Goal
All ClickHouse chart queries have LIMIT clauses applied and use coarser time buckets (GROUP BY) for date ranges > 90 days, preventing unbounded result sets.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| ClickHouse | `internal/clickhouse/queries.go` | modify | Add LIMIT clause to all chart query methods; add coarser GROUP BY for date ranges > 90 days |
| ClickHouse | `internal/clickhouse/queries_test.go` | modify | Add tests for LIMIT application and coarser bucket behavior |

#### Boundaries

**In scope:**
- Adding LIMIT clauses to chart query methods in `queries.go`
- Implementing coarser GROUP BY time buckets for date ranges > 90 days
- Adding tests for limit and bucketing behavior

**Out of scope:**
- Retention cohort bug fix (ST-1)
- Cache layer (ST-3)
- Service-level parallelization (ST-5)

#### Context

**Related design decisions:**
- No specific user-approved decision — this is a standard query optimization

**Applicable constraints:**
- No ClickHouse schema changes allowed
- Query modifications must be backward compatible (same return types)

**Key scenarios covered:**
- Mandatory edge case: "Large tenant -> query limits prevent unbounded results"

#### Dependencies

No dependencies — can start immediately.

#### Completion Criteria
- [ ] LIMIT clause applied to all chart query methods in `queries.go`
- [ ] Coarser GROUP BY used for date ranges > 90 days
- [ ] Tests verify LIMIT is applied correctly
- [ ] Tests verify coarser buckets are used for > 90 day ranges
- [ ] Existing tests continue to pass

---

### ST-3: Create ChartCache with Per-Chart TTL and Singleflight

**ID:** ST-3
**Type:** foundation
**Wave:** 1
**Priority:** critical-path
**Estimated scope:** medium

#### Purpose
The cache module is a new foundational component required by the analytics service (ST-5) and API wiring (ST-10). It provides in-memory caching with per-chart-type TTLs and singleflight deduplication to prevent thundering herd on cache misses.

#### Goal
A fully functional `ChartCache` module exists with Get/Set/Invalidate methods, per-chart-type TTL configuration, singleflight integration, and background cleanup — ready for integration into the analytics service.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| Cache | `internal/cache/chart_cache.go` | create | ChartCache struct with Get/Set/Invalidate, per-chart TTL config, singleflight integration, background cleanup goroutine |
| Cache | `internal/cache/chart_cache_test.go` | create | Tests for cache operations, TTL expiry, singleflight dedup, concurrent access safety |

#### Boundaries

**In scope:**
- `ChartCache` struct with `Get(chartID) (ChartData, bool)`, `Set(chartID, data)`, `Invalidate(chartID)`
- Per-chart-type TTL configuration (e.g., revenue: 5min, active_users: 1min, retention: 10min)
- singleflight integration to deduplicate concurrent requests for the same chart
- Background cleanup goroutine for expired entries
- Concurrent access safety (mutex-protected map)
- Comprehensive test coverage

**Out of scope:**
- Integration into the analytics service (ST-5)
- Initialization in `main.go` (ST-10)
- Redis or external cache — in-memory only per user decision

#### Context

**Related design decisions:**
- DD-2: In-memory cache (not Redis) — user confirmed single-instance deployment; sub-ms reads; cache lost on restart is acceptable
- DD-3: singleflight for cache stampede prevention — deduplicates concurrent requests; stdlib-adjacent package; zero config

**Applicable constraints:**
- Single-instance deployment (no distributed cache needed)
- Cache must handle concurrent access safely
- TTLs should be configurable per chart type

**Key scenarios covered:**
- Cache cold start -> singleflight prevents thundering herd
- Normal operation -> cache hit returns data without ClickHouse query
- TTL expiry -> stale data is evicted

#### Dependencies

No dependencies — can start immediately.

#### Completion Criteria
- [ ] `ChartCache` struct created with Get/Set/Invalidate methods
- [ ] Per-chart TTL configuration implemented (revenue: 5min, active_users: 1min, etc.)
- [ ] singleflight integration deduplicates concurrent requests for the same chart
- [ ] Background cleanup goroutine removes expired entries
- [ ] Concurrent access is safe (verified with race detector or mutex)
- [ ] Tests cover: cache hit, cache miss, TTL expiry, singleflight dedup, concurrent access
- [ ] All tests pass

---

### ST-4: Implement LTTB Downsampling Utility

**ID:** ST-4
**Type:** foundation
**Wave:** 1
**Priority:** high
**Estimated scope:** small

#### Purpose
The LTTB (Largest-Triangle-Three-Buckets) downsampling algorithm is needed by Chart.tsx (ST-9) to reduce large datasets to 500 points while preserving visual shape. This is a standalone utility with no external dependencies.

#### Goal
A working LTTB downsampling function exists that reduces time series data to a target number of points (default 500) while preserving the visual shape of the data.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| Frontend | `apps/dashboard/src/utils/downsample.ts` | create | LTTB downsampling algorithm implementation |
| Frontend | `apps/dashboard/src/utils/downsample.test.ts` | create | Tests for LTTB correctness, edge cases, performance |

#### Boundaries

**In scope:**
- LTTB algorithm implementation in TypeScript
- Function signature: accepts data array and target point count, returns downsampled array
- Tests for correctness (preserves shape), edge cases (fewer points than target, empty array), performance

**Out of scope:**
- Integration into Chart.tsx (ST-9)
- Any UI changes
- Server-side downsampling

#### Context

**Related design decisions:**
- DD-4: LTTB downsampling at 500 points — preserves visual shape better than naive sampling; 500 matches typical chart width; user confirmed threshold

**Applicable constraints:**
- Client-side only (not server-side)
- O(n) algorithm complexity
- Must preserve visual shape (not random or naive sampling)

**Key scenarios covered:**
- Large datasets (>500 points) are reduced to 500 while preserving peaks/valleys

#### Dependencies

No dependencies — can start immediately.

#### Completion Criteria
- [ ] LTTB downsampling function implemented in `downsample.ts`
- [ ] Function accepts data array and target point count (default 500)
- [ ] Correctly preserves visual shape of time series data
- [ ] Edge cases handled: empty array, fewer points than target, exactly target points
- [ ] Tests verify correctness and edge cases
- [ ] All tests pass

---

### ST-5: Parallelize Analytics Service with errgroup and Cache Integration

**ID:** ST-5
**Type:** implementation
**Wave:** 2
**Priority:** critical-path
**Estimated scope:** large

#### Purpose
This is the core backend optimization. The analytics service currently queries ClickHouse sequentially for each chart, taking 4+ seconds. This subtask refactors `LoadOverviewData` to use errgroup for parallel execution, integrates the ChartCache for caching, and wraps each chart loader with singleflight. It also adds a new `LoadChartData` method for individual chart loading.

#### Goal
The analytics service loads chart data in parallel using errgroup (bounded to 8 concurrent goroutines), with cache integration that eliminates redundant ClickHouse queries, reducing backend response time to <500ms for cached data.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| Analytics | `internal/analytics/service.go` | modify | Refactor `LoadOverviewData` to use errgroup with concurrency limit of 8; add `LoadChartData` method; integrate ChartCache (Get/Set); wrap each chart loader with singleflight |
| Analytics | `internal/analytics/service_test.go` | modify | Add tests for parallel execution, cache hit/miss paths, singleflight behavior, errgroup error handling (skip-and-continue) |

#### Boundaries

**In scope:**
- Refactoring `LoadOverviewData` to parallel execution with errgroup
- Adding `LoadChartData(ctx, chartID) (ChartData, error)` method
- Integrating ChartCache (calling Get before query, Set after query)
- Wrapping chart loaders with singleflight
- errgroup concurrency limit of 8
- Skip-and-continue error handling (one chart fails, others proceed)
- Test coverage for parallel execution, caching, singleflight, error handling

**Out of scope:**
- Creating the ChartCache module itself (ST-3)
- Modifying ClickHouse queries (ST-1, ST-2)
- API server initialization / wiring (ST-10)
- Frontend changes

#### Context

**Related design decisions:**
- DD-1: errgroup with concurrency limit of 8 — standard Go pattern; bounded goroutines; 8 matches max chart count; error propagation for skip-and-continue
- DD-2: In-memory cache — service receives ChartCache via constructor injection
- DD-3: singleflight — wraps each chart loader to deduplicate concurrent requests

**Applicable constraints:**
- Each goroutine must use its own ClickHouse connection (no shared mutable state)
- Skip-and-continue pattern preserved (failing chart doesn't block others)
- Connection pool must handle 8 concurrent connections (pool size is 20)
- Return type of `LoadOverviewData` changes (interface change)

**Key scenarios covered:**
- Primary scenario step 4: "Backend: cache check -> singleflight -> parallel ClickHouse"
- Mandatory edge case: "ClickHouse down -> skip-and-continue preserved"
- Risk: Connection pool exhaustion mitigated by concurrency limit of 8

#### Dependencies

| Dependency | Type | From | Unblock Condition |
|------------|------|------|-------------------|
| ChartCache module | blocking | ST-3 | `ChartCache` with Get/Set/Invalidate is implemented and tests pass |
| Fixed ClickHouse queries | soft | ST-1 | Retention bug is fixed (service can start with unfixed queries but results will be incorrect for retention) |
| Query limits | soft | ST-2 | LIMIT and coarser buckets are applied (service can start without them but large tenant queries may be slow) |

#### Completion Criteria
- [ ] `LoadOverviewData` uses errgroup with concurrency limit of 8
- [ ] `LoadChartData(ctx, chartID)` method exists for individual chart loading
- [ ] Cache is checked before ClickHouse query (Get) and stored after (Set)
- [ ] singleflight wraps chart loaders to deduplicate concurrent requests
- [ ] Skip-and-continue: one chart failure doesn't block other charts
- [ ] No shared mutable state between goroutines
- [ ] Tests pass for: parallel execution, cache hit, cache miss, singleflight dedup, error handling
- [ ] Race detector passes

---

### ST-6: Create useChartQuery Hook

**ID:** ST-6
**Type:** implementation
**Wave:** 2
**Priority:** critical-path
**Estimated scope:** medium

#### Purpose
The `useChartQuery` hook is the frontend foundation for per-chart data fetching. It fires an independent GraphQL query for a specific chart and returns `{data, loading, error}` state. This hook is used by the refactored Overview.tsx (ST-8) to enable progressive loading.

#### Goal
A custom React hook exists that fetches data for a single chart via GraphQL, returning loading/error/data states, enabling per-chart independent data fetching.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| Frontend | `apps/dashboard/src/hooks/useChartQuery.ts` | create | Custom hook: fires per-chart GraphQL query using Apollo Client, returns {data, loading, error} |

#### Boundaries

**In scope:**
- `useChartQuery(chartId)` hook implementation
- Per-chart GraphQL query definition
- Loading, error, and data state management
- Apollo Client integration (using existing setup)
- Quantizing time range to fix Apollo cache-busting issue

**Out of scope:**
- Modifying Overview.tsx (ST-8)
- Downsampling logic (ST-4, ST-9)
- Lazy loading (ST-7)
- Backend GraphQL resolver changes (existing schema supports per-chart queries)

#### Context

**Related design decisions:**
- DD-5: Per-chart GraphQL queries (not subscriptions) — simpler than subscriptions; progressive loading via independent queries; no WebSocket infrastructure needed

**Applicable constraints:**
- Must use existing Apollo Client setup
- GraphQL schema is additive (no breaking changes)
- Must fix the Apollo cache-busting issue by quantizing time range (`Overview.tsx:26-27` reference from agreed-task-model)

**Key scenarios covered:**
- Primary scenario step 2: "Per-chart queries fire (not monolithic)"
- Acceptance criterion: "Apollo cache hits"

#### Dependencies

No dependencies — can start immediately (uses existing GraphQL schema and Apollo Client setup).

#### Completion Criteria
- [ ] `useChartQuery(chartId)` hook implemented in `useChartQuery.ts`
- [ ] Returns `{data, loading, error}` for per-chart GraphQL query
- [ ] Uses existing Apollo Client
- [ ] Time range is quantized to prevent cache busting
- [ ] Hook works independently for each chart
- [ ] Tests verify correct data fetching and state management

---

### ST-7: Add Intersection Observer Lazy Loading to ChartGrid

**ID:** ST-7
**Type:** implementation
**Wave:** 2
**Priority:** normal
**Estimated scope:** small

#### Purpose
Charts below the fold on the overview page currently load immediately even though they're not visible. This subtask adds Intersection Observer-based lazy loading so that below-fold charts only fetch their data when scrolled into view, reducing initial ClickHouse load and improving perceived performance.

#### Goal
Below-fold charts in the ChartGrid only fire their data queries when they become visible in the viewport via Intersection Observer.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| Frontend | `apps/dashboard/src/components/ChartGrid.tsx` | modify | Add Intersection Observer to detect chart visibility; defer rendering/data-fetching of below-fold charts until visible |

#### Boundaries

**In scope:**
- Adding Intersection Observer to ChartGrid.tsx
- Deferring chart rendering/query firing until chart is visible
- Showing placeholder/skeleton for not-yet-visible charts

**Out of scope:**
- Per-chart query logic (ST-6)
- Overview.tsx refactoring (ST-8)
- Downsampling (ST-4, ST-9)
- Any backend changes

#### Context

**Related design decisions:**
- DD-6: Intersection Observer for lazy loading — native browser API; no library needed; works with chart grid layout

**Applicable constraints:**
- Native Intersection Observer API only (no external library)
- Must work with the chart grid layout
- Skeleton state for charts not yet loaded

**Key scenarios covered:**
- Primary scenario step 3: "Above-fold charts load first"
- Acceptance criterion: "Below-fold charts only load when scrolled into view"

#### Dependencies

No dependencies — can start immediately. Intersection Observer logic is independent of how charts fetch data.

#### Completion Criteria
- [ ] Intersection Observer integrated into ChartGrid.tsx
- [ ] Below-fold charts do not render/fetch until visible
- [ ] Skeleton/placeholder shown for not-yet-visible charts
- [ ] Above-fold charts load immediately
- [ ] Works with native Intersection Observer API (no external library)

---

### ST-8: Refactor Overview.tsx for Per-Chart Queries and Progressive Loading

**ID:** ST-8
**Type:** integration
**Wave:** 3
**Priority:** critical-path
**Estimated scope:** medium

#### Purpose
The Overview page currently fires a single monolithic GraphQL query for all charts and renders them only after all data arrives. This subtask refactors it to use the `useChartQuery` hook (ST-6) for each chart, enabling progressive loading where each chart renders independently as its data arrives with skeleton states for loading charts.

#### Goal
The Overview page renders each chart independently as its data arrives, using per-chart queries through the `useChartQuery` hook, with skeleton loading states.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| Frontend | `apps/dashboard/src/components/Overview.tsx` | modify | Remove monolithic GraphQL query; iterate chart identifiers; render each chart with `useChartQuery`; add skeleton loading states for progressive rendering |

#### Boundaries

**In scope:**
- Removing the monolithic overview GraphQL query
- Using `useChartQuery` hook for each chart
- Progressive rendering: each chart renders as its data arrives
- Skeleton loading states for charts still loading
- Removing the `data` prop dependency (Overview manages own data fetching)

**Out of scope:**
- Creating the `useChartQuery` hook (ST-6)
- Downsampling integration (ST-9)
- Lazy loading logic (ST-7 — handled by ChartGrid)
- Backend changes

#### Context

**Related design decisions:**
- DD-5: Per-chart GraphQL queries — split monolithic query into per-chart queries for progressive loading

**Applicable constraints:**
- Existing GraphQL queries must continue to work (additive schema changes only)
- Overview.tsx no longer receives `data` prop (interface change)
- Progressive loading is a visual change (intentional)

**Key scenarios covered:**
- Primary scenario steps 1-3: "User opens overview page -> per-chart queries fire -> above-fold charts load first"
- Acceptance criterion: "Progressive loading works"
- Acceptance criterion: "All 8 charts correct data"

#### Dependencies

| Dependency | Type | From | Unblock Condition |
|------------|------|------|-------------------|
| useChartQuery hook | blocking | ST-6 | `useChartQuery` hook is implemented and returns {data, loading, error} |

#### Completion Criteria
- [ ] Monolithic overview query removed from Overview.tsx
- [ ] Each chart uses `useChartQuery(chartId)` for independent data fetching
- [ ] Charts render progressively as data arrives
- [ ] Skeleton loading states shown for charts still loading
- [ ] Overview.tsx no longer depends on `data` prop
- [ ] All charts display correct data

---

### ST-9: Integrate Downsampling into Chart.tsx

**ID:** ST-9
**Type:** integration
**Wave:** 2
**Priority:** high
**Estimated scope:** small

#### Purpose
Chart.tsx currently renders all raw data points directly, which causes rendering lag for large datasets. This subtask integrates the LTTB downsampling utility (ST-4) so that datasets with more than 500 points are automatically downsampled before being passed to Recharts.

#### Goal
Chart.tsx automatically downsamples datasets exceeding 500 points using LTTB before rendering, eliminating rendering lag for large datasets.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| Frontend | `apps/dashboard/src/components/Chart.tsx` | modify | Import downsample utility; apply LTTB downsampling when data exceeds 500 points (or `maxPoints` prop); pass downsampled data to Recharts |

#### Boundaries

**In scope:**
- Importing the LTTB downsample function from `utils/downsample.ts`
- Adding optional `maxPoints` prop (default 500)
- Applying downsampling before passing data to Recharts
- Preserving existing Chart.tsx rendering behavior for small datasets

**Out of scope:**
- Implementing the LTTB algorithm (ST-4)
- Overview.tsx refactoring (ST-8)
- Server-side downsampling

#### Context

**Related design decisions:**
- DD-4: LTTB downsampling at 500 points — user confirmed threshold; preserves visual shape; client-side for flexibility

**Applicable constraints:**
- Default threshold is 500 points (user confirmed)
- Optional `maxPoints` prop for override
- Must preserve visual shape (LTTB guarantees this)
- Downsampled data must be compatible with Recharts

**Key scenarios covered:**
- Primary scenario step 5: "Data downsampled -> charts render"
- Acceptance criterion: "Chart data is numerically accurate (no rounding errors from downsampling beyond acceptable tolerance)"

#### Dependencies

| Dependency | Type | From | Unblock Condition |
|------------|------|------|-------------------|
| Downsampling utility | blocking | ST-4 | `downsample.ts` with LTTB function exists and tests pass |

#### Completion Criteria
- [ ] Chart.tsx imports downsample function from `utils/downsample.ts`
- [ ] Optional `maxPoints` prop added (default 500)
- [ ] Data > `maxPoints` is automatically downsampled via LTTB
- [ ] Data <= `maxPoints` is passed through unchanged
- [ ] Downsampled data renders correctly in Recharts
- [ ] Visual shape is preserved after downsampling

---

### ST-10: Wire Cache Initialization in API Server Entry Point

**ID:** ST-10
**Type:** integration
**Wave:** 4
**Priority:** high
**Estimated scope:** small

#### Purpose
The API server's `main.go` needs to initialize the ChartCache with TTL configuration and pass it to the AnalyticsService constructor. This is the final backend wiring step that connects the cache module (ST-3) to the parallelized service (ST-5).

#### Goal
The API server initializes ChartCache on startup with per-chart TTL configuration and injects it into the AnalyticsService, completing the backend integration.

#### Change Area

| Module | File | Change Type | Description |
|--------|------|-------------|-------------|
| API | `cmd/analytics-api/main.go` | modify | Initialize ChartCache with TTL config; pass to AnalyticsService constructor |

#### Boundaries

**In scope:**
- Creating ChartCache instance in `main.go` with per-chart TTL config
- Passing ChartCache to AnalyticsService constructor
- Verifying service starts correctly with cache

**Out of scope:**
- ChartCache implementation (ST-3)
- Service parallelization logic (ST-5)
- Frontend changes

#### Context

**Related design decisions:**
- DD-2: In-memory cache — initialized at startup; lives for the process lifetime
- DD-3: singleflight — integrated within ChartCache (no separate wiring needed)

**Applicable constraints:**
- Per-chart TTL configuration: revenue: 5min, active_users: 1min, retention: 10min (from design)
- Cache must be initialized before service starts handling requests

**Key scenarios covered:**
- System startup: cache is ready before first request
- Integration: cache + service + API are wired together

#### Dependencies

| Dependency | Type | From | Unblock Condition |
|------------|------|------|-------------------|
| ChartCache module | blocking | ST-3 | `ChartCache` with constructor accepting TTL config exists |
| Parallelized service | blocking | ST-5 | `AnalyticsService` constructor accepts ChartCache parameter |

#### Completion Criteria
- [ ] ChartCache initialized in `main.go` with per-chart TTL configuration
- [ ] ChartCache passed to AnalyticsService constructor
- [ ] Service starts correctly with cache wired in
- [ ] Verified no startup errors

---

## Critique Review

**Verdict: DECOMPOSITION_APPROVED**

| Criterion | Score | Reasoning |
|-----------|-------|-----------|
| Task clarity | PASS | Each subtask has clear purpose, goal, change area, boundaries, and completion criteria. An implementor can understand what to do without ambiguity. |
| Boundary quality | PASS | Explicit in-scope/out-of-scope sections with cross-references between subtasks. No chaotic overlaps. |
| Dependency correctness | PASS | Dependencies are correctly typed (blocking/soft), unblock conditions are specific, and the graph is acyclic and consistent. |
| Parallelizability | PASS | Waves are well-defined with genuinely independent subtasks. After refinement, ST-9 moved to Wave 2 to improve parallelism (4 subtasks in Wave 2 vs. original 3). |
| Conflict risk | PASS | All file overlaps identified: `queries.go` and `queries_test.go` between ST-1 and ST-2, both at different locations. Resolutions are appropriate. |
| Context completeness | PASS | Each subtask includes relevant design decisions, constraints, and scenarios from the agreed model. Self-contained for independent execution. |
| Scope discipline | PASS | All subtasks map directly to the implementation design and change map. No extra features or components added beyond what was agreed. |

**Issues addressed during critique:**
1. ST-9 was originally in Wave 3 but only depends on ST-4 (Wave 1) — moved to Wave 2 to improve parallelism.
2. `queries_test.go` file overlap between ST-1 and ST-2 was not originally in the conflict zones — added with low severity (additive test changes).

**No boundary overlaps, missing dependencies, unnecessary dependencies, scope additions, context gaps, or parallel execution risks found.**

## Coverage Review

**Verdict: COVERAGE_OK**
**Confidence: high**

All traceability links are clear and complete. Every requirement from the agreed task model, every change from the implementation design, every file from the change map, and every design decision maps to at least one subtask with specific completion criteria.

**Coverage summary:**
- Agreed task model: 8 acceptance criteria — all covered
- Implementation design: 5 modules, 4 new entities, 6 modified entities — all covered
- Change map: 8 files to modify, 5 files to create — all assigned
- Design decisions: 6 decisions — all reflected in relevant subtasks
- Deferred decisions: 3 deferred items — all correctly excluded

**Done-state assessment:** If all subtasks complete, the original task is done. The primary scenario works end-to-end (user opens page -> per-chart queries -> cache/singleflight/parallel ClickHouse -> progressive rendering -> downsampling -> lazy loading). All mandatory edge cases are handled. All acceptance criteria are met.

**No coverage gaps, no over-coverage, no missing structural subtasks detected.**

## User Review Log
User review skipped (test run). Decomposition finalized based on critic and coverage reviewer feedback.
