# Stage 5 Handoff — Implementation Decomposition Complete

## Task Summary
Reduce the analytics dashboard overview page load time from 8-10 seconds to under 2 seconds (P95) through full-stack optimization: backend parallelization with errgroup, in-memory caching with per-chart TTLs and singleflight, retention cohort bug fix, and frontend per-chart queries with progressive loading and client-side downsampling.

## Classification
- **Type:** refactor (performance)
- **Complexity:** high
- **Total subtasks:** 10
- **Execution waves:** 4
- **Max parallel subtasks:** 4
- **Solution direction:** systematic

## Implementation Approach
Stage 4 designed a three-layer full-stack optimization: (1) backend parallelization with errgroup + in-memory cache with singleflight, (2) ClickHouse query bug fix and limits, (3) frontend per-chart queries with progressive loading, LTTB downsampling, and lazy loading. The decomposition breaks this into 10 subtasks organized into 4 execution waves, maximizing parallel execution with up to 4 subtasks running simultaneously.

## Execution Strategy
The work is organized into 4 waves. Wave 1 (Foundation) runs 4 independent subtasks in parallel: retention bug fix, query limits, cache module, and downsampling utility — all with zero file overlap. Wave 2 (Core Implementation) runs 4 parallel subtasks: backend service parallelization (depends on cache), per-chart query hook, lazy loading, and Chart.tsx downsampling integration (depends on downsample utility). Wave 3 (Frontend Integration) refactors Overview.tsx to use per-chart queries (depends on hook). Wave 4 (Convergence) wires the cache into the API server (depends on cache + service). The backend critical path (ST-3 -> ST-5 -> ST-10) and frontend critical path (ST-6 -> ST-8) run independently.

## Subtask Summary

| ID | Title | Type | Wave | Scope | Blocking Dependencies | Completion Criteria Summary |
|----|-------|------|------|-------|-----------------------|---------------------------|
| ST-1 | Fix retention cohort bug | foundation | 1 | small | none | WHERE clause fixed at queries.go:67-85; tests pass |
| ST-2 | Add ClickHouse query limits and coarser time buckets | foundation | 1 | small | none | LIMIT applied; coarser GROUP BY for >90d; tests pass |
| ST-3 | Create ChartCache with per-chart TTL and singleflight | foundation | 1 | medium | none | Get/Set/Invalidate work; TTL expiry; singleflight dedup; concurrent access safe |
| ST-4 | Implement LTTB downsampling utility | foundation | 1 | small | none | LTTB reduces to target points; preserves shape; edge cases handled |
| ST-5 | Parallelize analytics service with errgroup and cache integration | implementation | 2 | large | ST-3 | errgroup with limit 8; cache integrated; singleflight wraps loaders; skip-and-continue; race detector passes |
| ST-6 | Create useChartQuery hook | implementation | 2 | medium | none | Per-chart GraphQL query; returns {data, loading, error}; time range quantized |
| ST-7 | Add Intersection Observer lazy loading to ChartGrid | implementation | 2 | small | none | Below-fold charts deferred; skeleton placeholders shown |
| ST-9 | Integrate downsampling into Chart.tsx | integration | 2 | small | ST-4 | maxPoints prop (default 500); LTTB applied; Recharts compatible |
| ST-8 | Refactor Overview.tsx for per-chart queries and progressive loading | integration | 3 | medium | ST-6 | Monolithic query removed; per-chart useChartQuery; progressive rendering; skeletons |
| ST-10 | Wire cache initialization in API server entry point | integration | 4 | small | ST-3, ST-5 | ChartCache initialized with TTL config; passed to AnalyticsService; startup verified |

## Execution Waves

### Wave 1 — Foundation
**Parallel group:** ST-1, ST-2, ST-3, ST-4
**Establishes:** Fixed ClickHouse queries, bounded query results, cache module with singleflight, LTTB downsampling utility

### Wave 2 — Core Implementation
**Parallel group:** ST-5 || ST-6 || ST-7 || ST-9; ST-5 after ST-3; ST-9 after ST-4
**Builds:** Parallelized backend service with caching, per-chart query hook, lazy loading, Chart.tsx downsampling

