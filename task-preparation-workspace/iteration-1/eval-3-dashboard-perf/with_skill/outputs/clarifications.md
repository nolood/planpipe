# Clarifications Needed

The readiness review returned **NEEDS_CLARIFICATION**. The following gaps and questions must be resolved before this task can proceed to deep analysis.

## Blocking Gaps

1. **Which charts are slow:** "Some of the charts" is too vague for targeted analysis. The next stage needs to know whether this is a handful of specific chart types (e.g., time-series, funnels, heatmaps) or a systemic issue across every chart on the page, because the root cause and fix approach differ significantly.

2. **Scope of permissible changes:** Whether ClickHouse-level changes (materialized views, schema adjustments, pre-aggregation tables) are within scope fundamentally shapes what solutions are viable. If only application-layer changes (API code, frontend code) are allowed, the optimization space is much narrower.

3. **Measurement definition:** Whether the 2-second target is per individual chart or for the total page load time changes the difficulty and approach substantially. A single chart at 2s is very different from all charts loaded within 2s.

## Unsafe Assumptions to Verify

- The assumption that the problem is query/rendering inefficiency rather than ClickHouse infrastructure capacity constraints should be verified with basic health checks.
- The assumption that there is no hard deadline should be confirmed, as it shapes whether the analysis should prioritize quick wins or thorough optimization.

## Clarification Questions

1. **Which specific charts on the main overview page are slow?** Is it all charts, or a specific subset (e.g., time-series charts, funnel charts, charts querying specific date ranges or large tables)?

2. **Is the 2-second performance target per individual chart, or for the entire page to finish loading all charts?**

3. **Are ClickHouse-level changes (adding materialized views, pre-aggregation tables, schema modifications) within scope**, or is this work limited to application-layer changes in the API and frontend code?

4. **Is there a deadline or time constraint for this work** (e.g., tied to a release, customer commitment, quarterly goal)?

5. **Does the GraphQL API in `services/analytics-api/` serve other consumers besides the dashboard?** If so, are there API contract constraints that would limit changes to query patterns or response shapes?
