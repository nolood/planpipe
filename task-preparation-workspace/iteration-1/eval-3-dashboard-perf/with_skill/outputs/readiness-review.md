# Readiness Review

## Verdict: NEEDS_CLARIFICATION

## Criteria Evaluation

| Criterion | Score | Reasoning |
|-----------|-------|-----------|
| Goal clarity | PASS | The goal is unambiguous: reduce chart load times on the main overview page to under 2 seconds. The target is quantified and the scope of the measurement is clear. |
| Problem clarity | PASS | The problem is well-articulated — slow charts hurt 50k DAU, the data path is identified, and the business motivation (user trust, engagement) is stated. |
| Scope clarity | WEAK | The scope covers "main overview page" but it is unclear which specific charts are problematic and whether infrastructure-level changes (ClickHouse schema, materialized views) are in or out. The "Out of Scope" section is essentially "TBD." Without knowing whether ClickHouse-level changes are permitted, the next stage cannot make informed architectural decisions. |
| Change target clarity | PASS | The system areas are clearly identified: `apps/dashboard/` for the frontend, `services/analytics-api/` for the API layer, and ClickHouse as the data source. The data path is well-defined. |
| Context sufficiency | WEAK | The context is thin on key details that would inform analysis: no information about existing caching, charting library, payload sizes, number of queries per page load, or whether APM/tracing exists. These are listed as unknowns, which is honest, but the density of unknowns is high enough that the next stage would need to do significant discovery before analysis. |
| Ambiguity level | WEAK | "Some of the charts" is critically ambiguous — it could mean 2 charts or 20. Whether the 2-second target applies to each chart individually or to total page load is unstated. Whether the GraphQL API has other consumers (constraining changes) is unknown. These ambiguities could send analysis in very different directions. |
| Assumption safety | WEAK | The assumption that "the problem is query/rendering inefficiency, not raw compute limits" is risky — if ClickHouse is actually at capacity, the entire optimization approach changes. The assumption about "no hard deadline" matters for whether quick-fix vs. proper-fix approaches should be considered. Both are flagged, which is good, but unverified. |
| Acceptance possibility | PASS | The acceptance criterion is clear and measurable: chart load time under 2 seconds, measured as wall-clock time. This can be verified with standard performance testing. |

## Summary

The preparation demonstrates honest and thorough thinking — unknowns are clearly labeled as unknowns, assumptions are flagged rather than hidden. However, the combination of 4 WEAK scores pushes this to NEEDS_CLARIFICATION. The core issue is that the ambiguity around "which charts" and "what scope of changes is allowed" would force the next stage to branch into multiple analysis paths without knowing which one is relevant. A small amount of clarification would collapse this uncertainty and make the task cleanly analyzable.

## Blocking Gaps

- **Which charts are slow:** "Some of the charts" is too vague for targeted analysis. The next stage needs to know whether this is a handful of specific chart types or a systemic issue across the page, because the root cause and fix differ significantly.
- **Scope of permissible changes:** Whether ClickHouse-level changes (materialized views, schema adjustments, pre-aggregation tables) are in scope fundamentally shapes what solutions are viable. If only application-layer changes are allowed, the optimization space is much narrower.
- **Measurement definition:** Whether the 2-second target is per-chart or total page load time changes the difficulty and approach substantially.

## Unsafe Assumptions

- **"The problem is query/rendering inefficiency, not raw compute limits"** — If ClickHouse is actually resource-constrained, no amount of query optimization or frontend tuning will hit the 2-second target. This assumption should be verified with basic infrastructure health checks before committing to an optimization approach.
- **"No hard deadline"** — If there is actually a deadline (e.g., tied to a quarterly review, a customer escalation), the analysis should prioritize quick wins over thorough optimization. This shapes the entire planning approach.

## Recommended Clarification Questions

1. Which specific charts on the main overview page are slow? Is it all charts, or a specific subset (e.g., time-series charts, funnel charts, charts with specific date ranges)?
2. Is the 2-second performance target per individual chart, or for the entire page to finish loading all charts?
3. Are ClickHouse-level changes (adding materialized views, pre-aggregation tables, schema modifications) within scope, or is this limited to application-layer changes in the API and frontend?
4. Is there a deadline or time constraint for this work (e.g., tied to a release, customer commitment, quarterly goal)?
5. Does the GraphQL API in `services/analytics-api/` serve other consumers besides the dashboard, and if so, are there API contract constraints?
