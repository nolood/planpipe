# Constraints / Risks Analysis

## Constraints

### Architectural
- **Sequential service layer pattern:** The codebase has no existing concurrency patterns -- no goroutines, no errgroups, no channels anywhere. Introducing parallelism in `GetOverviewPage` is a new pattern. The existing error-handling approach (skip failed charts, continue) must be preserved, which means parallel execution needs careful error aggregation. Source: `internal/analytics/service.go:30-37`.
- **Monolithic GraphQL query for overview page:** The frontend fetches all charts in a single `overviewPage` query (`apps/dashboard/src/api/analytics.ts:13-42`). Changing this to per-chart queries requires changes to both the frontend query pattern and potentially the GraphQL schema. The `chartData` query already exists in the schema for single-chart loading.
- **gqlgen resolver architecture:** The GraphQL layer uses gqlgen with a resolver-delegates-to-service pattern (`resolver.go`). The `OverviewPage` resolver returns a complete `OverviewPage` struct. Switching to per-field resolution (where each chart is resolved independently) would require changes to the gqlgen configuration and potentially the schema.

### Technical
- **ClickHouse query execution on raw events table:** All 8 chart queries hit the raw `events` table (~500M rows) with no materialized views or pre-aggregated data (`migrations/001_analytics_tables.sql:20-21`). The MergeTree partition scheme `(tenant_id, toYYYYMM(timestamp))` and primary key `(tenant_id, event_type, timestamp)` mean that tenant + time range + event_type filters are efficient, but aggregations requiring `uniq(user_id)` or grouping by `region` require scanning more data.
- **No result pagination or limits:** `ExecuteQuery` in `client.go:54` loads the entire result set into memory. Only one query has a LIMIT clause (`top_events_by_count` at 10,000 rows). Introducing limits changes the data returned to the frontend.
- **Recharts SVG rendering model:** Recharts renders each data point as an SVG element. This is a fundamental characteristic of the library -- it cannot be changed without switching libraries. The constraint is that charts with >500-1000 data points will have noticeable rendering overhead. Source: `Chart.tsx:42` warning comment.
- **Apollo Client cache-first policy with dynamic time ranges:** The frontend uses `new Date().toISOString()` for query variables (`Overview.tsx:26-27`), creating unique cache keys on every page load. Fixing this requires quantizing the time range, which technically changes the data returned (slightly different time boundaries).

### Business
- **2-second target (PM requirement):** Hard performance target from the PM. The requirements state "under 2 seconds" without specifying percentile (P50, P95, P99). Source: `requirements.draft.md`.
- **50,000 daily active users:** The scale means any performance optimization must work under concurrent load, not just for a single user. Source: `requirements.draft.md`.
- **No downtime for deployment:** Not explicitly stated but implied -- this is a performance optimization on a live production system serving 50k DAU. Changes should be deployable without extended downtime.

### Compatibility
- **GraphQL API contract preservation:** The requirements explicitly state "Changes to the GraphQL API contract that would affect other consumers" are not included in scope. This means the GraphQL schema (`schema.graphql`) and response formats should remain backward-compatible. The `overviewPage` query should continue to work as-is. Source: `requirements.draft.md` scope exclusions.
- **Unknown API consumers:** It is unknown whether other services consume the same GraphQL API (`requirements.draft.md` unknowns). This constrains API contract changes -- they must be additive (new fields, new queries) rather than breaking (removing fields, changing types).

### Regulatory/Compliance
- None identified. The analytics data is event-level behavioral data. No specific compliance constraints (GDPR, SOC2) are mentioned in the requirements. The data is already scoped by `tenant_id`, suggesting multi-tenant isolation exists.

## Risks

