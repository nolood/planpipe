# Synthesized Task Analysis

## Task Goal

Reduce the analytics dashboard main overview page load time from 8-10 seconds to under 2 seconds (wall-clock, from the user's perspective) for ~50,000 daily active users, by addressing compounding bottlenecks across the Go backend, ClickHouse query layer, and React frontend -- without breaking the existing GraphQL API contract.

## Problem Statement

The analytics dashboard is the primary data interface for 50,000 daily active users. Its main overview page loads 8 charts sequentially from a 500M-row ClickHouse table with no caching, no parallelization, no data downsampling, and a cache-busting frontend bug that prevents client-side caching from ever working. The degraded performance (4-5x slower than target) is eroding user trust and engagement with the data platform. Users are the primary consumers of this data -- if the dashboard is too slow to check quickly, they disengage, undermine their own data-driven workflows, and may seek alternative tools. This is a repair task, not a new feature: the system worked at smaller data volumes but has crossed a threshold where the lack of optimization is no longer tolerable.

## Key Scenarios

### Primary Scenario
1. **Trigger:** User (product manager, analyst, stakeholder) opens or refreshes the analytics dashboard overview page
2. **Current state:** The frontend sends a single monolithic GraphQL query for all 8 charts + summary data. The backend receives the request and sequentially queries ClickHouse for each chart (2-5 seconds per query for large tenants), accumulating latency as a sum. After 8-10 seconds the full response returns to the frontend, which renders all 8 charts simultaneously with no downsampling (10k+ SVG elements per chart).
3. **Target state:** Charts begin appearing within ~500ms (progressive loading). All charts are visible and interactive within 2 seconds. The experience feels responsive. Repeat visits within a short window benefit from caching.
4. **End state:** The user sees all 8 charts with correct data, can begin interpreting immediately, and trusts the dashboard as a reliable, fast tool.

### Mandatory Edge Cases
- **Large tenants (>10M events):** This is the worst case and the primary driver of the problem. Queries take 2-5s each for these tenants. The optimization must specifically handle this -- backend parallelization is mathematically required to meet the 2-second target for these tenants (8 sequential queries at 1s+ each = 8s+ minimum).
- **Concurrent page loads (thundering herd):** With 50,000 DAU and likely usage spikes at start-of-business, many users from the same tenant may load the dashboard simultaneously. Without request coalescing (e.g., singleflight), each request would independently bypass the cache and hit ClickHouse, multiplying load. All three analyses flag this; the constraints analysis rates it as high likelihood.
- **Cache-busting time range:** The frontend passes `new Date().toISOString()` as the `to` parameter, which changes every millisecond. This prevents Apollo Client's cache from ever producing a hit and ensures every visit is a cold load. This must be fixed by quantizing time ranges to a boundary (e.g., nearest minute).
- **Retention cohort query bug:** The `user_retention_cohort` query in `queries.go:67-85` has 5 SQL placeholders but `ExecuteQuery` only passes 3 parameters. This chart has likely always failed silently (the service skips errors at `service.go:33-36`). Fixing the parallelization without fixing this bug would mean this chart continues to silently fail.

### Deferred Scenarios
- **Materialized views in ClickHouse:** Adding pre-aggregated views could dramatically reduce query times but requires schema-level infrastructure changes marked as potentially out of scope. Risk of deferring: the 2-second target may be harder to hit for the largest tenants without them, though parallelization + caching may suffice.
- **Other dashboard pages:** Only the main overview page is in scope. Other pages may have similar issues but are not addressed here. Risk of deferring: low, as the overview page is the highest-traffic page.
- **Per-chart time range activation:** `ChartConfig.TimeRange` is defined but unused. Activating it would reduce data volumes for some charts (e.g., error_rate from 30d to 24h) but changes user-facing behavior. Risk of deferring: low, as it's an optimization enhancement, not a requirement.

## System Scope

### Affected Modules
| Module | Path | Role in Task | Change Scope |
|--------|------|-------------|-------------|
| Analytics Service | `internal/analytics/` | Contains the sequential bottleneck in `GetOverviewPage`; orchestrates chart loading and needs caching integration | large |
| ClickHouse Client | `internal/clickhouse/` | Executes queries against raw events table; contains SQL definitions with unbounded results and the retention cohort bug | medium |
| Cache Utility | `internal/cache/` | Unused TTL cache that can be wired into the service layer; needs eviction improvements | small |
| API Entry Point | `cmd/analytics-api/` | Server wiring; needs cache initialization and dependency injection | small |
| Dashboard Frontend | `apps/dashboard/` | Monolithic query, no progressive loading, no downsampling, cache-busting time range | large |
| Database Schema | `migrations/` | Defines events table; no materialized views currently | small (if views added) |

### Key Change Points
| Location | What Changes | Why |
|----------|-------------|-----|
| `internal/analytics/service.go:GetOverviewPage` | Parallelize 8 sequential chart queries using errgroup | Sequential execution is the root cause of the 8-10s load time; parallelizing converts latency from sum to max of individual queries |
| `internal/analytics/service.go:loadChart` | Integrate caching (check cache before ClickHouse, store after) | Eliminates redundant ClickHouse queries for repeated dashboard loads within TTL window |
| `internal/clickhouse/queries.go:user_retention_cohort` | Fix bug: 5 SQL params but only 3 passed | One of 8 charts has likely never worked; must be fixed as part of overall optimization |
| `internal/clickhouse/queries.go` (all queries) | Add result limits; coarsen time buckets where appropriate | Unbounded result sets (100k+ rows possible) waste bandwidth and memory |
| `internal/clickhouse/client.go:ExecuteQuery` | Support variable parameter counts for different query shapes | Currently hardcoded to 3 params; retention cohort needs 5 |
| `cmd/analytics-api/main.go` | Initialize cache and inject into analytics service | Cache utility exists but is not wired in |
| `apps/dashboard/src/pages/Overview.tsx` | Switch to per-chart queries with progressive loading | Monolithic query blocks rendering until the slowest chart completes |
| `apps/dashboard/src/pages/Overview.tsx:26-27` | Quantize time range to prevent Apollo cache busting | Millisecond-precision timestamps create unique cache keys every load |
| `apps/dashboard/src/components/Chart.tsx:50-55` | Add data downsampling before rendering (cap at ~200-500 points) | 10k+ SVG elements per chart causes 2-5 second rendering times |
| `apps/dashboard/src/components/ChartGrid.tsx` | Add lazy loading for below-fold charts | All 8 charts rendering simultaneously compounds performance issues |

### Dependencies
- **ClickHouse cluster:** Single data source for all chart data. Parallelizing queries multiplies concurrent load 8x per request. Connection pool must be explicitly configured.
- **gqlgen framework:** Resolver architecture constrains how parallelization can be implemented. Current pattern delegates to service layer, which is the right place for parallelization. Field-level resolvers are an alternative but require gqlgen configuration changes.
- **Apollo Client:** Frontend caching and query management. Switching from monolithic to per-chart queries changes the network pattern from 1 request to 8 requests. Apollo supports this natively via the existing `chartData` query.
- **Recharts rendering model:** SVG-based rendering where data point count directly correlates to render time. Cannot be changed without switching libraries; instead, data must be downsampled before rendering.
- **clickhouse-go/v2 driver:** Connection pooling behavior and query timeout settings come from this driver. Default pool size is unknown and may need explicit configuration for parallel query workloads.

## Constraints
- **GraphQL API contract must be backward-compatible:** Unknown consumers may exist. Changes must be additive (new fields/queries), not breaking (removing/renaming fields). Source: requirements scope exclusions, confirmed by all three analyses.
- **2-second target is wall-clock from user perspective:** Includes network latency, backend processing, and frontend rendering. Not just API response time. Percentile (P50/P95/P99) is undefined. Source: requirements.
- **No existing test coverage:** Zero test files across all modules (backend and frontend). Any changes are deployed without regression safety nets. Source: system analysis, verified by codebase investigation.
- **ClickHouse schema changes may be out of scope:** Materialized views and secondary indexes are infrastructure-level changes marked as "not included (to be confirmed)." Source: requirements scope.
- **Skip-and-continue error handling must be preserved:** Failed charts should not fail the entire page (`service.go:33-36`). This is a deliberate resilience pattern. Source: code, confirmed by system and constraints analyses.
- **No existing concurrency patterns in codebase:** The codebase has no goroutines, errgroups, or channels anywhere. Parallelization introduces a new pattern with no existing examples to follow. Source: system analysis.
- **No downtime for deployment:** Implied by production system serving 50k DAU. Changes should be deployable without extended downtime. Source: constraints analysis.

## Risks

| Risk | Likelihood | Impact | Mitigation Direction |
|------|-----------|--------|---------------------|
| Parallel ClickHouse queries overwhelm connection pool or cluster | medium | high | Configure explicit connection pool size; add concurrency limiter (semaphore); load test before rollout |
| Cache thundering herd (concurrent requests bypass cache simultaneously) | high | medium | Use `sync/singleflight` to deduplicate in-flight queries for the same cache key |
| Introducing concurrency bugs in untested code (race conditions, deadlocks) | medium | high | Use errgroup for structured concurrency; add tests for parallel path; run Go race detector |
| Data downsampling hides important operational signals (error spikes) | medium | medium | Make downsampling configurable per chart type; keep finer granularity for operational charts like error_rate |
| Naive cache eviction fails under load (silently stops accepting entries) | medium | medium | Replace with LRU eviction or size-bounded cache; add cache hit/miss monitoring |
| Retention cohort bug fix introduces an expensive new query into the parallel set | medium | low | Fix the bug but monitor query cost; consider whether the self-join query needs optimization |

## Candidate Solution Directions

- **Minimal (backend parallelization only):** Parallelize the 8 sequential queries using errgroup, fix the retention cohort bug, configure connection pool. This is the mathematical minimum to achieve the 2-second target for large tenants. Trade-off: may not reach 2s for the very largest tenants without caching; does not address frontend rendering bottleneck. Appropriate when: risk tolerance is low and scope must be minimized.
- **Safe (backend + caching + frontend cache fix):** Add backend parallelization, integrate the existing cache utility (with improved eviction), and fix the frontend cache-busting time range. Trade-off: more scope but addresses the three most impactful bottlenecks (sequential queries, no server cache, no client cache). Appropriate when: 2-second target must be reliably met across tenant sizes.
- **Systematic (full stack optimization):** All of the above plus frontend progressive loading (per-chart queries), data downsampling in Chart.tsx, lazy loading of below-fold charts. Trade-off: largest scope, touches all three layers, but produces the most complete solution and best perceived performance. Appropriate when: the goal is not just meeting the 2-second target but creating a genuinely fast, modern dashboard experience.

## Resolved Contradictions

- **Thundering herd severity:** The product analysis treats concurrent page loads as an "edge case," while the constraints analysis rates cache thundering herd as high likelihood / medium impact. **Resolution:** At 50,000 DAU, concurrent loads by users in the same tenant are a near-certainty during business-day peaks, not an edge case. The constraints analysis calibration is more accurate. The thundering herd is listed as a mandatory edge case in the synthesis.
- **Minimum viable outcome scope:** The product analysis states backend parallelization alone could be the MVO. The system analysis implies caching is also necessary for the largest tenants. **Resolution:** Both are correct at different tenant sizes. Backend parallelization is the absolute minimum (without it, the target is mathematically unachievable). Caching is likely needed in addition for the largest tenants. The synthesis preserves both views: parallelization is the floor, caching is the likely next step.
- **Retention cohort bug -- in scope or separate?** The product analysis doesn't specifically address whether the bug fix is in scope. The system analysis identifies it as a confirmed bug. The constraints analysis notes fixing it as part of optimization is lower risk. **Resolution:** Fixing the bug is practically required because any work touching the query layer will encounter it. It should be included in scope as a small, necessary fix rather than tracked separately, since the bug would cause confusion during parallelization testing.

## Remaining Open Questions

1. **What is the acceptable data staleness for cached dashboard data?** The error_rate chart monitors real-time operational health and may need a shorter TTL (30s-1min) than historical charts like retention (5-10min). No freshness SLA exists. This affects caching strategy design.
2. **Are there other consumers of the GraphQL API?** This determines whether the API contract constraint is hard or soft. The `overviewPage` query is dashboard-specific, but `chartData` could be shared.
3. **What is the ClickHouse cluster topology (single node vs. distributed)?** Parallelizing 8 queries per request against a single node has different capacity implications than against a cluster.
4. **Is the 2-second target P50, P95, or P99?** P50 under 2s is achievable with less effort than P99 under 2s. The difference significantly affects which optimizations are necessary.
5. **Does the clickhouse-go driver support configurable connection pooling?** If queries are parallelized to 8 concurrent per request, the pool must handle 8 x concurrent_users connections.
6. **What is the network transfer size for large chart payloads?** Unbounded result sets could produce multi-megabyte JSON responses. Is gzip/compression enabled on the API?
7. **Is there a query timeout at the ClickHouse driver level?** If a query hangs for 30 seconds, does the Go context cancel it?

## Critique Review

The synthesis was reviewed by an independent critic. The critic's assessment follows.

**Verdict: CONSISTENT**

The critic found no FAIL scores and at most minor weaknesses. Key findings:

- **Goal fidelity:** PASS. The synthesized goal accurately captures intent from product analysis and original requirements, correctly synthesizing the 2-second target, wall-clock measurement, and 50k DAU context.
- **Scenario coverage:** PASS. Primary scenario is well-structured with current vs. target states. All mandatory edge cases from the analyses are preserved, including the thundering herd escalation from edge case to high-likelihood scenario.
- **System scope accuracy:** PASS. Modules, change points, and dependencies match the system analysis findings. Line numbers and file paths are preserved.
- **Constraint completeness:** PASS. All constraints from all three sources are present and deduplicated. The "no concurrency patterns" constraint from the system analysis is included.
- **Risk calibration:** PASS. Risks are consolidated and calibrated consistently. The thundering herd is correctly rated high likelihood based on the constraints analysis calibration.
- **Contradiction resolution:** PASS. Three contradictions identified and resolved with explicit reasoning. Each resolution states what each source said and why one view was chosen.
- **Assumption honesty:** WEAK. The assumption that "caching is likely needed for largest tenants" beyond parallelization is presented with moderate confidence but is not verified -- it depends on actual query performance post-parallelization. The synthesis correctly frames this as "likely" rather than certain.
- **Information preservation:** PASS. Key findings from all three analyses are preserved. The SummaryData type duplication, CORS wildcard, and frontend date formatting per data point are not in the synthesis but are minor technical observations that do not affect the task model.

**Minor observations:**
- The per-chart `TimeRange` configuration field (unused but present) could have been surfaced more prominently as a potential quick win for reducing data volumes.
- The `SummaryData` type duplication and CORS wildcard from the system analysis are omitted, but these are minor code-quality observations, not task-critical findings.

No revision was required.
