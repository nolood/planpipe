# Questions to Resolve Before Starting

## Critical (must answer before implementation)

### Measurement & Baseline
1. **Do we have APM/tracing in place?** (e.g., Datadog, New Relic, OpenTelemetry, Jaeger) -- If yes, pull traces for the overview page to see exactly where time is spent. If no, instrumenting is step zero.
2. **What does "8-10 seconds" measure?** Full page load (LCP)? Time to interactive? Time until last chart renders? We need a precise metric definition to know when we've hit < 2s.
3. **Is there a staging environment with production-like data volume?** Optimizing against a small dev dataset will give misleading results.

### Data & Queries
4. **How many charts are on the overview page, and what data does each one query?** (time series, aggregations, breakdowns by dimension, etc.)
5. **What time range does the default overview page cover?** (last 24h, 7 days, 30 days?) Longer ranges over raw data are exponentially more expensive.
6. **Are there materialized views or rollup tables in ClickHouse already, or is the API querying raw event tables?**
7. **How large are the underlying ClickHouse tables?** (row counts, data size on disk)

### Architecture
8. **Is there any caching layer between the API and ClickHouse?** (Redis, in-memory, CDN, HTTP cache headers on GraphQL responses)
9. **How many GraphQL queries does a single page load trigger?** (one batched query? one per chart? nested queries?)
10. **What charting library does the frontend use?** (Recharts, Chart.js, ECharts, D3, Highcharts, etc.) -- each has different performance profiles and downsampling capabilities.

### Constraints
11. **How fresh does the data need to be?** Can we serve data that is 1 minute old? 5 minutes? 1 hour? This determines caching TTL viability.
12. **Are there any upcoming product changes to the dashboard?** (No point optimizing a page that's being redesigned next quarter.)
13. **Is the 2-second target for cold load or warm/cached load?** Very different engineering approaches.

## Important (answer during implementation)

14. **What is the frontend framework and build setup?** (React/Next.js/Vite/Webpack -- affects code-splitting strategy)
15. **Is the GraphQL API using a framework with built-in DataLoader support?** (Apollo Server, Mercurius, etc.)
16. **Do we have performance budgets or alerting set up?** If not, we should add them so this doesn't regress.
17. **What is the current p50/p95/p99 for the overview page load?** (Is 8-10s the worst case or the median?)
18. **Are there specific charts that are slower than others, or is it uniform?**
