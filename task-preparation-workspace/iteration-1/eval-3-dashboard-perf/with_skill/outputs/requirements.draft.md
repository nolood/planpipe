# Requirements Draft

## Goal

Reduce the load time of charts on the analytics dashboard's main overview page from the current 8-10 seconds to under 2 seconds.

## Problem Statement

The analytics dashboard's main overview page has degraded to 8-10 second load times for some charts, creating a poor experience for approximately 50,000 daily active users. The dashboard pulls data from ClickHouse through a GraphQL API layer, and the performance bottleneck is suspected to live in either the query layer (slow or unoptimized ClickHouse queries, inefficient GraphQL resolvers) or the frontend rendering layer (struggling to handle large datasets in the browser). The PM has set a target of under 2 seconds. This matters because a slow analytics dashboard undermines user trust in the data platform and likely reduces engagement — users who wait 10 seconds for a chart to load will stop checking dashboards.

## Scope

**Included:**
- Performance investigation and optimization of chart load times on the main overview page of the analytics dashboard
- The full data path: ClickHouse queries, GraphQL API resolvers/data layer (`services/analytics-api/`), and frontend chart rendering (`apps/dashboard/`)
- Specifically the "main overview page" — not every page in the dashboard

**Not included (to be confirmed):**
- Other dashboard pages beyond the main overview
- Changes to ClickHouse infrastructure (schema migrations, cluster scaling) — unclear if this is in scope
- Changes to the GraphQL API contract that would affect other consumers
- Mobile or non-web clients

## Out of Scope

To be determined — depends on clarification about whether infrastructure-level changes (ClickHouse schema, materialized views, cluster config) are acceptable, and whether other API consumers constrain the GraphQL contract.

## Constraints

- **Performance target:** Chart load times must be under 2 seconds (PM requirement)
- **Scale:** ~50,000 daily active users — solution must work at this scale
- **Existing architecture:** Data flows from ClickHouse through a GraphQL API to the frontend dashboard
- **Code locations:** Dashboard frontend in `apps/dashboard/`, API in `services/analytics-api/`

## Dependencies & Context

- **ClickHouse:** The underlying analytical database — query performance depends on table schemas, materialized views, indexes, and data volume
- **GraphQL API (`services/analytics-api/`):** The middleware layer between ClickHouse and the frontend — resolvers, data aggregation, and caching happen here
- **Dashboard frontend (`apps/dashboard/`):** Chart rendering, data transformation in the browser, possible client-side caching
- **Other API consumers:** Unknown whether other services or dashboards consume the same GraphQL API — changes to the API could have side effects

## Knowns

- Charts on the main overview page currently take 8-10 seconds to load
- The performance target is under 2 seconds
- Data flows from ClickHouse through a GraphQL API to a frontend dashboard
- The dashboard code lives in `apps/dashboard/`
- The API code lives in `services/analytics-api/`
- There are approximately 50,000 daily active users
- The team suspects the bottleneck is in the query layer or the frontend rendering layer

## Unknowns

- Which specific charts are slow (all of them, or a subset?)
- Where exactly the time is spent: ClickHouse query execution, GraphQL resolver processing, network transfer, frontend parsing/rendering, or a combination
- What volume of data is being returned to the frontend per chart (rows, payload size)
- Whether there is any caching in place today (query cache, CDN, API-level cache, client-side cache)
- What charting library the frontend uses and how it handles large datasets
- How many ClickHouse queries are triggered per page load (N+1 problem?)
- Whether the GraphQL API serves other consumers that could constrain changes
- What the current ClickHouse table schemas and query patterns look like
- Whether there are existing performance monitoring/tracing tools in place (APM, query logs)
- What "some of the charts" means — is it specific chart types, specific date ranges, specific data volumes?

## Assumptions

- The 2-second target is measured as wall-clock time from the user's perspective (page load to chart fully rendered), not just API response time
- The team has access to modify both the API and frontend code, and can deploy changes to both
- ClickHouse infrastructure changes (if needed) would require separate approval but are not categorically off the table
- The "main overview page" is a single, well-defined page that can be identified in the codebase
- The current 8-10 second measurement is reproducible and not caused by transient infrastructure issues
- The existing ClickHouse cluster has capacity headroom — the problem is query/rendering inefficiency, not raw compute limits
- No hard deadline has been set for this optimization work
