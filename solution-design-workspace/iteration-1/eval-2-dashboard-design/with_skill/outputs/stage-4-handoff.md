# Stage 4 Handoff — Solution Design Complete

## Task Summary
Optimize the analytics dashboard overview page from 8-10 second load time to under 2 seconds (P95). The system is a Go backend (gqlgen GraphQL) querying ClickHouse, serving a React frontend (Apollo Client + Recharts). The optimization is a full-stack systematic change: backend query parallelization with in-memory caching and singleflight deduplication, plus frontend per-chart progressive loading with data downsampling and Apollo cache fix.

## Classification
- **Type:** refactor (performance optimization)
- **Complexity:** high
- **Change scope:** 7 files modified, 5 new, 0 deleted across 5 modules
- **Solution direction:** systematic — all three layers (backend, cache, frontend)

## Implementation Approach
The design parallelizes 8 sequential ClickHouse queries using errgroup, adds an in-memory cache layer with per-chart-type TTLs (1m-15m) and singleflight for cache-miss deduplication, and restructures the frontend from a monolithic GraphQL query to 9 independent per-chart/summary queries with progressive rendering. This approach was chosen over backend-only optimization because the P95 2s target requires both fast backend responses AND fast frontend rendering — backend parallelization alone achieves 2-5s (max of queries) but frontend rendering of thousands of SVG elements adds 2-5s on top.

## Solution Overview
When a user opens the dashboard, the frontend fires 9 parallel GraphQL requests with quantized time ranges (5-minute boundaries for Apollo cache stability). Each chart loads independently — above-fold charts immediately, below-fold via IntersectionObserver lazy loading. On the backend, each chart request checks an in-memory cache (keyed by tenant+chart+timeRange). Cache hits return instantly. Cache misses go through singleflight deduplication (preventing thundering herd under 50k DAU), then execute the ClickHouse query in parallel with other chart queries via errgroup. Results are cached with chart-type-specific TTLs. Data is downsampled to 500 points via LTTB on the frontend before Recharts rendering. The retention cohort query bug (5 SQL placeholders, 3 params passed) is fixed as part of the query layer changes.

## Change Summary

### Modules Affected
| Module | Path | Changes | Scope |
|--------|------|---------|-------|
| Analytics Service | `internal/analytics/` | Errgroup parallelization in GetOverviewPage, cache+singleflight integration in loadChart, per-chart TTL config | large |
| ClickHouse Client | `internal/clickhouse/` | Variadic params in ExecuteQuery, retention cohort bug fix, error_rate granularity coarsened, top_events LIMIT reduced | medium |
| Cache Utility | `internal/cache/` | Per-key TTL via SetWithTTL, LRU eviction replacing naive eviction | medium |
| API Entry Point | `cmd/analytics-api/` | Cache initialization, service wiring, ClickHouse pool config | small |
| Dashboard Frontend | `apps/dashboard/` | Per-chart queries, progressive loading, LTTB downsampling, time quantization, lazy loading, skeleton states | large |

### Key Change Points
| Location | What Changes | Why |
|----------|-------------|-----|
| `internal/analytics/service.go:GetOverviewPage` | Sequential for-loop replaced with errgroup parallel execution | Eliminates the primary sequential bottleneck (sum of queries -> max of queries) |
| `internal/analytics/service.go:loadChart` | Wrapped with cache check + singleflight + cache store | Reduces ClickHouse load, prevents thundering herd |
| `internal/analytics/service.go:Service` struct | Adds `cache *cache.Cache` and `sfGroup *singleflight.Group` fields | New dependencies for caching and deduplication |
| `internal/clickhouse/client.go:ExecuteQuery` | Fixed params changed to variadic `args ...any` | Supports queries with varying parameter counts (fixes retention cohort bug) |
| `internal/clickhouse/queries.go:user_retention_cohort` | 5 params now correctly passed | Bug fix: query always failed due to parameter count mismatch |
| `internal/clickhouse/queries.go:error_rate_over_time` | `toStartOfMinute` changed to `toStartOfFiveMinute` | Reduces rows from 1440 to 288 for 24h range |
| `internal/clickhouse/queries.go:top_events_by_count` | LIMIT reduced from 10000 to 500 | Caps result set to a renderable size |
| `internal/cache/cache.go` | Added SetWithTTL, LRU eviction, lastAccessed tracking | Per-chart TTLs, fixes broken eviction that silently drops entries |
| `cmd/analytics-api/main.go` | Cache creation and injection into service | Wires the cache into the application |
| `apps/dashboard/src/pages/Overview.tsx` | Monolithic query replaced with per-chart hooks + lazy loading | Progressive loading, independent chart states |
| `apps/dashboard/src/components/Chart.tsx:50` | Downsample call added before Recharts transformation | Caps SVG elements at 500 for rendering performance |
| `apps/dashboard/src/components/ChartGrid.tsx` | LazyChart wrapper for below-fold charts | Defers rendering and data fetching for non-visible charts |

