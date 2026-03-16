# Coverage Matrix

> Task: Reduce analytics dashboard overview page load time from 8-10s to <2s (P95) through full-stack optimization
> Coverage verdict: COVERAGE_OK
> Confidence: high

## Requirement Traceability

### From Agreed Task Model

| Requirement / Scenario | Source | Covered By | Status |
|------------------------|--------|-----------|--------|
| P95 load <2s | acceptance criteria | ST-5, ST-8, ST-7, ST-9, ST-10 (combined effect) | covered |
| All 8 charts correct data | acceptance criteria | ST-1, ST-5, ST-8 | covered |
| Retention cohort works | acceptance criteria | ST-1 | covered |
| GraphQL contract unchanged | acceptance criteria | ST-6 (additive queries), ST-8 (no schema breaks) | covered |
| Progressive loading works | acceptance criteria | ST-8 | covered |
| Apollo cache hits | acceptance criteria | ST-6 (time range quantization) | covered |
| ClickHouse load reduced | acceptance criteria | ST-3 (cache), ST-5 (singleflight + parallel) | covered |
| Race detector passes | acceptance criteria | ST-5 (completion criterion) | covered |
| User opens overview page -> per-chart queries fire | primary scenario | ST-6, ST-8 | covered |
| Above-fold charts load first | primary scenario | ST-7, ST-8 | covered |
| Backend: cache check -> singleflight -> parallel ClickHouse | primary scenario | ST-3, ST-5 | covered |
| Data downsampled -> charts render | primary scenario | ST-4, ST-9 | covered |
| All charts visible in <2s P95 | primary scenario | ST-5, ST-8, ST-10 (end-to-end) | covered |
| Cache cold start -> singleflight prevents thundering herd | mandatory edge case | ST-3 | covered |
| Large tenant -> query limits prevent unbounded results | mandatory edge case | ST-2 | covered |
| Retention cohort -> bug fix makes it work | mandatory edge case | ST-1 | covered |
| ClickHouse down -> skip-and-continue preserved | mandatory edge case | ST-5 | covered |
| GraphQL API backward compatible | constraint | ST-6, ST-8 | covered |
| 2s P95 target | constraint | all subtasks combined | covered |
| Zero test coverage -> new tests needed | constraint | ST-1, ST-2, ST-3, ST-4, ST-5, ST-6 | covered |
| ClickHouse schema out of scope | constraint | n/a (exclusion) | covered |
| Skip-and-continue pattern preserved | constraint | ST-5 | covered |

### From Implementation Design

| Design Element | Source | Covered By | Status |
|----------------|--------|-----------|--------|
| Analytics Service — parallel errgroup + cache integration | implementation-design.md | ST-5 | covered |
| ClickHouse Client — bug fix | implementation-design.md | ST-1 | covered |
| ClickHouse Client — query limits and coarser buckets | implementation-design.md | ST-2 | covered |
| Cache — ChartCache with per-chart TTL + singleflight | implementation-design.md | ST-3 | covered |
| API — cache init and service wiring | implementation-design.md | ST-10 | covered |
| Frontend — per-chart queries (useChartQuery) | implementation-design.md | ST-6 | covered |
| Frontend — Overview.tsx refactor | implementation-design.md | ST-8 | covered |
| Frontend — Chart.tsx downsampling | implementation-design.md | ST-9 | covered |
| Frontend — ChartGrid.tsx lazy loading | implementation-design.md | ST-7 | covered |
| Frontend — downsample utility | implementation-design.md | ST-4 | covered |
| New entity: ChartCache | implementation-design.md | ST-3 | covered |
| New entity: chartCacheConfig | implementation-design.md | ST-3 | covered |
| New entity: useChartQuery hook | implementation-design.md | ST-6 | covered |
| New entity: downsample utility | implementation-design.md | ST-4 | covered |
| Modified entity: LoadOverviewData | implementation-design.md | ST-5 | covered |
| Modified entity: QueryRetentionCohort | implementation-design.md | ST-1 | covered |
| Modified entity: ClickHouseClient query methods | implementation-design.md | ST-2 | covered |
| Modified entity: Overview.tsx | implementation-design.md | ST-8 | covered |
| Modified entity: Chart.tsx | implementation-design.md | ST-9 | covered |
| Modified entity: ChartGrid.tsx | implementation-design.md | ST-7 | covered |
| Interface change: LoadOverviewData return type | implementation-design.md | ST-5 | covered |
| Interface change: Overview.tsx data prop removal | implementation-design.md | ST-8 | covered |
| Interface change: Chart.tsx maxPoints prop | implementation-design.md | ST-9 | covered |

