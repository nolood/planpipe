# Agreed Task Model: Dashboard Performance Optimization

**Status:** CONFIRMED (user accepted agreement package)
**Date:** 2026-03-14

---

## 1. Task Identity

- **Title:** Dashboard Overview Page Performance Optimization
- **Type:** Refactor (performance optimization of existing functionality)
- **Complexity:** High
- **Target:** Reduce overview page load time from 8-10s to <2s (P95, wall-clock)

## 2. Problem Definition

The analytics dashboard overview page has degraded to 8-10 second load times due to compounding bottlenecks across three layers:

1. **Database layer:** 8 ClickHouse queries hit a raw 500M-row events table with no pre-aggregation, producing 2-5s per query for large tenants. One query (retention cohort) has a parameter mismatch bug and has never worked.
2. **Backend layer:** Queries execute sequentially in a for-loop, making total time the SUM of individual query times (16-40s worst case). No caching exists despite an unused cache utility.
3. **Frontend layer:** A monolithic GraphQL query blocks until all charts complete. Millisecond-precision timestamps cache-bust Apollo Client. All 8 charts render simultaneously with no downsampling (10k+ SVG elements per chart).

The degradation is driven by data growth outpacing an implementation that assumed small data volumes. It is actively harming user trust and engagement for ~50,000 daily active users.

## 3. Success Criteria (Confirmed)

| Metric | Target | Measurement Method |
|--------|--------|-------------------|
| P95 page load time | < 2 seconds | Wall-clock from navigation to all charts visible |
| Time to first chart | < 500ms | From navigation to first chart with data rendered |
| Data accuracy | Unchanged | Same data as current (with defined staleness from caching) |
| API compatibility | Fully backward-compatible | No breaking changes to GraphQL schema or response types |
| Dashboard engagement | Non-regression | Usage rate does not decline post-optimization |
| ClickHouse load | Reduced | Query volume decrease from caching effectiveness |
| Chart error rate | Non-regression | Failed chart loads do not increase |

## 4. Minimum Viable Outcome (Confirmed)

**Backend parallelization of chart queries using errgroup.** This changes the total backend time from SUM(query_times) to MAX(query_times), which is the single largest improvement and is mathematically required to meet the 2-second target for large tenants.

## 5. Confirmed Scope

### In Scope

| # | Change | Layer | Priority | Rationale |
|---|--------|-------|----------|-----------|
| 1 | Fix retention cohort query bug (5 params needed, 3 passed) | Backend | P0 | Bug -- chart has never worked |
| 2 | Parallelize 8 chart queries with errgroup | Backend | P0 | Required to meet 2s target |
| 3 | Configure ClickHouse connection pool for parallel access | Backend | P0 | Prerequisite for safe parallelization |
| 4 | Integrate caching with singleflight deduplication | Backend | P1 | Reduces ClickHouse load under concurrency; handles thundering herd |
| 5 | Add result limits to ClickHouse queries | Backend | P1 | Prevents unbounded memory usage |
| 6 | Quantize frontend time range to nearest minute | Frontend | P1 | Enables Apollo Client cache hits |
| 7 | Switch to per-chart queries with progressive loading | Frontend | P1 | Perceived performance; first chart visible quickly |
| 8 | Add data downsampling before chart rendering | Frontend | P2 | Caps SVG elements; configurable per chart type |
| 9 | Add lazy loading for below-fold charts | Frontend | P2 | Defers non-visible chart rendering |
| 10 | Add targeted tests for changed code paths | Both | P0 | Zero coverage currently; required for safety |

### Out of Scope (Confirmed Deferred)

| Item | Reason |
|------|--------|
| ClickHouse materialized views / secondary indexes | Infrastructure-level; evaluate need after Phase 1 results |
| Dashboard visual redesign | Not a performance concern |
| Other dashboard pages beyond overview | Requirements scope boundary |
| Breaking GraphQL API changes | Backward compatibility constraint |
| Per-chart time range activation | Behavior change requiring separate product decision |

## 6. Constraints (Confirmed)

| # | Constraint | Type | Hardness |
|---|-----------|------|----------|
| C1 | GraphQL API must remain backward-compatible for unknown consumers | Compatibility | HARD |
| C2 | 2-second wall-clock target from user perspective (P95) | Performance | HARD |
| C3 | Zero test coverage -- changes need new tests or manual verification | Technical | HARD (fact) |
| C4 | Skip-and-continue error handling preserved (failed charts do not fail page) | Architectural | HARD |
| C5 | ClickHouse schema changes deferred to Phase 2 | Scope | SOFT |
| C6 | No deployment downtime -- changes deployed incrementally | Operational | MEDIUM |

## 7. Risks and Mitigations (Confirmed)

| # | Risk | L | I | Mitigation | Owner |
|---|------|---|---|------------|-------|
| R1 | Parallel queries overwhelm ClickHouse connection pool | M | H | Configure explicit pool size; add concurrency semaphore (max 8 per request); load test before rollout | Backend |
| R2 | Cache thundering herd under 50k DAU concurrent load | H | M | Use `sync/singleflight` to deduplicate in-flight queries per cache key | Backend |
| R3 | Concurrency bugs in untested code (races, deadlocks) | M | H | errgroup for structured concurrency; `go test -race`; add tests before parallelization | Backend |
| R4 | Downsampling hides operational signals (error spikes) | M | M | Per-chart configurable downsampling thresholds; finer granularity for error_rate chart | Frontend |
| R5 | Naive cache eviction fails silently when full | M | M | Replace with LRU/size-bounded cache; add cache hit/miss rate monitoring | Backend |
| R6 | No regression safety net (zero tests across codebase) | H | H | Add targeted tests for parallelized path, cache integration, query correctness before deploying changes | Both |

