# Implementation Design

> Task: Reduce analytics dashboard load time from 8-10s to <2s (P95)
> Solution direction: systematic (full-stack optimization)
> Design status: finalized

## Implementation Approach

### Chosen Approach
Three-layer full-stack optimization targeting the three main bottlenecks: sequential backend queries, missing cache layer, and monolithic frontend data fetching. The backend switches from sequential to parallel ClickHouse queries using errgroup with bounded concurrency. An in-memory cache with per-chart TTLs and singleflight eliminates redundant queries. The frontend splits the monolithic overview query into per-chart queries with progressive loading, and adds client-side LTTB downsampling for large datasets.

This approach was chosen because profiling shows three independent bottlenecks: backend sequential queries (~4s), repeated identical queries (~2s redundant), and frontend rendering of large datasets (~2s). Each layer can be optimized independently, and the combined effect should bring P95 under 2 seconds.

### Alternatives Considered
- **Backend-only optimization:** Parallelize + cache but keep monolithic frontend query → Rejected because: frontend still waits for all data before rendering; user explicitly chose full-stack
- **ClickHouse materialized views:** Pre-compute dashboard aggregations → Rejected because: out of agreed scope; adds schema complexity
- **Redis cache:** External cache service → Rejected because: user confirmed in-memory is sufficient for single-instance deployment
- **GraphQL subscriptions:** Real-time chart updates via WebSocket → Rejected because: over-engineered; per-chart queries with progressive loading are simpler and sufficient

### Approach Trade-offs
In-memory cache doesn't survive process restarts (cold start scenario handled by singleflight). Downsampling to 500 points slightly reduces data fidelity (acceptable per user). The errgroup concurrency limit of 8 means very large dashboards with >8 charts won't be fully parallel (but 8 is the current maximum chart count).

## Solution Description

### Overview
When a user opens the overview page, the frontend fires independent GraphQL queries for each chart. Each query hits the analytics service, which checks the in-memory cache. On cache miss, singleflight ensures only one ClickHouse query runs per chart (deduplicating concurrent requests). The ClickHouse query itself is bounded by row limits and uses coarser time buckets for large date ranges. Results are cached with chart-specific TTLs. The frontend renders each chart as its data arrives (progressive loading) and applies LTTB downsampling to reduce rendering cost. Charts below the fold are lazy-loaded via Intersection Observer.

### Data Flow
1. **Entry:** User opens Overview page → `Overview.tsx` renders chart grid
2. **Per-chart query:** Each `Chart.tsx` fires `useChartQuery(chartId)` → independent GraphQL query
3. **Backend handler:** GraphQL resolver calls `AnalyticsService.LoadChartData(chartId)`
4. **Cache check:** `ChartCache.Get(chartId)` → hit? return cached data
5. **Singleflight:** On cache miss, `singleflight.Do(chartId, func)` → deduplicate concurrent requests
6. **ClickHouse query:** `ClickHouseClient.QueryChart(chartId, params)` → filtered, limited, bucketed query
7. **Cache store:** Result stored in `ChartCache` with chart-specific TTL
8. **Response:** Data returned to frontend via GraphQL
9. **Downsampling:** `Chart.tsx` applies LTTB to reduce to ≤500 points if needed
10. **Render:** Chart renders with data; skeleton state removed

### New Entities

| Entity | Type | Location | Purpose |
|--------|------|----------|---------|
| `ChartCache` | service | `internal/cache/chart_cache.go` | In-memory cache with per-chart TTL, Get/Set/Invalidate |
| `chartCacheConfig` | config | `internal/cache/chart_cache.go` | TTL configuration per chart type (revenue: 5min, active_users: 1min, etc.) |
| `useChartQuery` | React hook | `apps/dashboard/src/hooks/useChartQuery.ts` | Per-chart data fetching with loading/error states |
| `downsample` | utility | `apps/dashboard/src/utils/downsample.ts` | LTTB downsampling algorithm implementation |

### Modified Entities

