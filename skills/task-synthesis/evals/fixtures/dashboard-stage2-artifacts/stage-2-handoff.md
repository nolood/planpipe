# Stage 2 Handoff — Deep Analysis Complete

## Task Summary

The analytics dashboard's main overview page loads 8 charts in 8-10 seconds for ~50,000 daily active users. The target is under 2 seconds. The system is a Go backend (gqlgen GraphQL API) querying ClickHouse (~500M-row events table) serving data to a React frontend (Apollo Client + Recharts). Deep analysis has identified multiple compounding bottlenecks: 8 ClickHouse queries executed sequentially on the backend with no caching, unbounded result sets loaded entirely into memory, all data sent to the frontend without downsampling, all charts rendered simultaneously as SVG, and a cache-busting time range parameter that prevents Apollo's client-side cache from ever producing a hit. An unused in-memory cache utility exists in the codebase. A bug in the retention cohort query (parameter count mismatch) means one of the 8 charts has likely never worked.

## Classification
- **Type:** refactor (performance optimization of existing functionality)
- **Complexity:** high — multiple bottlenecks across 3 layers (database, API, frontend) requiring coordinated changes with no existing test coverage
- **Primary risk area:** technical — introducing concurrency and caching into untested code with no regression safety net

## Analysis Summary

### Product / Business
The task exists because degraded dashboard performance is eroding user trust and engagement with the data platform. The primary actor is a product manager or analyst who opens the overview page daily to monitor key metrics. Success means charts load in under 2 seconds (wall-clock), the experience feels responsive, and the dashboard remains trustworthy. The minimum viable outcome is backend parallelization of chart queries, without which the 2-second target is mathematically unachievable for large tenants.

### Codebase / System
The performance bottleneck spans three layers. Backend: `GetOverviewPage` in `service.go:24-55` loads 8 charts in a sequential for-loop, each triggering a ClickHouse query on the raw events table (2-5 seconds per query for large tenants). No caching is integrated despite an unused cache utility at `internal/cache/cache.go`. Frontend: a single monolithic GraphQL query blocks rendering until all 8 charts complete, then all charts render simultaneously with no data downsampling (10k+ SVG elements per chart). A cache-busting bug in `Overview.tsx:26-27` prevents Apollo's cache from ever hitting. A confirmed bug in `queries.go:67-85` (retention cohort query has 5 SQL parameters but only 3 are passed) means one chart has likely always failed silently.

### Constraints / Risks
The GraphQL API contract must be preserved for unknown consumers. The codebase has zero test coverage across all modules, making any change risky. The highest risk is parallelizing queries without connection pool configuration (could overwhelm ClickHouse) and cache thundering herd under 50k DAU load. ClickHouse infrastructure changes (materialized views) are potentially out of scope. All proposed changes are rollback-safe since they modify internal execution without changing external contracts.

## System Map

### Modules Involved
| Module | Path | Role in Task | Change Scope |
|--------|------|-------------|-------------|
| Analytics Service | `internal/analytics/` | Orchestrates chart loading; contains the sequential bottleneck | large |
| ClickHouse Client | `internal/clickhouse/` | Executes queries; contains SQL definitions and unbounded result loading | medium |
| Cache Utility | `internal/cache/` | Unused TTL cache that could be wired into the service layer | small |
| API Entry Point | `cmd/analytics-api/` | Server wiring; needs cache initialization | small |
| Dashboard Frontend | `apps/dashboard/` | Monolithic query, no progressive loading, no downsampling | large |
| Database Schema | `migrations/` | Defines events table; no materialized views | small (if views added) |

### Key Change Points
| Location | What Changes | Scope |
|----------|-------------|-------|
| `internal/analytics/service.go:GetOverviewPage` | Parallelize 8 sequential chart queries using errgroup | medium |
| `internal/analytics/service.go:loadChart` | Integrate caching (check cache before ClickHouse, store after) | medium |
| `internal/clickhouse/queries.go:user_retention_cohort` | Fix bug: 5 SQL params but only 3 passed | small |
| `internal/clickhouse/queries.go` (all queries) | Add result limits; coarsen time buckets where appropriate | medium |
| `internal/clickhouse/client.go:ExecuteQuery` | Support variable parameter counts for different query shapes | medium |
| `cmd/analytics-api/main.go` | Initialize cache and inject into service | small |
| `apps/dashboard/src/pages/Overview.tsx` | Switch to per-chart queries with progressive loading | large |
| `apps/dashboard/src/pages/Overview.tsx:26-27` | Quantize time range to prevent Apollo cache busting | small |
| `apps/dashboard/src/components/Chart.tsx:50-55` | Add data downsampling before rendering (cap at ~200-500 points) | medium |
| `apps/dashboard/src/components/ChartGrid.tsx` | Add lazy loading for below-fold charts | medium |

### Critical Dependencies
- **ClickHouse cluster:** All chart data comes from this single data source. Query parallelization multiplies concurrent load 8x per request. Connection pool must be appropriately sized.
- **gqlgen framework:** Resolver architecture constrains how parallelization can be implemented (service layer vs. GraphQL field-level resolution).
- **Apollo Client:** Frontend caching and query management. Switching from monolithic to per-chart queries changes the network pattern from 1 request to 8 requests.
- **Recharts rendering model:** SVG-based rendering means data point count directly correlates to render time. Cannot be changed without switching libraries.

