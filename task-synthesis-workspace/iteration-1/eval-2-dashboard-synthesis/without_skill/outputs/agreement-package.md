# Agreement Package: Dashboard Performance Optimization

This document presents the synthesized understanding of the task for your review and confirmation before planning begins. Please review each section and confirm, adjust, or reject the understanding.

---

## Task Summary

Optimize the analytics dashboard overview page from 8-10 second load time to under 2 seconds for ~50,000 daily active users. The system is a Go backend (gqlgen + ClickHouse) serving a React frontend (Apollo Client + Recharts). Multiple compounding bottlenecks exist across database queries, backend orchestration, and frontend rendering. A confirmed bug in the retention cohort query means one of the 8 charts has likely never worked.

**Classification:** Refactor (performance optimization of existing functionality)
**Complexity:** High -- multiple bottlenecks across 3 layers with no test coverage

---

## Understanding to Confirm

### 1. Problem Statement

> The dashboard is slow because of compounding bottlenecks at every layer: 8 ClickHouse queries run sequentially with no caching, unbounded result sets are loaded fully into memory, all data is sent to the frontend without downsampling, and all charts render simultaneously as SVG. A cache-busting timestamp in the frontend query variables prevents Apollo's cache from ever hitting.

**Do you agree this captures the core problem?** [ ] Yes [ ] Needs adjustment

---

### 2. Success Criteria

We understand success as:

| Metric | Target | Measurement |
|--------|--------|-------------|
| Page load time | < 2 seconds (P95, wall-clock) | From navigation to all charts visible |
| Time to first chart | < 500ms | From navigation to first chart with data |
| Data accuracy | Unchanged | Same data, possibly with defined staleness from caching |
| API compatibility | Fully backward-compatible | No breaking changes to GraphQL schema |
| Engagement | Non-regression | Dashboard usage rate does not decline |

**Assumption:** The 2-second target is P95. If it is P50 or P99, the scope and effort change significantly.

**Do you agree with these success criteria?** [ ] Yes [ ] Needs adjustment

---

### 3. Minimum Viable Outcome

The absolute minimum that must be delivered:

> **Backend parallelization of chart queries** -- changing the sequential for-loop in `GetOverviewPage` to run chart queries concurrently using errgroup. This is mathematically required because 8 sequential queries at 1-2s each exceed the 2-second target even under optimistic assumptions.

Everything else (caching, frontend downsampling, progressive loading, materialized views) adds value but could be deferred if parallelization alone meets the target for most tenants.

**Do you agree this is the correct minimum viable outcome?** [ ] Yes [ ] Needs adjustment

---

### 4. Proposed Scope

#### In Scope

| Change | Layer | Why |
|--------|-------|-----|
| Parallelize 8 chart queries with errgroup | Backend | Required to meet 2s target |
| Integrate caching with singleflight deduplication | Backend | Reduces ClickHouse load under 50k DAU concurrency |
| Fix retention cohort query bug (parameter mismatch) | Backend | Bug fix -- chart has never worked |
| Add result limits to ClickHouse queries | Backend | Prevents unbounded memory usage |
| Quantize frontend time range to prevent cache busting | Frontend | Enables Apollo cache hits |
| Switch to per-chart queries with progressive loading | Frontend | Shows first chart quickly; enables partial rendering |
| Add data downsampling before chart rendering | Frontend | Prevents 10k+ SVG element render bottleneck |
| Add lazy loading for below-fold charts | Frontend | Defers rendering of non-visible charts |
| Add targeted tests for changed code paths | Both | Required -- zero test coverage currently exists |

#### Out of Scope (Deferred)

| Item | Reason |
|------|--------|
| ClickHouse materialized views | Infrastructure-level change; evaluate need after parallelization+caching |
| ClickHouse secondary indexes | Same as above |
| Dashboard visual redesign | Not a performance issue |
| Other dashboard pages | Requirements scope is overview page only |
| API contract changes (breaking) | Backward compatibility constraint |
| Per-chart time range activation | Behavior change that needs separate product decision |

**Do you agree with this scope boundary?** [ ] Yes [ ] Needs adjustment

---

### 5. Key Constraints