### From Change Map

| File / Change | Source | Covered By | Status |
|---------------|--------|-----------|--------|
| `internal/analytics/service.go` — parallel errgroup, cache, singleflight | change-map.md | ST-5 | covered |
| `internal/analytics/service_test.go` — parallel, caching, singleflight tests | change-map.md | ST-5 | covered |
| `internal/clickhouse/queries.go` — retention bug fix (lines 67-85) | change-map.md | ST-1 | covered |
| `internal/clickhouse/queries.go` — LIMIT, coarser GROUP BY | change-map.md | ST-2 | covered |
| `internal/clickhouse/queries_test.go` — retention tests | change-map.md | ST-1 | covered |
| `internal/clickhouse/queries_test.go` — limit and bucket tests | change-map.md | ST-2 | covered |
| `cmd/analytics-api/main.go` — cache init, service wiring | change-map.md | ST-10 | covered |
| `apps/dashboard/src/components/Overview.tsx` — per-chart queries | change-map.md | ST-8 | covered |
| `apps/dashboard/src/components/Chart.tsx` — LTTB downsampling | change-map.md | ST-9 | covered |
| `apps/dashboard/src/components/ChartGrid.tsx` — Intersection Observer | change-map.md | ST-7 | covered |
| `internal/cache/chart_cache.go` — new cache module | change-map.md | ST-3 | covered |
| `internal/cache/chart_cache_test.go` — cache tests | change-map.md | ST-3 | covered |
| `apps/dashboard/src/hooks/useChartQuery.ts` — per-chart hook | change-map.md | ST-6 | covered |
| `apps/dashboard/src/utils/downsample.ts` — LTTB algorithm | change-map.md | ST-4 | covered |
| `apps/dashboard/src/utils/downsample.test.ts` — LTTB tests | change-map.md | ST-4 | covered |
| Config: Chart TTL configuration | change-map.md | ST-3 | covered |
| Config: errgroup concurrency limit | change-map.md | ST-5 | covered |

### From Design Decisions

| Decision | Source | Covered By | Status |
|----------|--------|-----------|--------|
| DD-1: errgroup with concurrency limit of 8 | design-decisions.md | ST-5 | covered |
| DD-2: In-memory cache (not Redis) | design-decisions.md | ST-3 | covered |
| DD-3: singleflight for cache stampede prevention | design-decisions.md | ST-3 | covered |
| DD-4: LTTB downsampling at 500 points | design-decisions.md | ST-4, ST-9 | covered |
| DD-5: Per-chart GraphQL queries (not subscription) | design-decisions.md | ST-6, ST-8 | covered |
| DD-6: Intersection Observer for lazy loading | design-decisions.md | ST-7 | covered |
| Deferred: Cache warm-up strategy | design-decisions.md | n/a (excluded) | covered |
| Deferred: Per-chart time ranges | design-decisions.md | n/a (excluded) | covered |
| Deferred: Materialized views | design-decisions.md | n/a (excluded) | covered |

## Coverage Gaps

No coverage gaps detected.

## Over-Coverage

No over-coverage detected. All subtasks map directly to the agreed implementation design and change map. No subtask adds work beyond the agreed scope.

## Done-State Validation

**If all subtasks are completed, is the original task complete?**
- **Answer:** yes
- **Reasoning:** The 10 subtasks collectively cover all three optimization layers (backend parallelization + caching, ClickHouse query fixes, frontend progressive loading + downsampling + lazy loading). Every acceptance criterion from the agreed task model maps to at least one subtask with specific completion criteria. The primary scenario works end-to-end: user opens page -> per-chart queries fire -> cache/singleflight/parallel ClickHouse -> progressive rendering with downsampling -> lazy loading for below-fold charts. All mandatory edge cases are handled. The bug fix, performance optimizations, and new cache module are all covered. API wiring connects backend components. No work falls between subtasks.