## Constraints the Plan Must Respect
- **GraphQL API contract must be backward-compatible:** Unknown consumers may exist. Changes must be additive (new fields/queries) not breaking (removing/renaming fields). Source: requirements.draft.md scope exclusions.
- **2-second target is wall-clock from user perspective:** Includes network latency, backend processing, and frontend rendering. Not just API response time. Source: requirements.draft.md.
- **No existing test coverage anywhere in the codebase:** Any changes are deployed without regression safety nets. Manual testing or new test creation is necessary. Source: codebase investigation -- no test files found in any module.
- **ClickHouse schema changes may be out of scope:** Materialized views and secondary indexes are infrastructure-level changes. The requirements mark these as "not included (to be confirmed)." Source: requirements.draft.md scope.
- **Skip-and-continue error handling must be preserved:** Failed charts should not fail the entire page (`service.go:33-36`). This is a deliberate resilience pattern. Source: code.

## Risks the Plan Must Mitigate

| Risk | Likelihood | Impact | Suggested Mitigation |
|------|-----------|--------|---------------------|
| Parallel ClickHouse queries overwhelm connection pool or cluster | medium | high | Configure explicit connection pool size; add concurrency limiter (semaphore); load test before rollout |
| Cache thundering herd (many concurrent requests for same tenant bypass cache) | high | medium | Use `sync/singleflight` to deduplicate in-flight queries for the same cache key |
| Introducing concurrency bugs in untested code (race conditions, deadlocks) | medium | high | Use errgroup for structured concurrency; add tests for the parallel path; run race detector (`go test -race`) |
| Data downsampling hides important operational signals (error spikes) | medium | medium | Make downsampling configurable per chart type; keep finer granularity for operational charts |
| Naive cache eviction fails under load (silently stops accepting entries) | medium | medium | Replace with LRU eviction or size-bounded cache; add monitoring for cache hit/miss rates |

## Product Requirements for Planning
- **Main scenario:** User opens overview page and sees all 8 charts with data within 2 seconds, with progressive loading so early charts appear before later ones complete.
- **Success signals:** P95 page load time < 2 seconds; time to first chart visible; dashboard engagement rate; ClickHouse query load reduction.
- **Minimum viable outcome:** Backend parallelization of chart queries (without this, the 2-second target is mathematically unachievable for large tenants -- 8 sequential queries at 1s each = 8s minimum).
- **Backward compatibility:** GraphQL API contract unchanged for existing consumers; visual output of charts unchanged (data accuracy preserved).

## Critique Results

All three analyses were assessed as SUFFICIENT by the independent critic. Key findings from the critique:

**Product analysis:** Strong business intent connecting performance degradation to user trust erosion. Good edge cases, particularly the thundering herd scenario and cache-busting time range. Minor observation: could have more explicitly addressed the freshness-vs-caching tradeoff since no SLA is defined.

**System analysis:** Specific and evidence-based with file paths, line numbers, and verified code findings. The retention cohort query bug is a high-confidence finding. Minor observation: could have checked gqlgen documentation for dataloader/per-field resolver support.

**Constraints/risks analysis:** Well-calibrated risks with appropriate likelihood/impact ratings. Backward compatibility thoroughly assessed for each change type. Minor observation: could have investigated ClickHouse resource constraints from the Docker Compose setup.

No refinement was required. All critic observations were minor and have been documented as open questions where relevant.

## Open Questions for Planning

1. **What is the acceptable data staleness for cached dashboard data?** The error_rate chart monitors real-time operational health and may need a shorter cache TTL (30s-1min) than historical charts like retention (5-10min). No freshness SLA is defined anywhere. This affects caching strategy design.
2. **Are there other consumers of the GraphQL API?** This determines whether API contract changes are safe. The `overviewPage` query is dashboard-specific, but `chartData` could be shared. If no other consumers exist, the contract constraint is softer than assumed.
3. **What is the ClickHouse cluster topology (single node vs. distributed)?** Parallelizing 8 queries per request against a single node has different capacity implications than against a cluster. Docker Compose shows a single container, but production may differ.
4. **Is the 2-second target P50, P95, or P99?** P50 under 2s is achievable with less effort than P99 under 2s. The difference significantly affects which optimizations are necessary.
5. **Does the `clickhouse-go` driver support configurable connection pooling?** If queries are parallelized to 8 concurrent per request, the pool must handle 8 x concurrent_users connections. Default pool size is unknown.
6. **Should the retention cohort chart bug be fixed as part of this task or tracked separately?** It is currently silently failing. Fixing it adds a working chart to the page, which is positive, but the self-join query is inherently expensive.
7. **Is per-chart time range (using ChartConfig.TimeRange) intended behavior?** If activated, it would reduce data volumes for some charts (error_rate from 30d to 24h) but change the user experience.

## Detailed Analyses
These files contain the full analysis and can be consulted for details:
- `product-analysis.md` — full product/business analysis
- `system-analysis.md` — full codebase/system analysis
- `constraints-risks-analysis.md` — full constraints/risks analysis