### Wave 3 — Frontend Integration
**Sequential:** ST-8 after ST-6
**Builds:** Refactored Overview page with progressive per-chart loading

### Wave 4 — Convergence
**Sequential:** ST-10 after ST-3 + ST-5
**Validates:** Complete backend wiring (cache -> service -> API server)

## Dependency Graph

```
ST-1 (fix retention bug) ─soft─→ ST-5 (parallelize service)
ST-2 (query limits) ─soft─→ ST-5 (parallelize service)
ST-3 (cache module) ─block─→ ST-5 (parallelize service)
ST-3 (cache module) ─block─→ ST-10 (API wiring)
ST-4 (downsample util) ─block─→ ST-9 (Chart.tsx integration)
ST-5 (parallelize service) ─block─→ ST-10 (API wiring)
ST-6 (useChartQuery hook) ─block─→ ST-8 (Overview.tsx refactor)
ST-7 (lazy loading) — terminal (no downstream)
ST-8 (Overview.tsx refactor) — terminal (no downstream)
ST-9 (Chart.tsx downsampling) — terminal (no downstream)
ST-10 (API wiring) — terminal (no downstream)
```

## Conflict Zones
| Zone | Subtasks | Resolution |
|------|----------|------------|
| `internal/clickhouse/queries.go` | ST-1, ST-2 | Non-overlapping changes (different query functions). Low severity — trivial merge. |
| `internal/clickhouse/queries_test.go` | ST-1, ST-2 | Additive test changes to different test functions. Low severity — trivial merge. |

## Coverage Verification
- **Verdict:** COVERAGE_OK
- **Confidence:** high
- **All acceptance criteria mapped:** yes (8/8 criteria -> specific subtask completion criteria)
- **All change map files covered:** yes (13/13 files assigned to subtasks)
- **All design decisions traceable:** yes (6/6 decisions reflected in subtasks)

## Constraints Respected
- **GraphQL backward compatibility:** Per-chart queries are additive; existing monolithic query still works (ST-6, ST-8)
- **P95 <2s target:** Combined effect of parallel backend (ST-5), caching (ST-3), progressive loading (ST-8), downsampling (ST-9), lazy loading (ST-7)
- **Zero test coverage:** Tests added in ST-1, ST-2, ST-3, ST-4, ST-5, ST-6
- **No ClickHouse schema changes:** All changes are code-level (ST-1, ST-2)
- **Skip-and-continue pattern:** Preserved in ST-5 (errgroup error handling)

## Risks for Execution
| Risk | Affected Subtasks | Mitigation | Severity |
|------|-------------------|------------|----------|
| ClickHouse connection pool exhaustion from parallel queries | ST-5 | errgroup concurrency limit of 8; pool size is 20 | medium |
| Cache cold start thundering herd | ST-3, ST-5 | singleflight deduplicates concurrent requests | medium |
| Retention bug fix regression | ST-1 | Test with production-like data; verify result accuracy | medium |
| Race conditions in parallel service | ST-5 | Each goroutine uses own connection; no shared mutable state; race detector in tests | medium |
| Merge conflicts in queries.go/queries_test.go | ST-1, ST-2 | Changes target different locations; trivial merge | low |
| Downsampling hides data signals | ST-4, ST-9 | 500-point threshold is conservative; LTTB preserves shape; zoom shows full data | low |

## User Decisions Log
User review skipped (test run). Key decisions from earlier stages carried forward:
- Full-stack optimization scope (not backend-only)
- In-memory cache (not Redis)
- P95 target of 2 seconds
- LTTB downsampling at 500 points

## Acceptance Criteria
- P95 load <2s
- All 8 charts correct data
- Retention cohort works
- GraphQL contract unchanged
- Progressive loading works
- Apollo cache hits
- ClickHouse load reduced
- Race detector passes

## Detailed References
- `execution-backlog.md` — complete execution backlog with all subtasks
- `coverage-matrix.md` — requirement-to-subtask traceability
- `decomposition-review-package.md` — user review document
- `implementation-design.md` — implementation design (Stage 4)
- `change-map.md` — file-level change map (Stage 4)
- `design-decisions.md` — decision journal (Stage 4)
- `agreed-task-model.md` — agreed task model (Stage 3)
