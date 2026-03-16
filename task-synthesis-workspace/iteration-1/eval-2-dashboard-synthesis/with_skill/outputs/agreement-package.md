# Agreement Package

> Task: Optimize analytics dashboard overview page load time from 8-10s to under 2s
> Based on: Stage 2 analyses (product, system, constraints/risks)
> Purpose: Confirm or correct the synthesized understanding before planning

---

## Block 1 — Goal & Problem Understanding

**Our understanding:**
The analytics dashboard's main overview page is the primary data interface for ~50,000 daily active users. It currently takes 8-10 seconds to load all 8 charts, which is 4-5x slower than the 2-second target. This is caused by multiple compounding bottlenecks: sequential ClickHouse queries on the backend, no caching at any layer, unbounded result sets, all charts rendered simultaneously as SVG on the frontend, and a cache-busting bug that prevents client-side caching from ever working. The slow performance is eroding user trust in the data platform.

**Expected outcome:**
The overview page loads all 8 charts with correct data within 2 seconds (wall-clock, from the user's perspective). The experience feels responsive, with charts appearing progressively rather than all at once after a long wait. The optimization does not change the visual output, data accuracy, or GraphQL API contract.

**Confirm:** Is this the right goal? Are we solving the right problem? Is this the outcome you need?

---

## Block 2 — Scope

**Included:**
- Backend parallelization of the 8 sequential chart queries (errgroup)
- Server-side caching integration using the existing cache utility (with improvements)
- Fix the retention cohort query bug (5 SQL params, only 3 passed -- chart has likely never worked)
- Fix the frontend cache-busting time range (quantize timestamps)
- Frontend progressive loading (switch from monolithic query to per-chart queries)
- Frontend data downsampling before chart rendering (cap SVG elements)
- Lazy loading for below-fold charts
- Adding result limits to ClickHouse queries
- Connection pool configuration for parallel query workload

**Excluded:**
- ClickHouse materialized views or schema-level infrastructure changes (marked as "to be confirmed" in requirements)
- Changes to the GraphQL API contract that would break existing consumers
- Other dashboard pages beyond the main overview
- Switching the charting library (Recharts is retained)
- Per-chart time range activation (the unused `ChartConfig.TimeRange` field)
- Adding comprehensive test coverage (tests may be added for new parallel code paths, but a full test suite is not in scope)

**Confirm:** Is the scope correct? Anything missing? Anything that shouldn't be here?

---

## Block 3 — Key Scenarios

**Primary scenario:**
User opens the analytics dashboard overview page. Charts begin appearing progressively within ~500ms. All 8 charts are visible and interactive within 2 seconds. Repeat visits within a short window benefit from cached data. The dashboard feels responsive and trustworthy.

**Mandatory edge cases:**
- Large tenants (>10M events) -- these are the primary driver of the problem, with queries taking 2-5 seconds each. Backend parallelization is mathematically required for these tenants.
- Concurrent page loads by many users of the same tenant (thundering herd) -- at 50k DAU, this is near-certain during peak hours. Cache must use request coalescing (singleflight) to avoid multiplying ClickHouse load.
- Cache-busting time range on the frontend -- must be fixed or all client-side caching remains useless.
- Retention cohort query bug -- must be fixed as part of this work since it directly affects one of the 8 charts.

**Deferred (not in this task):**
- ClickHouse materialized views (infrastructure-level change, potentially out of scope)
- Optimization of other dashboard pages
- Activation of per-chart time range configuration

**Confirm:** Is the primary scenario correct? Are the mandatory edge cases right? Can the deferred items really wait?

---

## Block 4 — Constraints

- **GraphQL API contract must be backward-compatible:** Unknown consumers may exist; changes must be additive, not breaking
- **2-second target is wall-clock (user perspective):** Includes network, backend, and frontend rendering time. Percentile (P50/P95/P99) is undefined in requirements -- we assume P95 as a reasonable target
- **Zero existing test coverage:** No test files anywhere in the codebase (backend or frontend). Changes carry inherent regression risk
- **No existing concurrency patterns:** The codebase has never used goroutines or errgroups. Parallelization is a new pattern with no existing examples to follow
- **Skip-and-continue error handling must be preserved:** Failed charts must not crash the page -- this is a deliberate resilience pattern
- **ClickHouse schema changes may be out of scope:** Materialized views are infrastructure-level and marked as "to be confirmed"
- **No downtime for deployment:** Live production system serving 50k DAU

**Confirm:** Are these constraints accurate? Are there constraints we missed? Can any of these be relaxed?

---

## Block 5 — Candidate Solution Directions

Based on the analysis, we see these possible directions:

- **Minimal (backend parallelization only):** Parallelize the 8 sequential queries using errgroup, fix the retention cohort bug, configure connection pool. This is the mathematical minimum to reach 2 seconds. Trade-off: may not suffice for the very largest tenants without caching; does not address frontend rendering bottleneck. Lower risk, smaller scope.

- **Safe (backend + caching + frontend cache fix):** Add parallelization, integrate server-side caching (with singleflight for thundering herd), and fix the frontend cache-busting time range. Addresses the three most impactful bottlenecks. Trade-off: moderate scope but reliably meets the 2-second target across tenant sizes.

- **Systematic (full stack optimization):** All of the above plus frontend progressive loading (per-chart queries), data downsampling in charts, lazy loading of below-fold charts. Trade-off: largest scope touching all three layers, but produces the most complete solution and best perceived performance. The dashboard would feel genuinely fast, not just technically meeting a target.

**Confirm:** Which direction do you prefer? Minimal and safe, or systematic and thorough? Any direction we should avoid?
