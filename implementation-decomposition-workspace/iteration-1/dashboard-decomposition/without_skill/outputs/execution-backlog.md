# Execution Backlog: Dashboard Performance Optimization

> Goal: Reduce analytics dashboard overview page load from 8-10s to <2s (P95)
> Total subtasks: 12
> Waves: 4 (parallel execution within each wave)
> Estimated files: 8 modified, 5 new

---

## Wave 1 — Independent Foundations (no cross-task dependencies)

All tasks in this wave can execute in parallel. They have zero dependencies on each other.

---

### Task 1.1: Fix Retention Cohort Bug

**What:** Fix the broken WHERE clause in the retention cohort query at `queries.go` lines 67-85. The current query is missing a `tenant_id` and date range filter, causing a full-table scan instead of a filtered query.

**Files involved:**
- `internal/clickhouse/queries.go` (modify lines 67-85)

**Dependencies:** None

**Implementation details:**
- Add `WHERE tenant_id = ? AND date >= ? AND date <= ?` to the retention cohort query
- Ensure parameter binding matches the new WHERE clause (fix param count)
- Verify the fix eliminates the full-table scan via EXPLAIN if possible

**Done when:**
- The retention cohort query returns correctly filtered results for a given tenant and date range
- No full-table scan occurs (bounded query execution time)
- Existing retention data remains accurate

**Risk notes:** This changes query behavior for a production query. Test with production-like data volumes to ensure no regressions.

---

### Task 1.2: Add Query Limits and Coarser Time Buckets

**What:** Add LIMIT clauses to all chart queries and switch to coarser GROUP BY time buckets for date ranges exceeding 90 days. This prevents unbounded result sets from large tenants.

**Files involved:**
- `internal/clickhouse/queries.go` (modify — all chart query functions)

**Dependencies:** None (can be done alongside Task 1.1 since they modify different sections of the same file, but coordinate if same developer)

**Implementation details:**
- Add `LIMIT` clause to every chart query function (determine sensible per-chart limits)
- For date ranges > 90 days: change GROUP BY from daily to weekly buckets
- For date ranges > 365 days: consider monthly buckets
- Preserve existing sort order so LIMIT cuts off the right rows

**Done when:**
- All chart queries have explicit LIMIT clauses
- Queries spanning >90 days use coarser time buckets
- Large tenant queries complete within ClickHouse timeout
- Chart data remains correct for typical date ranges

---

### Task 1.3: Create ChartCache with Per-Chart TTL and Singleflight

**What:** Create a new in-memory cache service for chart data. Supports per-chart-type TTL configuration, thread-safe concurrent access, singleflight deduplication, and background cleanup of expired entries.

**Files involved:**
- `internal/cache/chart_cache.go` (create)
- `internal/cache/chart_cache_test.go` (create)

**Dependencies:** None

**Implementation details:**
- Define `ChartCache` struct with `sync.RWMutex`-protected map
- Methods: `Get(chartID string) (ChartData, bool)`, `Set(chartID string, data ChartData)`, `Invalidate(chartID string)`
- Integrate `golang.org/x/sync/singleflight` — expose `GetOrLoad(chartID string, loader func() (ChartData, error)) (ChartData, error)` method
- Per-chart-type TTL configuration:
  - `revenue`: 5 min
  - `active_users`: 1 min
  - `retention`: 10 min
  - Default: 2 min
- Background goroutine for expired entry cleanup (run every 30s)
- Constructor: `NewChartCache(config chartCacheConfig) *ChartCache`

**Tests needed:**
- Get returns stored data within TTL
- Get returns miss after TTL expiry
- Set overwrites existing entries
- Concurrent read/write safety (run with `-race`)
- Singleflight deduplicates concurrent requests for same chartID
- Invalidate removes entry immediately
- Background cleanup removes expired entries

**Done when:**
- All tests pass including race detector (`go test -race`)
- Cache correctly stores, retrieves, and expires chart data
- Singleflight prevents duplicate loads for same chart

---

### Task 1.4: Implement LTTB Downsampling Utility

**What:** Create a TypeScript utility implementing the Largest-Triangle-Three-Buckets (LTTB) downsampling algorithm. This reduces large datasets to a target number of points while preserving visual shape.

**Files involved:**
- `apps/dashboard/src/utils/downsample.ts` (create)
- `apps/dashboard/src/utils/downsample.test.ts` (create)

**Dependencies:** None

