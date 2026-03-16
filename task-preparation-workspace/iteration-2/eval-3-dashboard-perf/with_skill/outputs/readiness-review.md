# Readiness Review

## Verdict: NEEDS_CLARIFICATION

## Criteria Evaluation

| Criterion | Score | Reasoning |
|-----------|-------|-----------|
| Goal clarity | PASS | The goal is unambiguous: reduce chart load time on the main overview page from 8-10s to under 2s. "Done" is clearly measurable. |
| Problem clarity | PASS | The problem is well-articulated — slow charts degrade the experience for 50k DAU, the PM has set a concrete target, and the suspected bottleneck areas are identified. |
| Scope clarity | WEAK | The scope covers both frontend and backend, which is reasonable, but it is unclear which specific charts are slow, whether all charts on the page are in scope or only the slow ones, and whether infrastructure-level changes (e.g., ClickHouse materialized views, new indices) are in or out of scope. The edges are blurry. |
| Change target clarity | PASS | The affected areas are clearly identified: `apps/dashboard/` for frontend and `services/analytics-api/` for the API layer. The data path (ClickHouse -> GraphQL -> frontend) is known. |
| Context sufficiency | WEAK | We know the architecture at a high level, but critical details are missing: which charting library is used, what the payload sizes are, whether caching exists, what the query patterns look like, and whether APM data is available. Analysis is possible but would be working with significant blind spots. |
| Ambiguity level | WEAK | Several ambiguities remain: how the 8-10s is measured (TTFB vs. render vs. perceived), whether this is a regression or a long-standing issue, what default time ranges are queried, and whether server-side aggregation exists. These don't fully block analysis but could lead it in the wrong direction. |
| Assumption safety | WEAK | The assumption that ClickHouse infrastructure is healthy is risky — if the cluster is resource-constrained, the entire optimization strategy changes. The assumption about the charting library being standard is reasonable but unverified. The assumption that both frontend and backend changes are acceptable needs confirmation. |
| Acceptance possibility | PASS | Success is clearly measurable: chart load time under 2 seconds. This can be verified with performance testing and monitoring. |

## Summary

The task has a clear goal and measurable acceptance criteria, which is a strong foundation. However, there are 4 WEAK scores across scope clarity, context sufficiency, ambiguity level, and assumption safety. The combination of not knowing which specific charts are slow, how performance is currently measured, whether this is a regression, and whether critical infrastructure assumptions hold means that deep analysis would be operating with too many blind spots. A round of targeted clarification questions would significantly improve the quality of the next stage's work.

## Blocking Gaps

- No specific identification of which charts are slow — "some of the charts" is too vague to focus investigation effectively
- No clarity on how the 8-10 second figure was measured — without knowing whether this is TTFB, time-to-interactive, or a subjective observation, any analysis risks optimizing the wrong part of the pipeline
- No information on whether existing profiling/APM data is available — if it exists, it should inform the analysis rather than starting from scratch

## Unsafe Assumptions

- ClickHouse cluster health: If the cluster is actually resource-constrained (CPU, memory, disk I/O), query-level optimizations may not be sufficient, and the problem requires infrastructure work that is currently listed as out of scope
- Both frontend and backend changes are acceptable: If there are deployment constraints, feature freezes, or team ownership boundaries that limit where changes can be made, the optimization strategy is materially different

## Recommended Clarification Questions

1. Which specific charts on the overview page are slow? Is it all of them, or only certain chart types (e.g., time series, aggregation tables, heatmaps)?
2. How was the 8-10 second load time measured? Is this from browser DevTools network timing, a performance monitoring tool, user reports, or something else?
3. Is this a recent regression (i.e., it used to be fast and got slower) or has the dashboard always been this slow? If it's a regression, when did it start?
4. Is there any existing APM or performance monitoring in place (e.g., Datadog, New Relic, Grafana) that provides profiling data for the API or frontend?
5. Are there any caching layers currently in place between ClickHouse and the frontend (Redis, CDN, in-memory cache in the API)?
6. What default time range does the overview page query (last hour, last day, last 7 days, last 30 days)?
7. Are infrastructure-level changes to ClickHouse (adding materialized views, new indices, schema changes) acceptable, or should the optimization be limited to query patterns and application code?
8. Are there deployment or team ownership constraints that would limit changes to only the frontend or only the backend?