| Entity | Location | Current Behavior | New Behavior | Breaking? |
|--------|----------|-----------------|-------------|-----------|
| `AnalyticsService.LoadOverviewData` | `internal/analytics/service.go` | Sequentially queries all charts, returns complete struct | Parallelizes with errgroup, wraps in cache + singleflight | yes (return type changes) |
| `ClickHouseClient.QueryRetentionCohort` | `internal/clickhouse/queries.go:67-85` | Missing WHERE clause causes full-table scan | Correct WHERE clause with tenant_id and date range filter | no (bug fix) |
| `ClickHouseClient` query methods | `internal/clickhouse/queries.go` | Unbounded result sets | LIMIT clause, coarser GROUP BY for date ranges > 90 days | no |
| `Overview.tsx` | `apps/dashboard/src/components/Overview.tsx` | Single GraphQL query for all charts, renders all at once | Per-chart queries via useChartQuery, progressive rendering | yes (internal) |
| `Chart.tsx` | `apps/dashboard/src/components/Chart.tsx` | Renders raw data directly | Applies LTTB downsampling if data > 500 points | no |
| `ChartGrid.tsx` | `apps/dashboard/src/components/ChartGrid.tsx` | All charts rendered immediately | Intersection Observer lazy loading for below-fold charts | no |

## Change Details

### Module: Analytics Service
**Path:** `internal/analytics/`
**Role in changes:** Core backend optimization — parallelization, caching, singleflight integration

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `service.go` | modify | Refactor `LoadOverviewData` to use errgroup for parallel chart loading; integrate ChartCache for each chart loader; add singleflight wrapper | large |
| `service_test.go` | modify | Add tests for parallel execution, cache hit/miss paths, singleflight behavior | medium |

**Interfaces affected:**
- `LoadOverviewData` return type changes from `OverviewData` to per-chart results (or channel-based pattern)
- New method: `LoadChartData(ctx, chartID) (ChartData, error)` — individual chart loading with cache

**Tests needed:**
- Parallel execution completes correctly
- Cache hit returns cached data without ClickHouse query
- Cache miss triggers query and stores result
- Singleflight deduplicates concurrent identical requests
- errgroup error handling (one chart fails, others succeed — skip-and-continue)

### Module: ClickHouse Client
**Path:** `internal/clickhouse/`
**Role in changes:** Bug fix + query optimization

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `queries.go` | modify | Fix retention cohort WHERE clause (lines 67-85); add LIMIT to all chart queries; coarser GROUP BY for date ranges > 90 days | medium |
| `queries_test.go` | modify | Add tests for fixed retention query, limit behavior, bucket coarsening | small |

**Interfaces affected:**
- No interface changes (internal query modifications only)

**Tests needed:**
- Retention cohort returns correct filtered data (not full-table scan)
- LIMIT clause applied correctly
- Coarser buckets used for >90 day ranges

### Module: Cache
**Path:** `internal/cache/`
**Role in changes:** New in-memory cache layer

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `chart_cache.go` | create | ChartCache struct with Get/Set/Invalidate methods; per-chart TTL config; singleflight integration; background cleanup goroutine | medium |
| `chart_cache_test.go` | create | Tests for cache operations, TTL expiry, singleflight dedup, concurrent access | medium |

**Interfaces affected:**
- New `ChartCache` interface: `Get(chartID) (ChartData, bool)`, `Set(chartID, data)`, `Invalidate(chartID)`

**Tests needed:**
- Get returns stored data within TTL
- Get returns miss after TTL expiry
- Set overwrites existing entries
- Concurrent access is safe
- Singleflight deduplicates correctly

### Module: API Server
**Path:** `cmd/analytics-api/`
**Role in changes:** Cache initialization and service wiring

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `main.go` | modify | Initialize ChartCache with TTL config; pass to AnalyticsService constructor | small |

**Tests needed:**
- Service initializes with cache correctly

### Module: Frontend
**Path:** `apps/dashboard/`
**Role in changes:** Per-chart queries, progressive loading, downsampling, lazy loading

| File | Change Type | Description | Scope |
|------|------------|-------------|-------|
| `src/components/Overview.tsx` | modify | Remove monolithic query; render ChartGrid with individual chart identifiers | medium |
| `src/components/Chart.tsx` | modify | Apply LTTB downsampling when data > 500 points before passing to Recharts | small |
| `src/components/ChartGrid.tsx` | modify | Add Intersection Observer for lazy loading below-fold charts | small |
| `src/hooks/useChartQuery.ts` | create | Custom hook: fires per-chart GraphQL query, returns {data, loading, error} | medium |
| `src/utils/downsample.ts` | create | LTTB (Largest-Triangle-Three-Buckets) downsampling algorithm | small |

**Interfaces affected:**
- `Overview.tsx` no longer receives `data` prop (manages own data fetching)
- `Chart.tsx` adds optional `maxPoints` prop (default 500)

**Tests needed:**
- useChartQuery fetches correct chart data
- Downsampling reduces points to target while preserving shape
- Lazy loading only fetches visible charts

## Key Technical Decisions