**Implementation details:**
- Export function: `downsample(data: DataPoint[], targetPoints: number): DataPoint[]`
- `DataPoint` type: `{ x: number; y: number }` (or adapt to existing chart data types)
- If `data.length <= targetPoints`, return data unchanged
- Implement standard LTTB algorithm:
  1. Always keep first and last points
  2. Divide remaining data into `targetPoints - 2` buckets
  3. For each bucket, select point forming largest triangle with selected point from previous bucket and average point of next bucket
- Algorithm is O(n) time complexity

**Tests needed:**
- Returns input unchanged when length <= target
- Reduces to exactly `targetPoints` when input is larger
- Always includes first and last points
- Preserves peaks and valleys (shape preservation)
- Handles edge cases: empty array, single point, two points
- Handles negative values correctly

**Done when:**
- All tests pass
- Function correctly reduces datasets to target point count
- Visual shape is preserved (peaks/valleys retained)

---

### Task 1.5: Add Lazy Loading to ChartGrid

**What:** Add Intersection Observer-based lazy loading to `ChartGrid.tsx` so below-fold charts only fetch data when scrolled into view.

**Files involved:**
- `apps/dashboard/src/components/ChartGrid.tsx` (modify)

**Dependencies:** None (the lazy loading mechanism is independent of the data-fetching refactor; it wraps chart containers to control visibility)

**Implementation details:**
- Use native `IntersectionObserver` API (no library)
- Each chart wrapper gets a ref observed by the IntersectionObserver
- When a chart enters the viewport (or is within a threshold, e.g., 200px), set its `isVisible` state to `true`
- Only render the chart component (or trigger its data fetch) when `isVisible` is true
- Show a placeholder/skeleton with the correct dimensions when `isVisible` is false
- Clean up observer on unmount

**Done when:**
- Charts below the fold do not render or fetch data until scrolled near
- Charts above the fold render immediately
- Observer is properly cleaned up on component unmount
- No layout shift when charts load (placeholder has correct dimensions)

---

## Wave 2 — Backend Integration (depends on Wave 1 cache and query fixes)

These tasks integrate the Wave 1 backend foundations.

---

### Task 2.1: Parallelize Analytics Service with errgroup and Cache Integration

**What:** Refactor `AnalyticsService.LoadOverviewData` to load charts in parallel using errgroup, integrated with the ChartCache from Task 1.3. Add a new `LoadChartData` method for individual chart loading.

**Files involved:**
- `internal/analytics/service.go` (modify)
- `internal/analytics/service_test.go` (modify)

**Dependencies:**
- Task 1.1 (retention bug fix — queries must be correct before parallelizing)
- Task 1.2 (query limits — queries must be bounded before parallelizing)
- Task 1.3 (ChartCache must exist)

**Implementation details:**
- Add `ChartCache` as a dependency on the `AnalyticsService` struct
- New method: `LoadChartData(ctx context.Context, chartID string) (ChartData, error)`
  - Calls `ChartCache.GetOrLoad(chartID, loader)` where loader executes the ClickHouse query
