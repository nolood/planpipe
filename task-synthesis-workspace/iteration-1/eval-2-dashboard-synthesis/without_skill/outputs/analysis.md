# Synthesized Analysis: Dashboard Performance Optimization

## 1. Unified Problem Statement

The analytics dashboard overview page loads 8 charts in 8-10 seconds for ~50,000 daily active users against a target of under 2 seconds. The root cause is a compound bottleneck spanning three layers -- database queries, backend orchestration, and frontend rendering -- with no caching, no parallelism, and no data-volume management at any layer. A confirmed bug in the retention cohort query means one of the 8 charts has likely never worked. The degradation is driven by data growth (~500M rows in the events table) outpacing an implementation that assumed small data volumes.

## 2. Cross-Layer Bottleneck Map

The bottlenecks compound multiplicatively, not additively. Each layer amplifies the cost of the previous one:

```
Layer 1: ClickHouse Queries (root cause of latency)
  - 8 queries hit raw events table (~500M rows), 2-5s each for large tenants
  - No materialized views, no result limits (except one query at LIMIT 10000)
  - Retention cohort query is broken (5 params needed, 3 passed) -- silently fails
  - Summary data runs 4 more sequential QueryRow calls

Layer 2: Backend Orchestration (converts per-query cost to sum, not max)
  - GetOverviewPage runs 8 chart loads in a sequential for-loop
  - Total backend time = SUM of all query times (16-40s for large tenants)
  - No caching despite an unused cache utility existing in the codebase
  - No connection pool configuration for parallel access

Layer 3: Frontend Rendering (blocks on slowest + renders everything at once)
  - Single monolithic GraphQL query blocks until ALL 8 charts complete
  - new Date().toISOString() in query variables cache-busts Apollo on every load
  - All 8 charts render simultaneously with no lazy loading
  - No data downsampling: 10k+ SVG elements per chart cause 2-5s render time
```

**Critical insight:** Backend parallelization alone changes total time from SUM(query_times) to MAX(query_times), which is the single largest improvement available. Without it, the 2-second target is mathematically impossible for large tenants.

## 3. Findings Reconciliation

### Agreements Across Analyses

All three analysis streams converge on these points:

1. **Sequential query execution is the primary bottleneck.** Product analysis identifies it as the minimum viable outcome, system analysis pinpoints it at `service.go:24-55`, and constraints analysis confirms it is rollback-safe.

2. **The retention cohort query bug is real and high-confidence.** System analysis found the parameter mismatch at `queries.go:67-85`. Constraints analysis rates it as high-likelihood / low-impact (because it silently fails). Product analysis does not mention it directly but includes it implicitly in the "8 charts" count.

3. **Cache thundering herd is the highest-likelihood risk.** All analyses agree that 50k DAU with no caching creates a thundering herd scenario. The unused cache utility at `internal/cache/` is the starting point but needs improvements (LRU eviction, singleflight).

4. **The GraphQL API contract must be preserved.** All analyses agree this is a hard constraint with unknown downstream consumers.

5. **Zero test coverage across the entire codebase.** Every module has no tests. This is the primary risk amplifier for all proposed changes.

### Tensions and Resolutions

**Tension 1: Caching TTL vs. data freshness**
- Product analysis flags that error_rate monitors real-time operational health and needs tight freshness.
- System analysis notes that per-chart TimeRange configs exist but are unused.
- Constraints analysis identifies that no freshness SLA is defined anywhere.
- **Resolution:** Use differentiated cache TTLs per chart type. Operational charts (error_rate) get 30s-1min TTL. Historical charts (retention, conversion funnel) get 5-10min TTL. This is an open question for user confirmation but has a sensible default.

**Tension 2: Scope of ClickHouse changes**
- System analysis identifies materialized views as a potential optimization.
- Constraints analysis notes these are "potentially out of scope" per requirements.
- Product analysis does not address infrastructure changes.
- **Resolution:** Defer materialized views. Evaluate whether parallelization + caching alone meet the 2s target first. Keep materialized views as a Phase 2 option if needed.

**Tension 3: Frontend query pattern (monolithic vs. per-chart)**
- System analysis identifies the existing `CHART_DATA_QUERY` as an unused per-chart alternative.
- Constraints analysis notes switching from 1 request to 8 requests changes network patterns.
- Product analysis expects progressive loading (first chart visible quickly).
- **Resolution:** Per-chart queries enable progressive loading and are the recommended approach. The schema already supports it. Network overhead of 8 small requests is lower than 1 large blocked request for perceived performance.

**Tension 4: Data downsampling granularity**
- System analysis notes error_rate produces up to 1,440 rows (by minute for 24h).
- Constraints analysis warns downsampling could hide short error spikes.
- Product analysis identifies time to first chart as a key success signal.
- **Resolution:** Downsample on the frontend to ~200-500 points per chart, but make the threshold configurable per chart type. Operational charts (error_rate) should retain finer granularity than historical charts.

