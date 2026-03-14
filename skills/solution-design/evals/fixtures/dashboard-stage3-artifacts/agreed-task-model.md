# Agreed Task Model

> Status: draft — pending user confirmation in Stage 4
> Agreed on: [pending confirmation in Stage 4]
> Based on: Stage 2 analyses + synthesis critique

## Task Goal
Reduce the analytics dashboard overview page load time from 8-10 seconds to under 2 seconds (P95) while preserving the GraphQL API contract and chart data accuracy.

## Problem Statement
Degraded dashboard performance is eroding user trust and engagement. ~50,000 DAU rely on this page daily. Slow loads reduce adoption of data-driven decision making.

## Scope

### Included
- Backend parallelization (errgroup)
- In-memory caching with per-chart-type TTLs
- singleflight for cache miss dedup
- Retention cohort bug fix
- Frontend per-chart queries with progressive loading
- Apollo cache-busting fix
- Data downsampling (≤500 points)
- Lazy loading for below-fold charts

### Excluded
- ClickHouse schema changes
- Chart library switch
- GraphQL breaking changes
- Real-time streaming
- Per-chart time range config

## Key Scenarios

### Primary Scenario
1. User opens overview page
2. Per-chart queries fire (not monolithic)
3. Above-fold charts load first
4. Backend: cache check → singleflight → parallel ClickHouse
5. Data downsampled → charts render
6. All charts visible in <2s P95

### Mandatory Edge Cases
- Cache cold start → singleflight prevents thundering herd
- Large tenant → query limits prevent unbounded results
- Retention cohort → bug fix makes it work
- ClickHouse down → skip-and-continue preserved

### Explicitly Deferred
- Materialized views
- Per-chart time ranges (UX decision)
- Real-time streaming

## System Scope

### Affected Modules
| Module | Path | Role | Scope |
|--------|------|------|-------|
| Analytics | `internal/analytics/` | Parallel queries, caching | large |
| ClickHouse | `internal/clickhouse/` | Bug fix, query limits | medium |
| Cache | `internal/cache/` | Wire into service | small |
| API | `cmd/analytics-api/` | Cache init | small |
| Frontend | `apps/dashboard/` | Per-chart queries, downsampling | large |

### Key Change Points
| Location | What Changes | Why |
|----------|-------------|-----|
| `service.go:GetOverviewPage` | errgroup parallel | Sequential bottleneck |
| `service.go:loadChart` | Cache + singleflight | Reduce ClickHouse load |
| `queries.go:67-85` | Fix param count | Bug fix |
| `queries.go` | Add LIMIT, coarser buckets | Reduce data volume |
| `Overview.tsx` | Per-chart queries | Frontend parallel |
| `Overview.tsx:26-27` | Quantize time range | Fix cache busting |
| `Chart.tsx:50-55` | Downsampling | Reduce SVG elements |
| `ChartGrid.tsx` | Lazy loading | Reduce initial render |

### Dependencies
- ClickHouse: connection pool for 8x concurrency
- gqlgen: resolver architecture
- Apollo Client: query splitting
- Recharts: SVG perf

## Confirmed Constraints
- GraphQL API backward compatible — confirmed
- 2s P95 target — confirmed
- Zero test coverage — new tests needed — confirmed
- ClickHouse schema out of scope — confirmed
- Skip-and-continue pattern preserved — confirmed

## Risks to Mitigate

| Risk | Likelihood | Impact | Direction |
|------|-----------|--------|-----------|
| Connection pool overwhelm | medium | high | Explicit sizing, semaphore |
| Thundering herd | high | medium | singleflight |
| Concurrency bugs | medium | high | errgroup, race detector |
| Downsampling hides signals | medium | medium | Per-chart-type config |
| Cache eviction failure | medium | medium | LRU, size bounds |

## Solution Direction
Systematic full-stack optimization across backend (parallelization + caching), and frontend (progressive loading + downsampling).

## Accepted Assumptions
- In-memory cache sufficient for <50k DAU
- ClickHouse handles 8 concurrent queries with pool sizing
- 500-point downsampling preserves visual accuracy

## Deferred Decisions
- Materialized views — future if needed
- Per-chart time ranges — user rejected

## User Corrections Log
- **Scope:** Backend-only → full-stack (user's choice)
- **Per-chart time range:** Rejected for UX consistency
- **Target:** Confirmed P95
- **Cache:** In-memory over Redis

## Acceptance Criteria
- P95 load <2s
- All 8 charts correct data
- Retention cohort works
- GraphQL contract unchanged
- Progressive loading works
- Apollo cache hits
- ClickHouse load reduced
- Race detector passes
