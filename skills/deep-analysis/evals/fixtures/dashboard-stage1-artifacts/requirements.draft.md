# Requirements Draft

## Goal

Reduce the load time of charts on the analytics dashboard's main overview page from the current 8-10 seconds to under 2 seconds.

## Problem Statement

The analytics dashboard's main overview page has degraded to 8-10 second load times for some charts, creating a poor experience for approximately 50,000 daily active users. The dashboard pulls data from ClickHouse through a GraphQL API layer (Go backend). The bottleneck is suspected in either the query layer (slow ClickHouse queries, sequential execution) or the frontend rendering (large datasets in the browser). The PM has set a target of under 2 seconds. A slow dashboard undermines trust in the data platform and reduces engagement.

## Scope

**Included:**
- Performance investigation and optimization of chart load times on the main overview page
- The full data path: ClickHouse queries, GraphQL API resolvers (`internal/analytics/`, `internal/clickhouse/`), and frontend chart rendering (`apps/dashboard/`)
- Specifically the "main overview page" — not every page in the dashboard

**Not included (to be confirmed):**
- Other dashboard pages beyond the main overview
- ClickHouse infrastructure changes (schema migrations, cluster scaling)
- Changes to the GraphQL API contract that would affect other consumers
- Mobile clients

## Out of Scope

To be determined — depends on clarification about whether infrastructure-level changes are acceptable and whether other API consumers constrain the GraphQL contract.

## Constraints

- **Performance target:** Chart load times must be under 2 seconds (PM requirement)
- **Scale:** ~50,000 daily active users
- **Architecture:** Data flows from ClickHouse through Go GraphQL API to React frontend dashboard
- **Code locations:** Dashboard frontend in `apps/dashboard/`, API in `internal/analytics/` and `internal/clickhouse/`, entry point `cmd/analytics-api/`

## Dependencies & Context

- **ClickHouse:** Events table with ~500M rows total, partitioned by (tenant_id, toYYYYMM(timestamp))
- **GraphQL API (`internal/analytics/`):** Resolvers, service layer — resolves 8 charts sequentially per overview page load
- **Dashboard frontend (`apps/dashboard/`):** React + Recharts, Apollo Client for GraphQL
- **Other API consumers:** Unknown whether other services consume the same GraphQL API

## Knowns

- Charts take 8-10 seconds to load on the main overview page
- Performance target is under 2 seconds
- Data flows from ClickHouse through Go GraphQL API (gqlgen) to React frontend
- Dashboard code in `apps/dashboard/`, API in `internal/analytics/` and `internal/clickhouse/`
- 50,000 daily active users
- 8 charts on the overview page, each triggering a separate ClickHouse query
- Charts are loaded sequentially on the backend
- No caching is currently used in the analytics service
- An unused cache utility exists in `internal/cache/`

## Unknowns

- Which specific charts are slowest
- Exact breakdown of time: ClickHouse query, Go processing, network transfer, frontend rendering
- Volume of data returned per chart (rows, payload size)
- Whether other consumers use the same GraphQL API
- Whether materialized views would help
- Current ClickHouse query execution times per chart
- Whether the frontend renders all charts simultaneously or could defer

## Assumptions

- The 2-second target is wall-clock time from user's perspective
- Team has access to modify both API and frontend code
- ClickHouse infrastructure changes (materialized views) are acceptable if needed
- The "main overview page" is a single well-defined page
- The current 8-10 second measurement is reproducible
- The ClickHouse cluster has capacity headroom
