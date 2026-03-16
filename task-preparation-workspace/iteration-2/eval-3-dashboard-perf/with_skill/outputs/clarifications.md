# Clarifications Needed

> Task: Reduce analytics dashboard chart load time from 8-10s to under 2s on the main overview page
> Verdict: NEEDS_CLARIFICATION
> Open items: 3 blocking gaps, 7 unknowns, 6 assumptions to verify

## Blocking Gaps

1. **Slow chart identification:** "Some of the charts" are slow, but we do not know which ones. Without identifying the specific slow charts, investigation cannot be focused effectively — different chart types may have entirely different bottlenecks (heavy aggregation queries vs. rendering thousands of data points).
2. **Measurement methodology:** The 8-10 second figure has no defined measurement method. Whether this is time-to-first-byte from the API, time-to-render in the browser, or a subjective user observation determines which part of the pipeline to optimize first. Without this, analysis risks targeting the wrong layer.
3. **Profiling data availability:** If APM or performance monitoring tools are already capturing timing data for the API and frontend, that data should drive the analysis. Starting an investigation from scratch when profiling data already exists would waste effort.

## Open Unknowns

1. **Charting library:** Which charting library does the frontend use (Recharts, Chart.js, ECharts, Highcharts, D3, other)? This determines what rendering optimization strategies are available.
2. **Data payload sizes:** How much data is being sent from the API to the frontend per chart request? Are we talking kilobytes or megabytes?
3. **Caching layers:** Are there any existing caching mechanisms (Redis, CDN, application-level) between ClickHouse and the frontend? If so, are they configured and working?
4. **Default time range:** What time range does the overview page query by default (last hour, day, week, month)? This directly affects query volume and response size.
5. **Server-side aggregation:** Does the API perform aggregation before sending data to the frontend, or does it pass through granular/raw data for the frontend to aggregate?
6. **Regression vs. chronic:** Is this a recent regression or has the dashboard always been this slow? If regression, approximately when did it start?
7. **ClickHouse query patterns:** Are the current queries using materialized views and proper indices, or are they performing full table scans?

## Assumptions to Verify

1. **Performance target definition:** We are assuming the 2-second target applies to typical/p95 user experience, not just best-case. Is this correct, or is there a specific percentile target?
2. **ClickHouse infrastructure health:** We are assuming the ClickHouse cluster itself is healthy and not resource-constrained. Has anyone checked cluster resource utilization recently?
3. **Both layers are changeable:** We are assuming changes to both `apps/dashboard/` (frontend) and `services/analytics-api/` (backend) are acceptable. Are there any deployment freezes, team ownership boundaries, or other constraints?
4. **Infrastructure changes scope:** We are assuming ClickHouse schema changes (adding materialized views, indices) might be acceptable. Is this the case, or should optimization be limited to application code?
5. **GraphQL is the only data path:** We are assuming all chart data flows through the GraphQL API. Is there any direct data access, WebSocket streaming, or other data path?
6. **Test coverage:** We are assuming there are automated tests that can catch regressions from performance-related changes. Is this the case?

## Questions for the User

1. Which specific charts on the overview page are slow? Is it all of them, or only certain chart types?
2. How was the 8-10 second load time measured — browser DevTools, a monitoring tool, user reports, or something else?
3. Is this a recent regression or has the dashboard always performed this way? If a regression, when did it start?
4. Is there any APM or performance monitoring in place (Datadog, New Relic, Grafana, browser performance traces) that already captures timing data?
5. Are there any caching layers currently between ClickHouse and the frontend (Redis, CDN, in-memory)?
6. What default time range does the overview page query?
7. Are ClickHouse infrastructure changes (materialized views, new indices, schema changes) on the table, or should we limit to application-level optimizations?
8. Are there any constraints on which layer (frontend vs. backend) can be changed — team ownership, deployment schedules, feature freezes?
9. What is the specific performance target percentile — p50, p95, p99, or average?
10. What charting library does the frontend use?