- Refactor `LoadOverviewData` to use `errgroup.Group` with `.SetLimit(8)`
  - Launch one goroutine per chart via `g.Go(func() error { ... })`
  - Each goroutine calls `LoadChartData`
  - Collect results; skip-and-continue on individual chart errors (log error, don't propagate)
- Return type changes from `OverviewData` struct to per-chart results or `map[string]ChartData`
- Each errgroup goroutine gets its own ClickHouse connection (no shared mutable state)

**Tests needed:**
- Parallel execution: all charts load concurrently (verify via timing or mock)
- Cache hit: returns cached data without ClickHouse call
- Cache miss: triggers ClickHouse query, stores result in cache
- Singleflight: concurrent calls for same chart result in single query
- Error handling: one chart fails, others succeed (skip-and-continue)
- Context cancellation propagates correctly
- Run all tests with `-race` flag

**Done when:**
- All tests pass including race detector
- `LoadChartData` correctly integrates cache + singleflight + ClickHouse
- `LoadOverviewData` runs charts in parallel with errgroup
- Backend response time < 500ms for cached data
- Skip-and-continue pattern preserved

---

### Task 2.2: Wire Cache into API Server

**What:** Initialize ChartCache in the API server entrypoint and pass it to the AnalyticsService constructor.

**Files involved:**
- `cmd/analytics-api/main.go` (modify)

**Dependencies:**
- Task 1.3 (ChartCache must exist)
- Task 2.1 (service must accept ChartCache)

**Implementation details:**
- Import `internal/cache` package
- Create `chartCacheConfig` with per-chart TTLs
- Call `cache.NewChartCache(config)` during initialization
- Pass the cache instance to the `AnalyticsService` constructor
- Ensure cache background cleanup goroutine starts with the server

**Done when:**
- API server starts successfully with cache initialized
- AnalyticsService receives and uses the cache
- No initialization errors or panics

---

### Task 2.3: Add ClickHouse Query Tests

**What:** Add tests for the retention cohort fix, query limits, and coarser bucket behavior.

**Files involved:**
- `internal/clickhouse/queries_test.go` (modify or create)

**Dependencies:**
- Task 1.1 (retention bug fix)
- Task 1.2 (query limits)

**Implementation details:**
- Test that retention cohort query includes correct WHERE clause with tenant_id and date filters
- Test that all chart queries include LIMIT clause
- Test that queries for >90 day ranges use coarser time buckets
- Test boundary conditions (exactly 90 days, 91 days)

**Done when:**
- All tests pass
- Coverage exists for the fixed retention query, limits, and bucket coarsening

---

## Wave 3 — Frontend Data Layer (depends on Wave 1 utilities)

These tasks build the frontend data-fetching layer. They can run in parallel with Wave 2.

---

### Task 3.1: Create useChartQuery Hook

**What:** Create a custom React hook that fires a per-chart GraphQL query, returning loading/error/data state for an individual chart.

**Files involved:**
- `apps/dashboard/src/hooks/useChartQuery.ts` (create)

**Dependencies:**
- None from other tasks (uses existing GraphQL schema and Apollo Client)
- Follow existing hook patterns in `src/hooks/`

**Implementation details:**
- Export `useChartQuery(chartId: string, options?: { skip?: boolean })`
- Returns `{ data: ChartData | null, loading: boolean, error: Error | null }`
- Uses Apollo Client's `useQuery` under the hood with a per-chart GraphQL query
- Respects `skip` option for lazy loading integration (chart not yet visible)
- Quantize time range parameters to prevent Apollo cache busting (round to nearest 5-minute or similar boundary, per `Overview.tsx:26-27` fix from the agreed task model)

**Done when:**
- Hook correctly fetches chart data for a given chartId
- Returns proper loading/error/data states
- `skip` option prevents query from firing
- Time range quantization prevents excessive cache misses

---

### Task 3.2: Refactor Overview.tsx for Per-Chart Queries

**What:** Remove the monolithic GraphQL query from Overview.tsx. Instead, render a ChartGrid where each chart manages its own data fetching via `useChartQuery`.

**Files involved:**
- `apps/dashboard/src/components/Overview.tsx` (modify)

**Dependencies:**
- Task 3.1 (useChartQuery hook must exist)
- Task 1.5 (ChartGrid lazy loading should be ready, or stub with always-visible)

**Implementation details:**
- Remove the single large GraphQL query and its associated data/loading/error state
- Remove the `data` prop — Overview now manages its own data
- Render `ChartGrid` with chart identifiers (list of chart IDs for the overview page)
- Each chart in the grid uses `useChartQuery` internally (or receives the hook output)
- Integrate with ChartGrid's lazy loading: pass `skip` to `useChartQuery` when chart is not yet visible
- Show skeleton/loading state per chart (not a single page-level spinner)

**Done when:**
- Overview page no longer fires a monolithic query
- Each chart loads independently
- Skeleton states display per chart while loading
- Error in one chart does not block others
- Page remains functional with all charts rendering correctly

---

### Task 3.3: Add Client-Side Downsampling to Chart Component

**What:** Integrate the LTTB downsampling utility into Chart.tsx so large datasets are reduced to 500 points before rendering.

**Files involved:**
- `apps/dashboard/src/components/Chart.tsx` (modify)

**Dependencies:**
- Task 1.4 (downsample utility must exist)

**Implementation details:**
- Import `downsample` from `utils/downsample`
- Add optional `maxPoints` prop (default: 500)
- Before passing data to Recharts, run `downsample(data, maxPoints)` if `data.length > maxPoints`
- Use `useMemo` to memoize the downsampled result (re-compute only when data or maxPoints change)

**Done when:**
- Charts with >500 data points render with downsampled data
- Charts with <=500 points render with original data unchanged
- No perceptible lag when rendering large datasets
- Visual shape of charts is preserved after downsampling

---

## Wave 4 — Integration and Progressive Loading (depends on Waves 2 and 3)

Final integration and polish.

---

### Task 4.1: Progressive Loading and End-to-End Integration

**What:** Ensure the full pipeline works end-to-end: per-chart queries hit the parallelized backend, data flows through cache + singleflight, charts render progressively with skeleton states, and below-fold charts lazy-load.

**Files involved:**
- All modified files (integration testing)
- Potentially minor adjustments to any component for integration issues

**Dependencies:**
- All Wave 1, 2, and 3 tasks

**Implementation details:**
- Verify progressive loading: charts appear one by one as their backend queries complete
- Verify skeleton states render correctly and transition smoothly to chart data
- Verify lazy loading: below-fold charts only query when scrolled into view
- Verify cache behavior: second page load is significantly faster than first
- Verify skip-and-continue: simulate one chart's ClickHouse query failing; other charts still render
- Verify backward compatibility: existing monolithic GraphQL query still returns data (even though frontend no longer uses it)
- Performance test: measure P95 load time against the <2s target

**Done when:**
- Overview page loads in <2s (P95) for typical tenants
- Charts render progressively as data arrives
- Below-fold charts lazy-load on scroll
- Cache reduces ClickHouse query load by >50% on subsequent loads
- One chart failing does not break others
- Existing GraphQL queries continue to work
- Race detector passes on all Go code
- All acceptance criteria from the handoff document are met

---

## Task Dependency Graph

```
Wave 1 (all parallel):
  1.1 Fix retention bug           ─┐
  1.2 Add query limits             ├──→ Wave 2:
  1.3 Create ChartCache           ─┤     2.1 Parallelize service ──→ 2.2 Wire API
                                   │     2.3 Add ClickHouse tests
  1.4 LTTB downsample utility     ─┤
  1.5 ChartGrid lazy loading      ─┤──→ Wave 3:
                                   │     3.1 useChartQuery hook ──→ 3.2 Refactor Overview
                                   │     3.3 Downsample in Chart
                                   │
                                   └──→ Wave 4:
                                         4.1 Integration + progressive loading
```

## Summary Table

| Task | Wave | Files | Depends On | Assignable To |
|------|------|-------|------------|---------------|
| 1.1 Fix retention cohort bug | 1 | `queries.go` | None | Backend dev |
| 1.2 Add query limits / coarser buckets | 1 | `queries.go` | None | Backend dev |
| 1.3 Create ChartCache | 1 | `chart_cache.go`, `chart_cache_test.go` | None | Backend dev |
| 1.4 LTTB downsample utility | 1 | `downsample.ts`, `downsample.test.ts` | None | Frontend dev |
| 1.5 ChartGrid lazy loading | 1 | `ChartGrid.tsx` | None | Frontend dev |
| 2.1 Parallelize service + cache | 2 | `service.go`, `service_test.go` | 1.1, 1.2, 1.3 | Backend dev |
| 2.2 Wire cache into API | 2 | `main.go` | 1.3, 2.1 | Backend dev |
| 2.3 ClickHouse query tests | 2 | `queries_test.go` | 1.1, 1.2 | Backend dev |
| 3.1 useChartQuery hook | 3 | `useChartQuery.ts` | None | Frontend dev |
| 3.2 Refactor Overview.tsx | 3 | `Overview.tsx` | 3.1, 1.5 | Frontend dev |
| 3.3 Downsample in Chart.tsx | 3 | `Chart.tsx` | 1.4 | Frontend dev |
| 4.1 Integration + progressive loading | 4 | All | All prior tasks | Full-stack dev |

## Parallel Execution Plan

With 2 developers (1 backend, 1 frontend):

| Time | Backend Developer | Frontend Developer |
|------|------------------|--------------------|
| Wave 1 | 1.1 → 1.2 → 1.3 | 1.4 → 1.5 |
| Wave 2 | 2.1 → 2.2, 2.3 | 3.1 → 3.2, 3.3 (Wave 3) |
| Wave 4 | 4.1 (joint) | 4.1 (joint) |

With 4 developers (2 backend, 2 frontend):

| Time | BE Dev 1 | BE Dev 2 | FE Dev 1 | FE Dev 2 |
|------|----------|----------|----------|----------|
| Wave 1 | 1.1, 1.2 | 1.3 | 1.4 | 1.5 |
| Wave 2 | 2.3 | 2.1 → 2.2 | 3.1 → 3.2 | 3.3 |
| Wave 4 | 4.1 | — | 4.1 | — |

## Acceptance Criteria (Overall)

- [ ] P95 load time < 2 seconds for overview page
- [ ] All 8 charts show correct data
- [ ] Retention cohort bug is fixed (no full-table scan)
- [ ] Cache reduces ClickHouse query load by >50%
- [ ] Progressive loading: charts render as data arrives
- [ ] Below-fold charts lazy-load on scroll
- [ ] Existing GraphQL queries continue to work unchanged
- [ ] Race detector passes on all Go code (`go test -race ./...`)
- [ ] Downsampling preserves visual shape (LTTB at 500 points)
- [ ] Skip-and-continue: one chart failure does not block others