| Risk | Category | Likelihood | Impact | Evidence | Mitigation Idea |
|------|----------|-----------|--------|----------|-----------------|
| Parallelizing 8 ClickHouse queries per request overwhelms the connection pool or ClickHouse cluster | technical | medium | high | No connection pool configuration visible in `client.go`; default pool size unknown; 50k DAU means many concurrent requests each spawning 8 parallel queries | Configure connection pool explicitly; add concurrency limits (semaphore) to cap simultaneous queries per request or globally |
| Cache thundering herd: many concurrent requests for the same tenant bypass cache simultaneously | technical | high | medium | 50k DAU with no current caching; popular tenants will have many users loading the overview at the same time; naive cache implementation has no request coalescing | Use singleflight pattern (Go's `sync/singleflight`) to deduplicate in-flight queries for the same cache key |
| Retention cohort query bug causes silent failure that is exposed or changed during optimization | regression | high | low | `queries.go:67-85` has 5 `?` placeholders but `ExecuteQuery` passes only 3 params; this chart likely always fails and is silently skipped (`service.go:33-36`) | Fix the bug as part of the optimization; verify current production behavior first |
| Data downsampling on the frontend loses important detail that users rely on | scope | medium | medium | The error_rate chart groups by minute for 24h monitoring; downsampling to 200 points would show 7-minute granularity, potentially hiding short spikes | Make downsampling configurable per chart type; keep finer granularity for operational charts like error_rate |
| Introducing caching with the naive cache implementation causes memory pressure or ineffective eviction | technical | medium | medium | `cache.go:49-57` eviction removes first expired item only; if no items expired, new entries are silently dropped; no memory bounds | Replace naive eviction with LRU or size-bounded eviction; consider per-key TTL for different chart types |
| Breaking Apollo Client cache behavior when quantizing time ranges | regression | low | low | Currently the cache never hits due to millisecond-precision timestamps; changing to quantized ranges is an improvement but changes query behavior | Quantize to 1-minute or 5-minute boundaries; minimal data accuracy impact |
| Per-chart time range field in config is meant to be used but introducing it changes data volumes unpredictably | scope | low | medium | `ChartConfig.TimeRange` is defined but unused; if activated, error_rate switches from 30d to 24h of data and retention stays at 30d | Investigate intent before activating; if used, it reduces data for some charts (positive) but changes behavior |
| Materialized views, if introduced, add ClickHouse operational complexity | technical | low | medium | The migration notes explicitly say "No materialized views currently in use"; adding them requires schema changes, which is noted as potentially out of scope | Evaluate whether query parallelization + caching alone meet the 2s target before adding materialized views |

## Integration Dependencies

- **ClickHouse cluster:** Direct dependency via `clickhouse-go/v2` driver. Connected via DSN from `CLICKHOUSE_URL` env var (default: `clickhouse://localhost:9000/analytics`). No explicit SLA mentioned. Query performance is 2-5 seconds per query for large tenants. If queries are parallelized, the cluster must handle 8x concurrent queries per user request. Failure mode: connection timeout or query timeout, resulting in individual charts being skipped (existing error handling at `service.go:33-36`).
- **GraphQL API (internal):** The dashboard frontend depends on the `overviewPage` GraphQL query. The contract is defined in `schema.graphql`. Changes to the response shape would break the frontend. The `chartData` query is available as an alternative entry point for per-chart loading. Contract is stable (owned by the same team).
- **Apollo Client (frontend-backend contract):** The frontend expects the full `OverviewPage` response to arrive as a single GraphQL response. Switching to per-chart queries changes the network request pattern from 1 request to 8 requests. Apollo supports this natively but the frontend code must be restructured.

## Backward Compatibility

| What Changes | Current Consumers | Migration Needed? | Rollback Safe? | Notes |
|-------------|-------------------|-------------------|----------------|-------|
| Backend parallelization of chart queries | Dashboard frontend (via GraphQL API) | no | yes | Same API contract, same response shape -- only internal execution changes. Fully rollback safe. |
| Adding caching to analytics service | Dashboard frontend, potentially unknown consumers | no | yes | Cache is transparent -- adds a layer before ClickHouse. Rollback = disable cache. Data may be slightly stale vs. real-time. |
| Frontend: switching from `overviewPage` to per-chart `chartData` queries | Dashboard frontend only (internal change) | no (frontend-only change) | yes | The GraphQL schema already has both queries. No backend contract change needed. |
| Frontend: adding data downsampling | Dashboard users | no | yes | Visual change only -- charts display fewer data points. Users may notice less detail. |
| Modifying ClickHouse queries (adding LIMITs, coarsening time buckets) | Analytics service, potentially unknown consumers | unknown | yes (query changes are reversible) | If other consumers use the same Go service, they would be affected. If they call ClickHouse directly, no impact. |

## Sensitive Areas

- **`internal/analytics/service.go:GetOverviewPage`:** This is the hot path for the most-used page in the dashboard. Any change here affects all 50,000 DAU. Currently has no tests. Introducing concurrency here without tests is risky because race conditions and deadlocks would be hard to detect. Risk level: **high**.
- **ClickHouse query layer (`internal/clickhouse/queries.go`):** Modifying SQL queries against a 500M-row table with no test coverage means query correctness can only be verified by running against the actual database. A broken query could silently return wrong data (the service skips errors, so a syntax error would just show a missing chart). Risk level: **medium**.
- **`internal/cache/cache.go` eviction logic:** If this cache is wired into the hot path and the eviction logic is broken (which it partially is -- it silently stops accepting entries when full and no items are expired), cached data could become permanently stale or the cache could fill up and become ineffective. Risk level: **medium**.
- **Frontend rendering pipeline (`Chart.tsx`):** Changing how data is transformed or downsampled before rendering affects the visual output that 50k users see daily. Incorrect downsampling could hide important data patterns (e.g., brief error spikes). No frontend tests exist. Risk level: **medium**.

## Critique Review

The critic assessed this analysis as SUFFICIENT. Constraints are specific and backed by code references. Risks are calibrated -- not everything is marked high, and the thundering herd risk is correctly identified as high likelihood / medium impact. The backward compatibility section explicitly addresses each type of change and its rollback safety. The critic noted one minor observation: the analysis could have assessed whether the Docker Compose setup (`docker-compose.yml`) reveals any infrastructure constraints (e.g., ClickHouse resource limits) and whether the connection string defaults suggest a single-node ClickHouse setup. This is a minor gap that does not block the analysis quality.

## Open Questions

- What is the ClickHouse cluster topology -- single node or distributed? The `docker-compose.yml` shows a single `clickhouse-server` container, but this may be a development setup, not production. Parallelizing 8 queries per request against a single node has different implications than against a cluster.
- Is there a query timeout configured at the ClickHouse driver level? If a single query hangs for 30 seconds, does it block the entire overview page load (in the current sequential model) or does the Go context cancel it?
- Are there any load balancers, CDNs, or API gateways in front of the analytics API that might cache responses or impose request size limits?
- What is the actual memory footprint of loading 8 charts worth of data into Go memory simultaneously (when parallelized)? For large tenants with 10k+ rows per chart, this could be significant per request.