### New Entities
| Entity | Type | Location | Purpose |
|--------|------|----------|---------|
| `SetWithTTL` | method | `internal/cache/cache.go` | Per-key TTL support for chart-type-specific cache durations |
| `chartCacheTTLs` | var | `internal/analytics/service.go` | Maps chart IDs to their cache TTL durations |
| `downsample` | function | `apps/dashboard/src/utils/downsample.ts` | LTTB downsampling to cap data points at 500 |
| `useChartData` | hook | `apps/dashboard/src/hooks/useChartData.ts` | Per-chart data loading with quantized time range |
| `quantizeTime` | function | `apps/dashboard/src/utils/quantizeTime.ts` | Rounds timestamps to 5-minute boundaries for stable cache keys |
| `ChartSkeleton` | component | `apps/dashboard/src/components/ChartSkeleton.tsx` | Loading placeholder for individual chart slots |
| `LazyChart` | component | `apps/dashboard/src/components/LazyChart.tsx` | IntersectionObserver wrapper for deferred chart loading |

### Interface Changes
| Interface | Change | Consumers Affected |
|-----------|--------|-------------------|
| `analytics.NewService` | Adds `cache *cache.Cache` parameter | `cmd/analytics-api/main.go` (only consumer) |
| `clickhouse.Client.ExecuteQuery` | Changes from `QueryParams` struct to variadic `args ...any` | `internal/analytics/service.go:loadChart` (only consumer) |

## Implementation Sequence
| Step | What | Validates |
|------|------|-----------|
| 1 | Fix retention cohort bug + variadic ExecuteQuery (`client.go`, `queries.go`) | Retention chart query executes without parameter error |
| 2 | Enhance cache: SetWithTTL + LRU eviction (`cache.go`) | Cache stores/retrieves with per-key TTL; LRU eviction works |
| 3 | Errgroup + singleflight + cache in service (`service.go`) | Parallel execution, cache hit/miss, singleflight dedup, skip-and-continue preserved |
| 4 | Wire cache in main.go, configure ClickHouse pool | Server starts with caching active; end-to-end backend optimization working |
| 5 | Optimize ClickHouse queries: error_rate granularity, top_events LIMIT (`queries.go`) | Fewer rows returned; charts display correctly |
| 6 | Create frontend utilities: downsample.ts, quantizeTime.ts | Unit tests pass for downsampling and time quantization |
| 7 | Create frontend components: ChartSkeleton, LazyChart, useChartData hook | Components render in isolation; hook fires correct queries |
| 8 | Rewrite Overview.tsx + update Chart.tsx + update ChartGrid.tsx | Progressive loading works; Apollo cache hits; downsampled rendering |