## 4. Unified System Map

### Modules and Change Scope

| Module | Path | Change Size | Key Changes |
|--------|------|-------------|-------------|
| Analytics Service | `internal/analytics/` | LARGE | Parallelize chart loading with errgroup; integrate caching; preserve skip-on-error pattern |
| ClickHouse Client | `internal/clickhouse/` | MEDIUM | Fix retention query bug; add result limits; support variable param counts |
| Cache Utility | `internal/cache/` | SMALL-MEDIUM | Wire into service; improve eviction to LRU; add singleflight for dedup |
| API Entry Point | `cmd/analytics-api/` | SMALL | Initialize cache; inject into service constructor |
| Dashboard Frontend | `apps/dashboard/` | LARGE | Per-chart queries; progressive loading; time-range quantization; data downsampling; lazy loading for below-fold charts |
| Database Schema | `migrations/` | NONE (Phase 1) | Deferred -- evaluate after parallelization + caching results |

### Change Dependency Order

```
1. Fix retention cohort query bug (independent, small, immediate value)
2. Backend parallelization with errgroup (prerequisite for 2s target)
3. Cache integration with singleflight (reduces ClickHouse load under concurrency)
4. Frontend: quantize time ranges (enables Apollo cache hits)
5. Frontend: per-chart queries with progressive loading (perceived performance)
6. Frontend: data downsampling (rendering performance)
7. Frontend: lazy loading for below-fold charts (rendering performance)
```

Steps 1-3 are backend changes. Steps 4-7 are frontend changes. They can proceed in parallel after step 2 is validated.

## 5. Consolidated Constraints

| Constraint | Source | Hardness | Impact on Plan |
|-----------|--------|----------|----------------|
| GraphQL API backward compatibility | requirements.draft.md | HARD | All changes must be additive; no field removal or type changes |
| 2-second wall-clock target | PM requirement | HARD | Drives minimum scope (parallelization required) |
| Zero test coverage | codebase investigation | HARD (fact) | Every change needs new tests or manual verification |
| Skip-and-continue error handling | service.go:33-36 | HARD (pattern) | Parallelization must preserve partial-failure resilience |
| No ClickHouse schema changes (Phase 1) | requirements.draft.md (to be confirmed) | SOFT | Materialized views deferred unless parallelization+caching insufficient |
| No downtime deployment | implied by 50k DAU | MEDIUM | Changes must be deployable incrementally |

## 6. Consolidated Risk Matrix

| # | Risk | Likelihood | Impact | Mitigation |
|---|------|-----------|--------|------------|
| R1 | Parallel queries overwhelm ClickHouse connection pool | Medium | High | Configure explicit pool size; add semaphore to cap concurrent queries; load test |
| R2 | Cache thundering herd under 50k DAU | High | Medium | Use `sync/singleflight` to deduplicate in-flight queries per cache key |
| R3 | Concurrency bugs in untested code (races, deadlocks) | Medium | High | Use errgroup for structured concurrency; add tests; run `go test -race` |
| R4 | Downsampling hides operational signals (error spikes) | Medium | Medium | Configurable per-chart downsampling thresholds; finer granularity for operational charts |
| R5 | Naive cache eviction fails silently under load | Medium | Medium | Replace with LRU/size-bounded cache; add cache hit/miss monitoring |
| R6 | No regression safety net (zero tests) | High | High | Add targeted tests for parallelized path, cache integration, and query correctness before deploying |

## 7. Open Questions (Consolidated and Prioritized)

### Must Resolve Before Planning

1. **What is the acceptable data staleness for cached dashboard data?** Affects cache TTL design. Proposed default: 30s-1min for operational charts, 5-10min for historical charts.
2. **Is the 2-second target P50, P95, or P99?** Determines optimization depth. P95 is assumed if not specified.
3. **Should the retention cohort bug be fixed in this task or separately?** Recommended: fix it here since the code is being touched anyway, but the self-join query will be expensive once it works.

### Should Resolve Before Implementation

4. **Are there other consumers of the GraphQL API?** Determines constraint hardness for API changes.
5. **What is the ClickHouse cluster topology in production?** Single node vs. distributed affects parallelization safety.
6. **Does `clickhouse-go/v2` support configurable connection pooling?** Determines whether pool sizing is possible.

### Can Resolve During Implementation

7. **Is per-chart TimeRange intended behavior?** If activated, reduces data volumes for some charts.
8. **Is gzip/compression enabled on the API?** Affects payload transfer time for large responses.
9. **What are ClickHouse resource limits in production?** Affects parallelization aggressiveness.