| # | Constraint | Impact |
|---|-----------|--------|
| C1 | GraphQL API must remain backward-compatible | All changes are internal or additive |
| C2 | Zero test coverage across entire codebase | Must add tests before or alongside changes |
| C3 | Skip-and-continue error handling must be preserved | Failed charts must not break the page |
| C4 | ClickHouse schema changes deferred (Phase 1) | No materialized views unless explicitly approved |
| C5 | No deployment downtime | Changes deployed incrementally |

**Do you agree these constraints are correctly identified?** [ ] Yes [ ] Needs adjustment

---

### 6. Key Risks and Mitigations

| # | Risk | Mitigation |
|---|------|------------|
| R1 | Parallel queries overwhelm ClickHouse (8x concurrent load per request) | Configure connection pool; add concurrency semaphore; load test before rollout |
| R2 | Cache thundering herd (50k DAU, popular tenants) | Use `sync/singleflight` for in-flight deduplication |
| R3 | Concurrency bugs in untested code | errgroup for structured concurrency; `go test -race`; add tests first |
| R4 | Downsampling hides operational signals | Per-chart configurable thresholds; finer granularity for error_rate |
| R5 | Naive cache eviction fails silently when full | Replace with LRU; add hit/miss monitoring |
| R6 | No regression safety net | Add targeted tests for all changed code paths |

**Do you agree these risks are correctly identified and prioritized?** [ ] Yes [ ] Needs adjustment

---

### 7. Implementation Approach (High Level)

The proposed implementation order, designed to deliver value incrementally and validate assumptions early:

```
Phase 1 - Backend (reduces server time from SUM to MAX of query times)
  1. Fix retention cohort query bug
  2. Parallelize chart queries with errgroup
  3. Configure ClickHouse connection pool
  4. Integrate caching with singleflight
  5. Add result limits to queries

Phase 2 - Frontend (reduces perceived load time and rendering cost)
  6. Quantize time range variables
  7. Switch to per-chart queries with progressive loading
  8. Add data downsampling before rendering
  9. Add lazy loading for below-fold charts

Phase 3 - Validation
  10. Add targeted tests (unit + integration)
  11. Load test under realistic concurrency
  12. Measure P95 against 2-second target
```

**Note:** Phases 1 and 2 can proceed in parallel after step 2 is validated. Phase 3 runs throughout but is listed last for clarity.

**Do you agree with this general approach?** [ ] Yes [ ] Needs adjustment

---

### 8. Open Questions Requiring Your Input

These questions affect planning decisions. Please provide answers or indicate if you want us to proceed with the stated defaults:

| # | Question | Default Assumption |
|---|----------|-------------------|
| Q1 | What is the acceptable data staleness for cached data? | 30s-1min for operational charts (error_rate), 5-10min for historical charts |
| Q2 | Is the 2-second target P50, P95, or P99? | P95 |
| Q3 | Should the retention cohort bug be fixed in this task? | Yes -- the code is being touched anyway |
| Q4 | Are there other consumers of the GraphQL API? | Unknown -- treating API contract as hard constraint |
| Q5 | What is the ClickHouse topology in production? | Assumed single-node based on Docker Compose; plan accounts for this |
| Q6 | Should the retention cohort query be optimized beyond the bug fix? | Defer -- fix the bug first, optimize if it becomes a bottleneck |

---

## Modules That Will Be Changed

| Module | Path | Change Size |
|--------|------|-------------|
| Analytics Service | `internal/analytics/` | Large |
| ClickHouse Client | `internal/clickhouse/` | Medium |
| Cache Utility | `internal/cache/` | Small-Medium |
| API Entry Point | `cmd/analytics-api/` | Small |
| Dashboard Frontend | `apps/dashboard/` | Large |

---

## Confirmation Request

Please review this agreement package and confirm:

1. **Problem statement** is accurate
2. **Success criteria** are correct (especially the P95 assumption)
3. **Minimum viable outcome** (backend parallelization) is agreed
4. **Scope boundaries** (in-scope vs. deferred) are acceptable
5. **Constraints** are correctly identified
6. **Risks and mitigations** are appropriate
7. **Implementation approach** direction is acceptable
8. **Answers to open questions** (or acceptance of defaults)

Once confirmed, planning will proceed based on this agreed understanding.
