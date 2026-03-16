# Requirements: Analytics Dashboard Performance Optimization

## Objective
Reduce chart loading time on the main overview page from 8-10 seconds to under 2 seconds.

## Success Criteria
- P95 page load time for the main overview page is under 2 seconds.
- No regression in data accuracy or completeness of charts.
- No degradation in experience for other dashboard pages.
- Solution must sustain 50k DAU without performance collapse.

## Scope
- **Frontend**: `apps/dashboard/` -- chart rendering, data fetching, client-side state.
- **API**: `services/analytics-api/` -- GraphQL resolvers, query construction, response shaping.
- **Database**: ClickHouse queries executed by the API layer.

## Constraints
- Target: < 2 seconds end-to-end (user clicks page -> charts fully rendered).
- Must not break existing dashboard functionality or data correctness.
- 50k DAU -- solution must be load-tested or reasoned about at that concurrency.

## Out of Scope (unless investigation reveals otherwise)
- Network/CDN infrastructure changes.
- ClickHouse cluster topology or hardware changes.
- Redesigning the dashboard UI/UX beyond performance fixes.