## 8. Resolved Questions

| # | Question | Resolution |
|---|----------|------------|
| Q1 | Acceptable data staleness for cached data | 30s-1min TTL for operational charts (error_rate); 5-10min TTL for historical charts (retention, funnel) |
| Q2 | 2-second target percentile | P95 |
| Q3 | Fix retention cohort bug in this task? | Yes -- code is being modified in this area anyway |
| Q4 | Other GraphQL API consumers? | Unknown -- treat API contract as hard constraint |
| Q5 | ClickHouse production topology? | Assumed single-node; plan accounts for connection pool limits |
| Q6 | Optimize retention cohort query beyond bug fix? | Defer -- fix the parameter bug first; optimize the self-join only if it becomes a measured bottleneck after parallelization |

## 9. System Map (Confirmed)

### Modules and Roles

| Module | Path | Role | Change Size |
|--------|------|------|-------------|
| Analytics Service | `internal/analytics/` | Orchestration layer; contains sequential bottleneck | Large |
| ClickHouse Client | `internal/clickhouse/` | Query execution; contains SQL definitions and bug | Medium |
| Cache Utility | `internal/cache/` | Unused TTL cache to be integrated | Small-Medium |
| API Entry Point | `cmd/analytics-api/` | Server wiring; needs cache initialization | Small |
| Dashboard Frontend | `apps/dashboard/` | Monolithic query, no progressive loading, no downsampling | Large |

### Key Change Points

| # | Location | Change | Scope |
|---|----------|--------|-------|
| CP1 | `internal/analytics/service.go:GetOverviewPage` (lines 24-55) | Parallelize sequential chart loop with errgroup | Medium |
| CP2 | `internal/analytics/service.go:loadChart` | Integrate caching (check cache -> query -> store) | Medium |
| CP3 | `internal/clickhouse/queries.go:67-85` | Fix retention cohort: 5 SQL params but 3 passed | Small |
| CP4 | `internal/clickhouse/queries.go` (all queries) | Add result limits; coarsen time buckets where appropriate | Medium |
| CP5 | `internal/clickhouse/client.go:ExecuteQuery` | Support variable parameter counts | Medium |
| CP6 | `cmd/analytics-api/main.go` | Initialize cache; inject into service | Small |
| CP7 | `apps/dashboard/src/pages/Overview.tsx` | Per-chart queries with progressive loading | Large |
| CP8 | `apps/dashboard/src/pages/Overview.tsx:26-27` | Quantize time range to prevent Apollo cache busting | Small |
| CP9 | `apps/dashboard/src/components/Chart.tsx:50-55` | Data downsampling before rendering (cap ~200-500 points) | Medium |
| CP10 | `apps/dashboard/src/components/ChartGrid.tsx` | Lazy loading for below-fold charts | Medium |

### Critical Dependencies

| Dependency | Type | Risk |
|-----------|------|------|
| ClickHouse cluster | External database | 8x concurrent query load per request after parallelization |
| gqlgen framework | Upstream library | Constrains resolver architecture for parallelization approach |
| Apollo Client | Frontend library | Cache policy and query management for per-chart loading |
| Recharts | Frontend library | SVG rendering model -- data points directly correlate to render time |

## 10. Implementation Phases (Confirmed Direction)

```
Phase 1 - Backend (target: server time from SUM to MAX of query times)
  Step 1: Fix retention cohort query parameter bug
  Step 2: Add tests for existing behavior (baseline)
  Step 3: Parallelize chart queries with errgroup
  Step 4: Configure ClickHouse connection pool
  Step 5: Integrate caching with singleflight
  Step 6: Add result limits to queries
  Validation: Measure backend response time improvement

Phase 2 - Frontend (target: perceived load time and rendering cost)
  Step 7: Quantize time range variables
  Step 8: Switch to per-chart queries with progressive loading
  Step 9: Add data downsampling before rendering
  Step 10: Add lazy loading for below-fold charts
  Validation: Measure wall-clock page load time

Phase 3 - Verification
  Step 11: Load test under realistic concurrency (50k DAU simulation)
  Step 12: Measure P95 against 2-second target
  Step 13: Monitor cache hit rates and ClickHouse query volume
  Step 14: Validate error_rate chart data granularity after downsampling

Decision Gate: If P95 > 2s after Phases 1+2, evaluate ClickHouse
materialized views (currently deferred).
```

## 11. Existing Patterns to Preserve

| Pattern | Location | Why |
|---------|----------|-----|
| Skip-and-continue error handling | `service.go:33-36` | Deliberate resilience -- failed charts must not fail the page |
| Named query registry | `queries.go:14` | Clean separation of SQL from logic |
| Resolver-delegates-to-service | `resolver.go` | gqlgen pattern -- performance changes go in service layer, not resolvers |
| Apollo useQuery pattern | `Overview.tsx` | Frontend data-fetching convention |
| Cache-first Apollo policy | `Overview.tsx` | Will become effective once time range quantization is applied |
