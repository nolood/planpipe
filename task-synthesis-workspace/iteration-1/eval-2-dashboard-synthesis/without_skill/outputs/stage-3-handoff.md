# Stage 3 Handoff: Dashboard Performance Optimization

**From:** Synthesis (Stage 2.5)
**To:** Planning (Stage 3)
**Status:** Task model agreed and confirmed. Ready for detailed planning.

---

## Task Summary

Optimize the analytics dashboard overview page from 8-10 second load time to under 2 seconds (P95, wall-clock) for ~50,000 daily active users. The system is a Go backend (gqlgen GraphQL API + ClickHouse) serving a React frontend (Apollo Client + Recharts). Multiple compounding bottlenecks span three layers. The task is classified as a high-complexity refactor with the primary risk being the introduction of concurrency and caching into a codebase with zero test coverage.

## Classification

- **Type:** refactor (performance optimization of existing functionality)
- **Complexity:** high -- multi-layer bottlenecks across database, API, and frontend with no test coverage
- **Primary risk area:** technical -- concurrency in untested code; connection pool management; cache correctness under load

## Agreed Success Criteria

| Metric | Target |
|--------|--------|
| P95 page load time | < 2 seconds (wall-clock, navigation to all charts visible) |
| Time to first chart | < 500ms |
| Data accuracy | Unchanged (with defined staleness: 30s-1min operational, 5-10min historical) |
| API compatibility | Fully backward-compatible (GraphQL schema preserved) |
| Chart error rate | Non-regression |

## Agreed Minimum Viable Outcome

Backend parallelization of chart queries using errgroup. Changes total backend time from SUM(query_times) to MAX(query_times). Mathematically required for the 2-second target.

## Scope Boundary

### In Scope (Confirmed)

**Backend (Go):**
1. Fix retention cohort query bug (`queries.go:67-85` -- 5 params needed, 3 passed)
2. Parallelize 8 chart queries with errgroup (`service.go:24-55`)
3. Configure ClickHouse connection pool for parallel access
4. Integrate caching with `sync/singleflight` deduplication (`cache.go` + `service.go`)
5. Add result limits to unbounded ClickHouse queries
6. Support variable parameter counts in `ExecuteQuery`
7. Wire cache initialization in `main.go`

**Frontend (React):**
8. Quantize time range to nearest minute (`Overview.tsx:26-27`)
9. Switch from monolithic `overviewPage` to per-chart `chartData` queries with progressive loading
10. Add data downsampling before rendering (~200-500 point cap, configurable per chart type)
11. Add lazy loading / intersection observer for below-fold charts

**Testing:**
12. Add targeted tests for parallelized path, cache integration, and query correctness

### Out of Scope (Confirmed Deferred)

- ClickHouse materialized views / secondary indexes (evaluate if P95 > 2s after implementation)
- Per-chart time range activation (separate product decision)
- Other dashboard pages
- Breaking GraphQL API changes
- Dashboard visual redesign

## Constraints the Plan Must Respect

| # | Constraint | Hardness |
|---|-----------|----------|
| C1 | GraphQL API backward-compatible for unknown consumers | HARD |
| C2 | 2-second P95 wall-clock target | HARD |
| C3 | Zero test coverage -- must add tests alongside changes | HARD |
| C4 | Skip-and-continue error handling preserved | HARD |
| C5 | ClickHouse schema changes deferred | SOFT |
| C6 | No deployment downtime | MEDIUM |

## Risks the Plan Must Mitigate

| # | Risk | L | I | Required Mitigation |
|---|------|---|---|---------------------|
| R1 | Parallel queries overwhelm ClickHouse | M | H | Explicit pool size config; concurrency semaphore; load test |
| R2 | Cache thundering herd (50k DAU) | H | M | `sync/singleflight` per cache key |
| R3 | Concurrency bugs (races, deadlocks) | M | H | errgroup; `go test -race`; tests before parallelization |
| R4 | Downsampling hides error spikes | M | M | Per-chart configurable thresholds |
| R5 | Naive cache eviction fails silently | M | M | LRU replacement; hit/miss monitoring |
| R6 | No regression safety net | H | H | Targeted tests for all changed paths |