| # | Decision | Reasoning | Alternatives Rejected | User Approved? |
|---|----------|-----------|----------------------|----------------|
| 1 | errgroup with concurrency limit of 8 | Standard Go pattern; bounded goroutines prevent resource exhaustion; 8 matches max chart count | sync.WaitGroup (no error handling), worker pool (over-engineered) | not required |
| 2 | In-memory cache (not Redis) | User confirmed; single instance; simpler deployment; sub-ms reads | Redis (network latency, operational overhead) | yes |
| 3 | singleflight for cache stampede prevention | Deduplicates concurrent requests for same chart; stdlib package; zero config | Probabilistic early expiry, mutex per key | not required |
| 4 | LTTB downsampling at 500 points | Preserves visual shape better than naive sampling; 500 matches typical chart width; user confirmed threshold | Min/max bucketing, random sampling | yes |
| 5 | Per-chart GraphQL queries (not subscription) | Simpler than subscriptions; progressive loading via independent queries; no WebSocket infrastructure needed | GraphQL subscriptions, SSE | not required |
| 6 | Intersection Observer for lazy loading | Native browser API; no library needed; works with chart grid layout | Scroll event listener, virtualization library | not required |

## Dependencies

### Internal Dependencies
- **Analytics Service → Cache:** Service uses ChartCache for caching
- **Analytics Service → ClickHouse:** Service calls ClickHouse client for data
- **API → Cache:** main.go initializes cache and passes to service
- **Frontend → GraphQL:** Per-chart queries depend on existing GraphQL schema

### External Dependencies
- **ClickHouse:** Existing dependency; connection pool settings may need tuning for parallel queries
- **Recharts:** Existing frontend charting library; downsampled data passed to it
- **Apollo Client:** Existing GraphQL client; per-chart queries use its caching

### Migration Dependencies
No migrations required. All changes are code-level.

## Implementation Sequence

| Step | What | Why This Order | Validates |
|------|------|----------------|-----------|
| 1 | Fix retention cohort bug | Independent fix; immediate value; no dependencies | Correct query results |
| 2 | Add query limits and coarser buckets | Independent of cache/parallel; prevents unbounded queries | Large tenant queries bounded |
| 3 | Create ChartCache with singleflight | Foundation for caching layer; must exist before service integration | Cache operations work correctly |
| 4 | Parallelize service with errgroup + cache integration | Core backend optimization; depends on cache and fixed queries | Backend responds fast |
| 5 | Create useChartQuery hook and downsample utility | Frontend foundation; independent of backend changes | Hook fetches data, downsample works |
| 6 | Refactor Overview.tsx for per-chart queries | Depends on useChartQuery hook | Charts load independently |
| 7 | Add progressive loading and skeleton states | Depends on per-chart queries | Progressive rendering works |
| 8 | Add lazy loading for below-fold charts | Final optimization; depends on per-chart rendering | Below-fold charts deferred |

## Risk Zones

| Risk Zone | Location | What Could Go Wrong | Mitigation | Severity |
|-----------|----------|-------------------|------------|----------|
| Connection pool | `service.go` (errgroup) | 8 concurrent ClickHouse queries exhaust pool | Limit errgroup concurrency; monitor pool metrics | medium |
| Cache cold start | `chart_cache.go` | First request after restart hits ClickHouse for all charts | singleflight prevents thundering herd; warm-up optional | medium |
| Retention bug fix | `queries.go:67-85` | Fix changes query behavior; regression possible | Test with production-like data; verify result accuracy | medium |
| Downsampling fidelity | `downsample.ts` | LTTB may hide important data spikes in some edge cases | 500-point threshold is conservative; zoom shows full data | low |
| Race conditions | `service.go` | Shared state between errgroup goroutines | Each goroutine uses own context and connection; no shared mutable state | low |

## Backward Compatibility

### API Changes
GraphQL schema is additive. Existing monolithic overview query continues to work but is no longer used by the frontend.

### Data Changes
No data/schema changes.

### Behavioral Changes
- Overview page renders progressively instead of all-at-once (visual change, intentional)
- Large datasets are downsampled on client (visual change, intentional, preserves shape)

## Critique Review
Design critic returned DESIGN_APPROVED. All criteria PASS. Minor observations: (1) consider adding metrics for cache hit rate; (2) document the warm-up strategy for cache cold start. Both incorporated as notes.

## User Approval Log
- **Full-stack scope:** User chose both backend and frontend optimization
- **Cache type:** User confirmed in-memory over Redis
- **P95 target:** User confirmed 2 seconds
- **Downsampling:** User confirmed 500-point threshold with LTTB
