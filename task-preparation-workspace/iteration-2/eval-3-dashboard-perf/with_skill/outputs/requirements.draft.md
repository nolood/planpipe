# Requirements Draft

## Goal

Reduce the load time of charts on the analytics dashboard main overview page from 8-10 seconds to under 2 seconds.

## Problem Statement

The analytics dashboard's main overview page has degraded performance: some charts take 8-10 seconds to load, which is unacceptable for the ~50k daily active users who rely on it. The PM has set a target of under 2 seconds. The suspected bottleneck is either in the ClickHouse query layer (via GraphQL API) or in the frontend rendering of large datasets. The slow experience directly impacts user productivity and trust in the analytics platform.

## Scope

- Investigate and fix performance of chart loading on the main overview page of the analytics dashboard
- Both the frontend rendering path (`apps/dashboard/`) and the backend query/API layer (`services/analytics-api/`) are in scope
- The ClickHouse query performance and the GraphQL API response handling are in scope
- The target metric is page load time for charts: from 8-10s down to under 2s

## Out of Scope

- Dashboard pages other than the main overview page (unless they share the same slow code paths)
- Changes to the ClickHouse schema or cluster infrastructure (to be confirmed)
- Adding new features or charts to the dashboard
- Changes to the GraphQL schema contract (to be confirmed)
- User authentication or authorization performance

## Constraints

- Performance target: chart load time under 2 seconds (PM requirement)
- ~50k daily active users — solution must work at this scale without degradation
- Must not break existing dashboard functionality or data accuracy
- Dashboard code lives in `apps/dashboard/`
- API code lives in `services/analytics-api/`
- Data source is ClickHouse, accessed via a GraphQL API

## Dependencies & Context

- ClickHouse database — query performance depends on table structure, materialized views, indices
- GraphQL API layer in `services/analytics-api/` — may have resolver-level inefficiencies, missing caching, or N+1 query patterns
- Frontend charting library (unknown which one) — rendering performance depends on data volume sent to the client and how the library handles it
- No information about existing caching layers (CDN, API-level, application-level)
- No information about whether server-side aggregation is already in place or if raw data is sent to the client

## Knowns

- Charts on the main overview page take 8-10 seconds to load
- The performance target is under 2 seconds
- The data flows from ClickHouse through a GraphQL API to the frontend
- Dashboard code is in `apps/dashboard/`
- API code is in `services/analytics-api/`
- There are approximately 50k daily active users
- The suspected bottleneck is either in the query layer or in frontend rendering of large datasets

## Unknowns

- Which specific charts on the overview page are slow (all of them, or only certain ones?)
- What charting library the frontend uses and its known performance characteristics
- How much data is being transferred per chart request (payload sizes)
- Whether there are existing caching mechanisms (Redis, CDN, in-memory) and if so, whether they are effective
- Whether the ClickHouse queries use materialized views, proper indices, or are scanning large tables
- What the GraphQL resolver structure looks like — single query per chart, batched, or N+1 pattern
- Whether the 8-10 second measurement is time-to-first-byte, time-to-render, or total perceived load time
- Whether server-side aggregation is performed or raw/granular data is sent to the client for aggregation
- What time ranges the overview page queries by default (last hour, day, week, month?)
- Whether there are any existing performance monitoring or APM tools in place to provide profiling data
- Whether the problem has worsened recently (regression) or has always been this slow

## Assumptions

- The 2-second target applies to the p95 or typical user experience, not just best-case scenario
- The dashboard and analytics-api are independently deployable services
- The ClickHouse cluster itself is healthy and not resource-constrained (the problem is in queries or data handling, not infrastructure)
- The GraphQL API is the only data path for the dashboard charts (no direct DB access from frontend)
- Changing query patterns and adding caching is acceptable — we are not restricted to frontend-only fixes
- The charting library is a standard one (e.g., Recharts, Chart.js, ECharts, Highcharts) and not a custom rendering engine
- Existing automated tests cover dashboard functionality and can catch regressions from performance changes