## System Map for Planning

### Modules

| Module | Path | Change Size | Key Entry Points |
|--------|------|-------------|-----------------|
| Analytics Service | `internal/analytics/` | Large | `service.go:GetOverviewPage` (line 24), `service.go:loadChart` |
| ClickHouse Client | `internal/clickhouse/` | Medium | `client.go:ExecuteQuery` (line 54), `queries.go` (query map) |
| Cache Utility | `internal/cache/` | Small-Medium | `cache.go:Get`, `cache.go:Set` |
| API Entry Point | `cmd/analytics-api/` | Small | `main.go` (service initialization) |
| Dashboard Frontend | `apps/dashboard/` | Large | `Overview.tsx`, `Chart.tsx`, `ChartGrid.tsx`, `analytics.ts` |

### Change Dependency Graph

```
                    [CP3: Fix retention bug]
                           |
                    [CP1: Parallelize queries]
                      /         \
            [CP5: Connection     [CP2: Integrate cache
             pool config]         + singleflight]
                                    |
                              [CP4: Add query limits]

--- Backend above / Frontend below ---

            [CP8: Quantize time range]
                     |
            [CP7: Per-chart queries + progressive loading]
                  /            \
      [CP9: Downsampling]   [CP10: Lazy loading]
```

CP3 (bug fix) is independent and can start immediately.
CP1 (parallelization) is the critical path prerequisite.
Backend (CP1-CP6) and Frontend (CP7-CP10) can proceed in parallel after CP1 is validated.

### Existing Patterns to Preserve

| Pattern | Location | Constraint |
|---------|----------|-----------|
| Skip-and-continue error handling | `service.go:33-36` | Parallel execution must still skip failed charts, not fail the page |
| Named query registry | `queries.go:14` | SQL modifications go in the query map, not inline |
| Resolver-delegates-to-service | `resolver.go` | Performance changes in service layer, not resolvers |
| Apollo useQuery | `Overview.tsx` | Frontend keeps Apollo patterns; switch to per-chart queries using existing `CHART_DATA_QUERY` |

## Resolved Questions

| Question | Answer |
|----------|--------|
| Data staleness tolerance | 30s-1min for operational (error_rate); 5-10min for historical (retention, funnel) |
| 2-second target percentile | P95 |
| Fix retention bug here? | Yes |
| Other API consumers? | Unknown -- API contract treated as hard constraint |
| ClickHouse topology | Assumed single-node; plan for connection pool limits |
| Optimize retention query beyond bug fix? | Defer -- fix first, optimize if measured bottleneck |

## Implementation Direction (Agreed)

The plan should be structured in two parallel tracks after the bug fix:

**Track A - Backend:** Bug fix -> Tests for baseline -> Parallelize -> Pool config -> Cache + singleflight -> Query limits
**Track B - Frontend:** Quantize time range -> Per-chart queries + progressive loading -> Downsampling -> Lazy loading

**Decision gate after implementation:** If P95 still exceeds 2 seconds, evaluate ClickHouse materialized views as a Phase 2 escalation.

## Detailed Analyses (Reference)

The following files contain the full analysis details and can be consulted during planning:
- `product-analysis.md` -- business intent, actor scenarios, success signals, edge cases
- `system-analysis.md` -- module map, change points, dependencies, technical observations, test coverage
- `constraints-risks-analysis.md` -- constraints, risks, backward compatibility, sensitive areas
- `analysis.md` -- synthesized cross-analysis with reconciled tensions and unified system map
- `agreement-package.md` -- the confirmed agreement between analysis and user
- `agreed-task-model.md` -- the complete confirmed task model with all resolved questions