## Key Technical Decisions
| Decision | Reasoning | User Approved |
|----------|-----------|---------------|
| errgroup for parallelization | Idiomatic Go structured concurrency; provides context cancellation and error collection | yes |
| Enhance existing cache (not replace) | Less disruptive than new library; core structure is sound, only eviction/TTL needed fixing | yes |
| singleflight for cache-miss dedup | Prevents thundering herd under 50k DAU; stdlib-adjacent, purpose-built | yes |
| 5-minute time range quantization | Enables Apollo Client cache hits; balances freshness vs. cache effectiveness | yes |
| Frontend downsampling (not backend) | Preserves GraphQL API contract; unknown consumers get full data | yes |
| LTTB algorithm for downsampling | Preserves visual chart shape better than naive sampling | yes (implicit) |
| Variadic ExecuteQuery params | Fixes retention bug; supports queries with varying param counts | yes (implicit) |
| Preserve overviewPage query | Backward compatibility for unknown consumers; internal parallelization benefits them | yes |
| Per-chart-type cache TTLs | Different freshness needs: error_rate 1m, retention 15m | yes |

## Constraints Respected
- **GraphQL API backward compatible:** No schema changes. `overviewPage` and `chartData` queries both work. No fields added or removed.
- **2s P95 target:** Backend parallelization (max ~5s) + caching (near 0s on hit) + frontend progressive loading (first chart <1s) achieves the target. Cache cold start is the risk case — mitigated by singleflight and parallel execution.
- **Zero test coverage:** Design specifies tests needed for each module. Implementation must include tests.
- **ClickHouse schema unchanged:** No materialized views, no index changes. Only SQL query text modifications (granularity, limits).
- **Skip-and-continue preserved:** errgroup allows individual chart failures without failing the page.
- **Per-chart time range rejected:** All charts use the same time range from the frontend.

## Risks and Mitigations
| Risk | Mitigation | Severity |
|------|------------|----------|
| Parallel queries exhaust ClickHouse connection pool | Configure MaxOpenConns=20 in DSN; caching reduces query frequency | high |
| Race conditions in new concurrent code | Pre-allocated result slices (no concurrent append); `go test -race` | high |
| Cache eviction failure under load | LRU eviction replaces broken naive strategy; monitor hit/miss rates | medium |
| 9 frontend requests hit HTTP/1.1 connection limit | Lazy loading limits initial requests to 5; consider HTTP/2 | medium |
| Cache cold start after deploy | singleflight deduplicates; parallel execution still achieves 2-5s uncached | medium |
| Singleflight amplifies transient errors | Existing skip-and-continue handles this; monitor chart error rates | low |
| Downsampling hides data patterns | Error_rate at 288 points is below 500 cap (not downsampled); LTTB preserves peaks/valleys | low |

## Backward Compatibility
All changes are backward compatible. The GraphQL API schema is unchanged — same queries, same fields, same types. The `overviewPage` query returns the same response shape but executes faster. Cached responses may be up to 1-15 minutes stale (per chart type), which is a behavioral change from the previous always-real-time behavior. The error_rate chart now returns 5-minute granularity instead of 1-minute, and top_events returns max 500 rows instead of 10,000. No data migrations needed.

## User Decisions Log
- Full-stack optimization over backend-only: confirmed in Stage 3
- P95 target (not P50): confirmed in Stage 3
- In-memory cache over Redis: confirmed in Stage 3
- Per-chart time ranges rejected: confirmed in Stage 3
- Per-chart-type cache TTLs (1m-15m): approved in design review
- 5-minute error_rate granularity: approved in design review
- Lazy loading for below-fold charts: approved in design review
- Preserving overviewPage query for backward compatibility: approved in design review

## Acceptance Criteria
- P95 page load time under 2 seconds for overview page
- All 8 charts render with correct data
- Retention cohort chart works (bug fixed)
- GraphQL API contract unchanged
- Above-fold charts appear before below-fold
- Apollo cache produces hits on subsequent loads within 5-minute window
- ClickHouse query load reduced vs. baseline (cache hit rate > 0%)
- No race conditions (`go test -race` passes)

## Detailed References
- `implementation-design.md` — complete implementation design with data flows, entity tables, and module-by-module change details
- `change-map.md` — detailed file-level change map with dependency order
- `design-decisions.md` — full decision journal with 9 decisions, alternatives, and reasoning
- `design-review-package.md` — user review document with 4 approval points
- `agreed-task-model.md` — agreed task model from Stage 3
- `stage-3-handoff.md` — Stage 3 handoff with full context
