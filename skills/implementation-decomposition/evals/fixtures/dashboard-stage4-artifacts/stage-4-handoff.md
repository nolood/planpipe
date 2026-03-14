# Stage 4 Handoff — Solution Design Complete

## Task Summary
Reduce the analytics dashboard overview page load time from 8-10 seconds to under 2 seconds (P95) through full-stack optimization: backend parallelization with errgroup, in-memory caching with per-chart TTLs and singleflight, retention cohort bug fix, and frontend per-chart queries with progressive loading and client-side downsampling.

## Classification
- **Type:** refactor (performance)
- **Complexity:** high
- **Change scope:** 8 files modified, 3 new, 0 deleted across 5 modules
- **Solution direction:** systematic

## Implementation Approach
Three-layer optimization: (1) Backend — parallelize sequential ClickHouse queries using errgroup, add in-memory cache with per-chart TTLs and singleflight for deduplication; (2) Data — fix retention cohort calculation bug at `queries.go:67-85`, add row limits and coarser time buckets for large datasets; (3) Frontend — split monolithic GraphQL query into per-chart queries, progressive loading with skeleton states, client-side downsampling to 500 points, lazy loading for below-fold charts.

## Solution Overview
The current overview page fires a single large GraphQL query that sequentially fetches all chart data from ClickHouse. The optimization breaks this into per-chart GraphQL queries on the frontend, each backed by parallelized ClickHouse queries with caching on the backend. Charts render progressively as their data arrives. Large datasets are downsampled client-side to 500 points. A bug in retention cohort calculation (`queries.go:67-85`) that causes unnecessary full-table scans is also fixed.

## Change Summary

### Modules Affected
| Module | Path | Changes | Scope |
|--------|------|---------|-------|
| Analytics Service | `internal/analytics/` | Parallelize queries (errgroup), add caching (singleflight + in-memory), restructure service methods | large |
| ClickHouse Client | `internal/clickhouse/` | Fix retention bug, add row limits, coarser time buckets | medium |
| Cache | `internal/cache/` | New in-memory cache with per-chart TTL configuration | medium |
| API | `cmd/analytics-api/` | Initialize cache, update service wiring | small |
| Frontend | `apps/dashboard/` | Per-chart queries, progressive loading, downsampling, lazy loading | large |

### Key Change Points
| Location | What Changes | Why |
|----------|-------------|-----|
| `internal/analytics/service.go` | Parallelize `LoadOverviewData` with errgroup; wrap each chart loader in cache | Sequential queries are the main bottleneck |
| `internal/analytics/service.go:loadChart` | Add cache check + singleflight wrapper | Prevent redundant ClickHouse queries |
| `internal/clickhouse/queries.go:67-85` | Fix retention cohort WHERE clause | Bug causes full-table scan instead of filtered query |
| `internal/clickhouse/queries.go` | Add LIMIT clause and coarser GROUP BY for large tenants | Prevent unbounded result sets |
| `internal/cache/chart_cache.go` | New file: in-memory cache with per-chart TTL | Cache layer for chart data |
| `apps/dashboard/src/components/Overview.tsx` | Split single query into per-chart queries | Enable progressive rendering |
| `apps/dashboard/src/components/Chart.tsx` | Add client-side downsampling (LTTB, 500 points) | Reduce rendering cost for large datasets |
| `apps/dashboard/src/components/ChartGrid.tsx` | Add Intersection Observer lazy loading | Don't load below-fold charts until visible |

### New Entities
| Entity | Type | Location | Purpose |
|--------|------|----------|---------|
| `ChartCache` | service | `internal/cache/chart_cache.go` | In-memory cache with per-chart TTL and singleflight |
| `chartCacheConfig` | config | `internal/cache/chart_cache.go` | Per-chart TTL configuration |
| `useChartQuery` | hook | `apps/dashboard/src/hooks/useChartQuery.ts` | Per-chart GraphQL query hook with loading state |

### Interface Changes
| Interface | Change | Consumers Affected |
|-----------|--------|-------------------|
| `AnalyticsService.LoadOverviewData` | Returns `chan ChartResult` instead of `OverviewData` struct | API handler, tests |
| `Overview.tsx` props | Removes `data` prop, manages own data fetching per chart | Parent page component |

## Implementation Sequence
| Step | What | Validates |
|------|------|-----------|
| 1 | Fix retention cohort bug (`queries.go:67-85`) | Query returns correct filtered results |
| 2 | Add row limits and coarser time buckets to ClickHouse queries | Large tenant queries complete within timeout |
| 3 | Create ChartCache with per-chart TTL and singleflight | Cache stores/retrieves chart data correctly |
| 4 | Parallelize service with errgroup, integrate cache | Backend responds in <500ms for cached data |
| 5 | Split Overview.tsx into per-chart queries | Each chart loads independently |
| 6 | Add progressive loading with skeleton states | Charts render as data arrives |
| 7 | Add client-side downsampling (LTTB, 500 points) | Large datasets render without lag |
| 8 | Add lazy loading for below-fold charts | Only visible charts fetch data |

## Key Technical Decisions
| Decision | Reasoning | User Approved |
|----------|-----------|---------------|
| errgroup for parallelization | Standard Go concurrency pattern; bounded goroutines; error propagation | not required |
| In-memory cache (not Redis) | User confirmed; sufficient for single-instance; simpler deployment | yes |
| singleflight for cache stampede | Prevents thundering herd on cache miss; stdlib package | not required |
| LTTB downsampling at 500 points | User confirmed threshold; preserves visual shape; 500 is sweet spot for chart width | yes |
| Per-chart GraphQL queries (not subscription) | Simpler than subscriptions; progressive loading via independent queries | not required |

## Constraints Respected
- GraphQL schema backward compatible (additive changes only)
- P95 < 2s target for overview page
- Zero existing test coverage (add tests for new code)
- Skip-and-continue pattern preserved (failing chart doesn't block others)

## Risks and Mitigations
| Risk | Mitigation | Severity |
|------|------------|----------|
| ClickHouse connection pool exhaustion from parallel queries | Limit errgroup concurrency to 8; monitor connection pool | medium |
| Thundering herd on cache cold start | singleflight deduplicates concurrent requests for same chart | medium |
| Downsampling hides data signals | 500-point threshold preserves shape; user can zoom for full data | low |
| Concurrent errgroup introduces race conditions | Each goroutine gets own ClickHouse connection; no shared mutable state | medium |
| Cache eviction causing stale data | Short TTLs (30s-5min per chart type); manual invalidation endpoint | low |

## Backward Compatibility
GraphQL schema is additive — existing queries continue to work. The monolithic overview query still functions but is no longer used by the frontend. No data schema changes.

## User Decisions Log
- Full-stack optimization: User chose both backend and frontend changes (not backend-only)
- In-memory cache: User confirmed over Redis
- P95 target: User confirmed 2 seconds
- Downsampling threshold: User confirmed 500 points

## Acceptance Criteria
- Overview page loads in <2s (P95) for typical tenants
- Chart data is numerically accurate (no rounding errors from downsampling beyond acceptable tolerance)
- Retention cohort bug is fixed (correct WHERE clause at `queries.go:67-85`)
- Cache reduces ClickHouse query load by >50% during normal usage
- Progressive loading shows charts as they become available
- Below-fold charts only load when scrolled into view
- Existing GraphQL queries continue to work unchanged

## Detailed References
- `implementation-design.md` — complete implementation design
- `change-map.md` — detailed file-level change map
- `design-decisions.md` — full decision journal
- `agreed-task-model.md` — agreed task model (Stage 3)
