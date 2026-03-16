# Change Map -- Dashboard Performance Optimization

## Legend

- **Scope:** S (small, <20 lines), M (medium, 20-100 lines), L (large, 100+ lines or structural change)
- **Risk:** Low / Medium / High
- **Phase:** 1 (backend parallelization), 2 (caching), 3 (frontend), 4 (testing)

---

## Backend Changes

### `internal/analytics/service.go`

| Line Range | What Changes | How | Scope | Risk | Phase |
|-----------|-------------|-----|-------|------|-------|
| 1-11 (imports) | Add imports for `errgroup`, `sync/singleflight`, `fmt`, cache package | Add new import lines | S | Low | 1, 2 |
| 13-15 (Service struct) | Add `cache` and `sf` fields | Add `cache *cache.Cache` and `sf singleflight.Group` to struct | S | Low | 2 |
| 17-19 (NewService) | Accept `cache` parameter | Change signature to `NewService(ch *clickhouse.Client, cache *cache.Cache) *Service` | S | Low | 2 |
| 24-55 (GetOverviewPage) | Replace sequential loop with errgroup | Rewrite: pre-allocate `[]*ChartData` slice, spawn goroutines per chart, run summary in parallel, collect non-nil results. Preserve skip-and-continue by returning `nil` from goroutines on error. | L | High | 1 |
| 58-103 (loadChart) | Wrap with cache check + singleflight | Rename existing to `loadChartFromDB`. New `loadChart` checks cache, uses `sf.Do` for dedup, calls `loadChartFromDB` on miss, caches result with per-chart TTL. | L | Medium | 2 |
| 68-72 (ExecuteQuery call) | Handle queries needing extra params | Add conditional: if `cfg.DuplicateParams`, pass doubled params via `ExecuteQueryWithParams` | M | Medium | 1 |
| (new) ~line 110 | Add `chartTTLs` map | Define per-chart TTL configuration as a package-level `map[string]time.Duration` | S | Low | 2 |

### `internal/analytics/models.go`

| Line Range | What Changes | How | Scope | Risk | Phase |
|-----------|-------------|-----|-------|------|-------|
| 50-56 (ChartConfig) | Add `DuplicateParams` field | Add `DuplicateParams bool` field to `ChartConfig` struct | S | Low | 1 |
| 64 (user-retention config) | Set `DuplicateParams: true` | Add field to user-retention entry in `DefaultOverviewCharts` | S | Low | 1 |

### `internal/clickhouse/client.go`

| Line Range | What Changes | How | Scope | Risk | Phase |
|-----------|-------------|-----|-------|------|-------|
| 17-33 (NewClient) | Configure connection pool | Add `opts.MaxOpenConns = 12`, `opts.MaxIdleConns = 6`, `opts.ConnMaxLifetime = 10 * time.Minute`, `opts.DialTimeout = 5 * time.Second` after `ParseDSN` | S | Low | 1 |
| (new, after line 86) | Add `ExecuteQueryWithParams` method | New method identical to `ExecuteQuery` but accepting `...any` instead of `QueryParams`. Reuses the same row-scanning logic. | M | Low | 1 |

### `internal/clickhouse/queries.go`

| Line Range | What Changes | How | Scope | Risk | Phase |
|-----------|-------------|-----|-------|------|-------|
| 27-36 (events_volume) | Coarsen time buckets, add LIMIT | Change `toStartOfHour` to `toStartOfInterval(timestamp, INTERVAL 4 HOUR)`. Add `LIMIT 500`. | S | Medium | 1 |
| 38-48 (top_events_by_count) | Reduce LIMIT | Change `LIMIT 10000` to `LIMIT 100` | S | Low | 1 |
| 66-85 (user_retention_cohort) | Fix 5-param bug | Restructure as CTE or keep as-is but document that it needs 6 params. The key fix is in the calling code (`service.go` + `client.go` `ExecuteQueryWithParams`). | M | Medium | 1 |
| 87-97 (users_by_region) | Add LIMIT | Add `LIMIT 50` at end of query | S | Low | 1 |
| 112-124 (error_rate_over_time) | Coarsen time buckets, add LIMIT | Change `toStartOfMinute` to `toStartOfFiveMinute`. Add `LIMIT 1000`. | S | Medium | 1 |

### `internal/cache/cache.go`

