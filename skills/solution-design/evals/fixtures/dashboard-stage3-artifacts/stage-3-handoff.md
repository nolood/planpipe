# Stage 3 Handoff — Task Synthesis Complete

> Status: draft — pending user confirmation in Stage 4

## Task Summary
Optimize the analytics dashboard overview page from 8-10 second load time to under 2 seconds. The system is a Go backend (gqlgen GraphQL) querying ClickHouse, serving a React frontend (Apollo + Recharts). Multiple compounding bottlenecks exist across backend (sequential queries, no caching), and frontend (monolithic query, no downsampling, cache-busting bug).

## Classification
- **Type:** refactor (performance optimization)
- **Complexity:** high
- **Primary risk area:** technical — concurrency and caching in untested code
- **Solution direction:** systematic — address all three layers (backend, cache, frontend) in coordinated phases

## Synthesized Goal
Reduce the analytics dashboard overview page load time from 8-10 seconds to under 2 seconds (P95) while preserving the GraphQL API contract and chart data accuracy.

## Synthesized Problem Statement
Degraded dashboard performance is eroding user trust and engagement. Product managers and analysts rely on this page daily; slow loads reduce adoption and data-driven decision making. ~50,000 DAU are affected.

## Synthesized Scope

### Included
- Backend: parallelize 8 sequential ClickHouse queries using errgroup
- Backend: integrate in-memory cache with per-chart-type TTLs
- Backend: add singleflight for cache miss deduplication
- Backend: fix retention cohort query bug (5 params, 3 passed)
- Frontend: split monolithic GraphQL query into per-chart queries with progressive loading
- Frontend: fix Apollo cache-busting bug (quantize time range)
- Frontend: add data downsampling before chart rendering (cap at 500 points)
- Frontend: lazy-load below-fold charts

### Excluded
- ClickHouse schema changes (materialized views, secondary indexes)
- Switching chart rendering library (Recharts stays)
- GraphQL API contract changes (existing fields/queries unchanged)
- Real-time streaming / WebSocket updates
- Per-chart time range configuration

## Key Scenarios for Planning

### Primary Scenario
1. User opens dashboard overview page
2. Frontend fires per-chart GraphQL queries (not monolithic)
3. Above-fold charts load first, below-fold lazy-loaded on scroll
4. Backend checks in-memory cache per chart query
5. Cache hit → return immediately; cache miss → singleflight dedup → parallel ClickHouse queries
6. Results cached with chart-type-specific TTLs
7. Data downsampled to ≤500 points before frontend rendering
8. User sees first charts in <1s, all charts in <2s (P95)

### Mandatory Edge Cases
- Cache cold start (fresh deploy) → all queries hit ClickHouse, singleflight prevents thundering herd
- Large tenant (100M+ events) → query limits prevent unbounded result sets
- Retention cohort chart → bug fix makes it functional for the first time
- ClickHouse unavailable → skip-and-continue pattern preserved, failed charts show error state

## System Map for Planning

### Modules to Change
| Module | Path | What Changes | Scope |
|--------|------|-------------|-------|
| Analytics Service | `internal/analytics/` | Parallelize queries, integrate cache, singleflight | large |
| ClickHouse Client | `internal/clickhouse/` | Fix bug, add result limits, coarsen time buckets | medium |
| Cache Utility | `internal/cache/` | Wire into service, configure per-chart TTLs | small |
| API Entry Point | `cmd/analytics-api/` | Cache initialization, inject into service | small |
| Dashboard Frontend | `apps/dashboard/` | Per-chart queries, progressive loading, downsampling | large |

### Key Change Points
| Location | What Changes | Why |
|----------|-------------|-----|
| `internal/analytics/service.go:GetOverviewPage` | errgroup parallelization | Sequential bottleneck elimination |
| `internal/analytics/service.go:loadChart` | Cache integration + singleflight | Reduce ClickHouse load |
| `internal/clickhouse/queries.go:67-85` | Fix retention cohort param count | Bug fix |
| `internal/clickhouse/queries.go` | Add LIMIT clauses, coarser time buckets | Reduce data volume |
| `cmd/analytics-api/main.go` | Cache init + injection | Service dependency setup |
| `apps/dashboard/src/pages/Overview.tsx` | Per-chart queries + progressive loading | Frontend parallelization |
| `apps/dashboard/src/pages/Overview.tsx:26-27` | Quantize time range params | Fix Apollo cache busting |
| `apps/dashboard/src/components/Chart.tsx:50-55` | Downsampling before render | Reduce SVG elements |
| `apps/dashboard/src/components/ChartGrid.tsx` | Lazy loading for below-fold | Reduce initial render work |

### Critical Dependencies
- **ClickHouse cluster:** Connection pool sizing for 8x concurrent load per request
- **gqlgen framework:** Resolver architecture for per-chart query support
- **Apollo Client:** Query splitting and caching behavior
- **Recharts:** SVG rendering perf tied to data point count

## Constraints for Planning
- GraphQL API backward compatible — no breaking changes
- 2-second target is wall-clock P95 including network + backend + frontend
- Zero test coverage — changes need new tests
- ClickHouse schema changes out of scope
- Skip-and-continue error handling preserved

## Risks to Mitigate

| Risk | Likelihood | Impact | Mitigation Direction |
|------|-----------|--------|---------------------|
| Parallel queries overwhelm ClickHouse connection pool | medium | high | Explicit pool size config, concurrency semaphore, load test |
| Cache thundering herd under 50k DAU | high | medium | singleflight for in-flight dedup |
| Concurrency bugs in untested code | medium | high | errgroup for structure, race detector, new tests |
| Downsampling hides operational signals | medium | medium | Configurable per chart type, finer granularity for operational charts |
| Cache eviction issues under load | medium | medium | LRU eviction, size bounds, hit/miss monitoring |

## Product Requirements for Planning
- **Primary scenario:** Charts load progressively in <2s P95
- **Success signals:** P95 load time <2s, time to first chart visible, dashboard engagement rate, ClickHouse load reduction
- **Minimum viable outcome:** Backend parallelization (without it, 2s target is mathematically impossible for large tenants)
- **Backward compatibility:** GraphQL API unchanged, chart visual output unchanged

## Solution Direction
Systematic — address all three layers in coordinated phases. Full-stack optimization rather than backend-only, because backend parallelization alone likely achieves ~2s but not reliably under P95 for all tenants. Pending user confirmation in Stage 4.

## Assumptions (pending confirmation)
- In-memory cache is sufficient (no Redis/external cache needed for <50k DAU)
- ClickHouse can handle 8 concurrent queries per request with proper pool sizing
- 500-point downsampling preserves visual accuracy for trend charts

## Deferred Items
- ClickHouse materialized views (future optimization if needed)
- Per-chart configurable time ranges
- Real-time streaming updates

## Acceptance Criteria
- P95 page load time under 2 seconds for overview page
- All 8 charts render with correct data
- Retention cohort chart works (bug fixed)
- GraphQL API contract unchanged
- Above-fold charts appear before below-fold
- Apollo cache produces hits on subsequent loads
- ClickHouse query load reduced vs. baseline
- No race conditions (go test -race passes)

## Detailed References
- `analysis.md` — synthesized task analysis
- `agreement-package.md` — agreement blocks for Stage 4's combined review
- `agreed-task-model.md` — draft task model (pending user confirmation)
- `product-analysis.md` — detailed product/business analysis (Stage 2)
- `system-analysis.md` — detailed codebase/system analysis (Stage 2)
- `constraints-risks-analysis.md` — detailed constraints/risks analysis (Stage 2)
