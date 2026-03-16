# Design Review Package -- Dashboard Performance Optimization

## Executive Summary

This design reduces the analytics dashboard overview page load time from 8-10 seconds to under 2 seconds (P95) through coordinated optimizations across three layers: backend query execution, caching, and frontend rendering. The GraphQL API contract is preserved. No infrastructure changes (ClickHouse schema, new services) are required.

**Estimated effort:** 10-15 engineering days across 5 sprints.
**Risk level:** Medium (introducing concurrency into untested code is the primary risk).
**Files changed:** 9 existing + 4 new (1 component + 3 test files).

---

## Problem Statement

The dashboard overview page serving 50,000 DAU loads in 8-10 seconds due to compounding bottlenecks:

1. **Backend:** 8 ClickHouse queries run sequentially (sum = 16-40s for large tenants)
2. **Backend:** No caching -- every page load hits ClickHouse
3. **Backend:** One chart query (retention cohort) has a parameter bug and has never worked
4. **Frontend:** Single monolithic GraphQL query blocks rendering until all 8 charts are ready
5. **Frontend:** No data downsampling -- charts render 10,000+ SVG elements, causing 2-5s rendering
6. **Frontend:** Apollo cache never hits due to millisecond-precision timestamps in query variables

---

## Solution Architecture

### Before
```
User -> Overview.tsx (1 monolithic query)
  -> GraphQL resolver
    -> service.GetOverviewPage (sequential for-loop)
      -> ClickHouse query 1 (2-5s)
      -> ClickHouse query 2 (2-5s)
      -> ... x8 sequential
      -> ClickHouse summary (4 sequential QueryRow calls)
    -> Return all data at once
  -> Render all 8 charts simultaneously (10k+ SVG elements each)

Total: 8-10s (backend sum + rendering)
```

### After
```
User -> Overview.tsx (8 per-chart queries + 1 summary query, parallel)
  -> GraphQL resolvers (one per chart)
    -> service.loadChart (check cache -> singleflight -> ClickHouse)
      -> Cache hit: ~1ms
      -> Cache miss: single ClickHouse query (0.5-2s with query optimization)
    -> Return per chart
  -> Render each chart as data arrives (max 500 SVG elements each)
  -> Below-fold charts lazy-loaded on scroll

Total: <2s P95 (max single query + rendering)
Cache hit: <500ms
```

---

## Change Summary by Layer

### Backend (Go)

| Change | Impact | Risk |
|--------|--------|------|
| Parallelize 8 chart queries with `errgroup` | Backend time: sum -> max of queries | High (new concurrency pattern in untested code) |
| Fix retention cohort query bug (5 placeholders, 3 params) | Retention chart works for the first time | Low (clear bug fix) |
| Coarsen time buckets in 2 queries (events_volume, error_rate) | Reduce result set sizes by 4-40x | Medium (changes data granularity) |
| Add LIMIT clauses to 3 unbounded queries | Cap result sets, reduce memory | Low |
| Configure ClickHouse connection pool (MaxOpenConns=12) | Support parallel queries without connection starvation | Low |
| Integrate in-memory cache with per-chart TTLs (1-30 min) | Eliminate repeat ClickHouse queries | Medium (cache coherency) |
| Add singleflight for cache miss deduplication | Prevent thundering herd under 50k DAU | Low (well-understood pattern) |

### Frontend (React)

| Change | Impact | Risk |
|--------|--------|------|
| Fix Apollo cache-busting (quantize timestamps to 5-min) | Enable Apollo cache hits on page refresh | Low |
| Split monolithic query into 8 per-chart queries | Progressive loading -- first chart in <1s | Medium (structural change) |
| Add data downsampling (cap at 500 points per chart) | Rendering time: 2-5s -> 50-100ms per chart | Medium (potential visual detail loss) |
| Lazy-load below-fold charts (IntersectionObserver) | Reduce initial render work by ~50% | Low |