| Line Range | What Changes | How | Scope | Risk | Phase |
|-----------|-------------|-----|-------|------|-------|
| 12-17 (Cache struct) | Add LRU tracking | Add `order []string` field for LRU order tracking | S | Low | 2 |
| 19-22 (cacheItem) | Keep as-is | No change needed; per-key TTL is set via the `Set` method | - | - | - |
| 24-32 (New constructor) | Change signature | Remove `ttl` param (TTL is now per-key). Change to `New(maxSize int) *Cache`. Initialize `order` slice. | S | Low | 2 |
| 45-63 (Set method) | Add per-key TTL, proper LRU eviction | Change signature to `Set(key string, value any, ttl time.Duration)`. Replace naive eviction with LRU: remove front of `order` slice when full. Track key order for LRU. | M | Medium | 2 |
| 34-43 (Get method) | Add LRU touch | On cache hit, move key to end of `order` slice (mark as recently used). Delete expired items on access. | M | Low | 2 |
| 65-78 (cleanup) | Adjust for per-key TTL | Change ticker interval to a fixed period (e.g., 1 minute) since there is no single global TTL. Iterate and remove expired items. | S | Low | 2 |

### `cmd/analytics-api/main.go`

| Line Range | What Changes | How | Scope | Risk | Phase |
|-----------|-------------|-----|-------|------|-------|
| 17 (imports) | Add cache import | Add `"github.com/acme/analytics/internal/cache"` | S | Low | 2 |
| 35 (service creation) | Initialize cache, pass to service | Add `queryCache := cache.New(1000)` before service creation. Change to `analytics.NewService(chClient, queryCache)`. | S | Low | 2 |

### `go.mod`

| Line Range | What Changes | How | Scope | Risk | Phase |
|-----------|-------------|-----|-------|------|-------|
| 5-12 (require) | Add errgroup dependency | Add `golang.org/x/sync` to require block | S | Low | 1 |

---

## Frontend Changes

### `apps/dashboard/src/pages/Overview.tsx`

| Line Range | What Changes | How | Scope | Risk | Phase |
|-----------|-------------|-----|-------|------|-------|
| 1-4 (imports) | Change imports | Remove `OVERVIEW_PAGE_QUERY` import. Add `SUMMARY_QUERY` import. Add `useMemo` from React. Add `ChartWithLoading` import. | S | Low | 3 |
| 21-32 (useQuery) | Replace monolithic query with per-chart pattern | Remove single `useQuery(OVERVIEW_PAGE_QUERY)`. Add time quantization with `useMemo`. Define `CHART_IDS` array and `ABOVE_FOLD` set. | L | Medium | 3 |
| 26-27 (time range) | Quantize timestamps | Replace `new Date().toISOString()` with `quantizeTime()` helper that rounds to 5-minute boundaries. Wrap in `useMemo`. | S | Low | 3 |
| 34-36 (loading state) | Remove global loading gate | Delete the `if (loading) return <div>Loading...</div>` block. Each chart now has its own loading state. | S | Low | 3 |
| 44-77 (render) | Restructure to progressive loading | Extract `SummaryBar` as a separate component with its own `useQuery(SUMMARY_QUERY)`. Replace `charts.map(chart => <Chart>)` with `CHART_IDS.map(id => <ChartWithLoading lazy={...}>)`. | L | Medium | 3 |
| (new, top of file) | Add `quantizeTime` helper | Add utility function `quantizeTime(date, intervalMs)` that floors timestamps to interval boundaries. | S | Low | 3 |

### `apps/dashboard/src/components/ChartWithLoading.tsx` (NEW FILE)

| What | Description | Scope | Risk | Phase |
|------|------------|-------|------|-------|
| New component | Wraps per-chart data fetching with `useQuery(CHART_DATA_QUERY)`, loading skeleton, error state, and IntersectionObserver for lazy loading. | L | Medium | 3 |

**Structure:**
- `ChartWithLoading` -- outer component handling visibility (IntersectionObserver for lazy charts)
- `ChartFetcher` -- inner component that runs `useQuery` and renders `Chart` or loading/error states
- Props: `tenantId`, `chartId`, `from`, `to`, `lazy`
- Dependencies: `@apollo/client`, `react`, `Chart` component, `CHART_DATA_QUERY`

### `apps/dashboard/src/components/Chart.tsx`

