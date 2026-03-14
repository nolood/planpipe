# Change Map

> Task: Reduce analytics dashboard load time from 8-10s to <2s (P95)
> Total files affected: 8 modified, 5 new, 0 deleted

## Files to Modify

| File | Module | Change Description | Scope | Dependencies |
|------|--------|-------------------|-------|-------------|
| `internal/analytics/service.go` | Analytics | Refactor LoadOverviewData to parallel errgroup; integrate ChartCache + singleflight | large | `chart_cache.go` must exist |
| `internal/analytics/service_test.go` | Analytics | Add tests for parallel execution, caching, singleflight, error handling | medium | `service.go` changes |
| `internal/clickhouse/queries.go` | ClickHouse | Fix retention cohort WHERE (lines 67-85); add LIMIT; coarser GROUP BY for >90d | medium | none |
| `internal/clickhouse/queries_test.go` | ClickHouse | Add tests for fixed retention, limits, coarser buckets | small | `queries.go` changes |
| `cmd/analytics-api/main.go` | API | Initialize ChartCache; pass to AnalyticsService | small | `chart_cache.go` must exist |
| `apps/dashboard/src/components/Overview.tsx` | Frontend | Remove monolithic query; render per-chart via useChartQuery hook | medium | `useChartQuery.ts` must exist |
| `apps/dashboard/src/components/Chart.tsx` | Frontend | Add LTTB downsampling for data > 500 points | small | `downsample.ts` must exist |
| `apps/dashboard/src/components/ChartGrid.tsx` | Frontend | Add Intersection Observer lazy loading for below-fold charts | small | none |

## Files to Create

| File | Module | Purpose | Template/Pattern |
|------|--------|---------|-----------------|
| `internal/cache/chart_cache.go` | Cache | In-memory cache: Get/Set/Invalidate, per-chart TTL, singleflight, background cleanup | Standard Go service pattern |
| `internal/cache/chart_cache_test.go` | Cache | Unit tests for cache operations, TTL, singleflight, concurrency | Standard Go test pattern |
| `apps/dashboard/src/hooks/useChartQuery.ts` | Frontend | Custom React hook for per-chart GraphQL queries with loading/error state | Existing hook patterns in `src/hooks/` |
| `apps/dashboard/src/utils/downsample.ts` | Frontend | LTTB downsampling algorithm implementation | Standalone utility module |
| `apps/dashboard/src/utils/downsample.test.ts` | Frontend | Tests for LTTB correctness and edge cases | Existing test patterns |

## Files to Delete

No files to delete.

## Interfaces Changed

| Interface | Location | Current Signature | New Signature | Consumers |
|-----------|----------|------------------|---------------|-----------|
| `AnalyticsService.LoadOverviewData` | `internal/analytics/service.go` | `LoadOverviewData(ctx) (OverviewData, error)` | `LoadChartData(ctx, chartID) (ChartData, error)` + parallel wrapper | API handler, tests |
| `Overview` component | `apps/dashboard/src/components/Overview.tsx` | Receives `data: OverviewData` prop | Manages own data via `useChartQuery` per chart | Parent page |
| `Chart` component | `apps/dashboard/src/components/Chart.tsx` | Renders raw data points | Accepts optional `maxPoints` prop (default 500), applies LTTB | Overview/ChartGrid |

## Data / Schema Changes

No data/schema changes.

## Configuration Changes

| What | Location | Description |
|------|----------|-------------|
| Chart TTL configuration | `internal/cache/chart_cache.go` | Per-chart-type TTLs (e.g., revenue: 5min, active_users: 1min, retention: 10min) |
| errgroup concurrency | `internal/analytics/service.go` | Max concurrent ClickHouse queries (default: 8) |

## Change Dependency Order

```
queries.go bug fix (independent)
queries.go limits (independent)
chart_cache.go → service.go → main.go
downsample.ts (independent)
useChartQuery.ts → Overview.tsx → Chart.tsx + ChartGrid.tsx
```