### Testing (new)

| File | Coverage |
|------|----------|
| `service_test.go` | Parallel execution, skip-and-continue, caching, singleflight, race detector |
| `queries_test.go` | Query parameter count validation (catches retention-class bugs) |
| `cache_test.go` | Per-key TTL, LRU eviction, concurrent access |

---

## Key Design Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | errgroup over raw goroutines | Structured concurrency, context propagation, concurrency limits. Standard Go pattern. |
| 2 | In-memory cache over Redis | User confirmed in-memory is sufficient for <50k DAU. No infrastructure changes needed. |
| 3 | Per-chart TTLs (1-30 min) | Operational charts (error-rate) need freshness; trend charts (retention) can be stale. |
| 4 | singleflight for dedup | Prevents thundering herd on cache cold start -- critical for 50k DAU. |
| 5 | Per-chart frontend queries | Enables progressive loading; `chartData` query already exists in the schema. |
| 6 | 5-minute time quantization | Enables caching at both frontend (Apollo) and backend levels. Max 5-min staleness is acceptable for 7-30 day dashboards. |
| 7 | Client-side bucket averaging | Simple, chart-type-agnostic downsampling. 500 points = ~2px/point, at visual resolution limit. |
| 8 | Keep `overviewPage` query | Unknown API consumers may depend on it. Backend optimizations benefit all callers. |

---

## Performance Budget

| Component | Current | After Optimization | Notes |
|-----------|---------|-------------------|-------|
| Backend (cache miss) | 16-40s (sum of 8 queries) | 2-5s (max single query) | errgroup parallelization |
| Backend (cache hit) | N/A | ~1ms | In-memory lookup |
| Backend query data volume | 10,000+ rows per chart | 100-1,000 rows per chart | LIMIT + time bucket coarsening |
| Network transfer | Multi-MB response | 100-500KB per chart | Reduced result sets |
| Frontend rendering | 2-5s (10k+ SVG elements) | 50-100ms (500 elements) | Downsampling |
| Apollo cache | Never hits | Hits within 5-min window | Time quantization fix |
| **Total (cache miss, P95)** | **8-10s** | **<2s** | **Target met** |
| **Total (cache hit)** | **N/A** | **<500ms** | **Bonus improvement** |

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Concurrency bugs in parallelized GetOverviewPage | Medium | High | errgroup for structure; `go test -race`; index-based result collection (no shared state) |
| ClickHouse connection pool exhaustion under load | Medium | High | Explicit pool config (12 max); singleflight reduces concurrent queries; cache reduces total queries |
| Cache thundering herd on cold start | High | Medium | singleflight deduplicates concurrent cache misses for same key |
| Downsampling hides operational signals | Medium | Medium | Per-chart max points (error-rate gets 1000 points, others get 500); backend coarsening already limits granularity |
| Cache fills up and LRU eviction is buggy | Medium | Medium | Thorough `cache_test.go`; cache size 1000 supports ~125 tenants; fallback to hashicorp/golang-lru if needed |
| 8 HTTP requests overwhelm browser/network | Low | Low | Browsers support 6-8 concurrent connections; total payload smaller than monolithic response |

---

## Backward Compatibility

| Area | Compatible? | Details |
|------|------------|---------|
| GraphQL schema | Yes | No changes. `overviewPage`, `chartData`, `summary` queries all preserved. |
| API response format | Yes | Same types, same fields. Only internal execution changes. |
| Frontend behavior | Yes (improved) | Charts render progressively instead of all-at-once. Same visual output. |
| Error handling | Yes | Skip-and-continue pattern preserved. Failed charts show error state per chart instead of failing the whole page. |

---

## Rollback Strategy

Each phase is independently rollbackable:

1. **Backend parallelization:** Revert `GetOverviewPage` to sequential loop. Zero API impact.
2. **Caching:** Remove cache from service constructor. Falls back to direct ClickHouse queries.
3. **Query changes:** Revert SQL strings in `queries.go`. No schema changes to undo.
4. **Frontend:** Revert `Overview.tsx` to use monolithic `OVERVIEW_PAGE_QUERY`. The query still works.

---

## Execution Plan

| Sprint | Focus | Duration | Checkpoint |
|--------|-------|----------|------------|
| 1 | Backend parallelization + bug fix | 3-4 days | Backend time drops from 25s to 5s |
| 2 | Caching layer (per-key TTL, singleflight) | 2-3 days | Cache hits return in <100ms |
| 3 | Query optimization (limits, coarser time buckets) | 1-2 days | First-load time drops to 2-3s |
| 4 | Frontend (progressive loading, downsampling) | 3-4 days | First chart visible in <1s, all in <2s |
| 5 | Testing + validation | 1-2 days | P95 < 2s confirmed, race detector passes |

**Total estimated effort:** 10-15 engineering days.

---

## Success Criteria

Per the agreed acceptance criteria from Stage 3:

- [ ] P95 page load time under 2 seconds for overview page
- [ ] All 8 charts render with correct data
- [ ] Retention cohort chart works (bug fixed)
- [ ] GraphQL API contract unchanged
- [ ] Above-fold charts appear before below-fold
- [ ] Apollo cache produces hits on subsequent loads within 5-minute window
- [ ] ClickHouse query load reduced vs. baseline (measured via cache hit rate)
- [ ] No race conditions (`go test -race` passes)

---

## Open Questions for Reviewers

1. **Connection pool sizing:** Is 12 max connections appropriate for the production ClickHouse topology? The docker-compose shows a single node, but production may differ.

2. **Downsampling threshold:** Is 500 points acceptable for line charts? Product/design input on visual fidelity vs. performance trade-off would be valuable.

3. **Cache TTL for error-rate chart:** Currently set to 1 minute. Is this fresh enough for operational monitoring, or should it be shorter (30s)?

4. **Test infrastructure:** There are zero tests today. Should we set up a test framework (e.g., testify, vitest) as a prerequisite, or is stdlib testing sufficient?

5. **Monitoring:** Should we add cache hit/miss metrics (e.g., Prometheus counters) as part of this work, or as a follow-up?

---

## Files Affected (Complete List)

### Modified (9 files)
| File | Change Type |
|------|------------|
| `internal/analytics/service.go` | Major rewrite (parallelization + cache integration) |
| `internal/analytics/models.go` | Minor (add DuplicateParams field) |
| `internal/clickhouse/client.go` | Medium (pool config + new method) |
| `internal/clickhouse/queries.go` | Medium (fix bug, add limits, coarsen time) |
| `internal/cache/cache.go` | Major rewrite (per-key TTL, LRU) |
| `cmd/analytics-api/main.go` | Minor (wire cache) |
| `apps/dashboard/src/pages/Overview.tsx` | Major rewrite (progressive loading) |
| `apps/dashboard/src/components/Chart.tsx` | Medium (downsampling + useMemo) |
| `apps/dashboard/src/api/analytics.ts` | Minor (add SUMMARY_QUERY) |
| `go.mod` | Minor (add golang.org/x/sync) |

### Created (4 files)
| File | Purpose |
|------|---------|
| `apps/dashboard/src/components/ChartWithLoading.tsx` | Per-chart data fetching + lazy loading wrapper |
| `internal/analytics/service_test.go` | Backend parallelization + caching tests |
| `internal/clickhouse/queries_test.go` | Query parameter validation tests |
| `internal/cache/cache_test.go` | Cache implementation tests |

### Not Changed (explicitly preserved)
| File | Why |
|------|-----|
| `internal/analytics/schema.graphql` | API contract unchanged |
| `internal/analytics/resolver.go` | Thin delegation layer, no changes needed |
| `migrations/001_analytics_tables.sql` | ClickHouse schema changes out of scope |