| Line Range | What Changes | How | Scope | Risk | Phase |
|-----------|-------------|-----|-------|------|-------|
| 1-17 (imports) | Add `useMemo` import | Add `useMemo` from React | S | Low | 3 |
| (new, before component) | Add `downsampleData` function | New function that reduces data points using bucket averaging. Configurable max per chart type: line=500, area=500, bar=100, pie=50. | M | Medium | 3 |
| (new, before component) | Add `MAX_POINTS` constant | Map of chart type to maximum point count | S | Low | 3 |
| 49-55 (data transform) | Wrap in useMemo, add downsampling | Call `downsampleData` before transformation. Wrap both downsampling and `chartData` mapping in `useMemo` to prevent recomputation on re-renders. | M | Low | 3 |
| 84 (XAxis) | Delegate date formatting to tickFormatter | Change `dataKey="time"` to `dataKey="timestamp"` with `tickFormatter`. Remove pre-formatting from the data transform step. | S | Low | 3 |

### `apps/dashboard/src/components/ChartGrid.tsx`

| Line Range | What Changes | How | Scope | Risk | Phase |
|-----------|-------------|-----|-------|------|-------|
| No changes | ChartGrid itself remains a layout component | The lazy loading logic lives in `ChartWithLoading`, not here | - | - | - |

### `apps/dashboard/src/api/analytics.ts`

| Line Range | What Changes | How | Scope | Risk | Phase |
|-----------|-------------|-----|-------|------|-------|
| (new, after line 65) | Add `SUMMARY_QUERY` | New GraphQL query for summary-only data. Uses the existing `summary` query in the schema. | S | Low | 3 |
| 13-42 (OVERVIEW_PAGE_QUERY) | Keep but mark deprecated | Add comment marking as deprecated (kept for backward compatibility with any other consumers). Do not delete. | S | Low | 3 |

---

## New Files

| File | Purpose | Scope | Phase |
|------|---------|-------|-------|
| `apps/dashboard/src/components/ChartWithLoading.tsx` | Per-chart data fetching wrapper with lazy loading | L | 3 |
| `internal/analytics/service_test.go` | Tests for parallel execution, caching, singleflight | L | 4 |
| `internal/clickhouse/queries_test.go` | Tests for query param counts | M | 4 |
| `internal/cache/cache_test.go` | Tests for LRU eviction, per-key TTL, concurrency | M | 4 |

---

## Dependency Changes

| File | Package | Version | Why |
|------|---------|---------|-----|
| `go.mod` | `golang.org/x/sync` | latest | Required for `errgroup` in parallel query execution |

No new frontend dependencies are needed. `IntersectionObserver` is a native browser API. `useMemo` is built into React.

---

## Files NOT Changed (explicitly)

| File | Why Not |
|------|---------|
| `internal/analytics/schema.graphql` | GraphQL API contract preserved. No new queries or fields needed -- `chartData` and `summary` queries already exist in the schema. |
| `internal/analytics/resolver.go` | Resolver logic is thin delegation to service. No changes needed -- the `ChartData()` and `Summary()` resolvers already exist and work. |
| `migrations/001_analytics_tables.sql` | ClickHouse schema changes are explicitly out of scope. |
| `docker-compose.yml` | Infrastructure unchanged. |
| `apps/dashboard/package.json` | No new npm dependencies needed. |

---

## Change Order (Dependencies)

```
Phase 1: Backend Foundation
  go.mod (add x/sync)
    -> client.go (pool config + ExecuteQueryWithParams)
      -> models.go (add DuplicateParams)
        -> queries.go (fix retention, coarsen queries, add limits)
          -> service.go (errgroup parallelization)

Phase 2: Caching
  cache.go (upgrade to per-key TTL + LRU)
    -> service.go (add cache + singleflight to loadChart)
      -> main.go (wire cache)

Phase 3: Frontend
  analytics.ts (add SUMMARY_QUERY)
    -> ChartWithLoading.tsx (new file)
      -> Chart.tsx (add downsampling + useMemo)
        -> Overview.tsx (rewrite to progressive loading)

Phase 4: Testing (parallel with each phase)
  service_test.go
  queries_test.go
  cache_test.go
```

---

## Impact Summary

| Metric | Files Changed | Files Created | Lines Changed (est.) | Lines Added (est.) |
|--------|--------------|--------------|---------------------|-------------------|
| Backend | 6 | 0 | ~200 | ~150 |
| Frontend | 3 | 1 | ~100 | ~150 |
| Tests | 0 | 3 | 0 | ~300 |
| **Total** | **9** | **4** | **~300** | **~600** |
