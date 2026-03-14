# Agreement Package

> Task: Optimize analytics dashboard performance (8-10s → <2s P95)
> Based on: Stage 2 analyses (product, system, constraints/risks)
> Purpose: Confirm or correct the synthesized understanding before planning

---

## Block 1 — Goal & Problem Understanding

**Our understanding:**
Reduce the analytics dashboard overview page load time from 8-10 seconds to under 2 seconds (P95). The bottleneck is three-layered: backend sequential ClickHouse queries, unused caching layer, and frontend rendering all charts at once with full data sets. ~50,000 DAU are affected.

**Expected outcome:**
Overview page loads in <2s P95 with all 8 charts showing correct data. Progressive loading shows above-fold charts first.

**Confirm:** Is this the right goal? Are we solving the right problem? Is this the outcome you need?

---

## Block 2 — Scope

**Included:**
- Backend parallelization (errgroup)
- In-memory caching with per-chart-type TTLs
- singleflight for cache miss dedup
- Retention cohort bug fix
- Frontend per-chart queries with progressive loading
- Apollo cache-busting fix
- Data downsampling (≤500 points)
- Lazy loading for below-fold charts

**Excluded:**
- ClickHouse schema changes
- Chart library switch
- GraphQL breaking changes
- Real-time streaming
- Per-chart time range config

**Confirm:** Is the scope correct? Anything missing? Anything that shouldn't be here?

---

## Block 3 — Key Scenarios

**Primary scenario:**
User opens overview → per-chart queries fire → above-fold charts load first → backend: cache check → singleflight → parallel ClickHouse → data downsampled → charts render → all visible in <2s.

**Mandatory edge cases:**
- Cache cold start → singleflight prevents thundering herd
- Large tenant → query limits prevent unbounded results
- Retention cohort → bug fix makes it work
- ClickHouse down → skip-and-continue preserved

**Deferred:**
- Materialized views
- Per-chart time ranges (UX decision)
- Real-time streaming

**Confirm:** Is the primary scenario correct? Are the mandatory edge cases right? Can the deferred items really wait?

---

## Block 4 — Constraints

- GraphQL API must remain backward compatible
- 2-second P95 target
- Zero test coverage — new tests needed
- ClickHouse schema out of scope
- Skip-and-continue pattern preserved

**Confirm:** Are these constraints accurate? Are there constraints we missed? Can any of these be relaxed?

---

## Block 5 — Candidate Solution Directions

Based on the analysis, we see these possible directions:

- **Backend-first:** Parallelization + caching only. Simpler, addresses the biggest bottleneck. Frontend unchanged.
- **Full-stack optimization:** Backend parallelization + caching + frontend progressive loading + downsampling. More work but hits the 2s target more reliably.

**Confirm:** Which direction do you prefer? Backend-only or full-stack? Any direction we should avoid?
