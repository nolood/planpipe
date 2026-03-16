# Product Analysis: Dashboard Chart Load Time Optimization

## Business Intent

The analytics dashboard is a core product surface serving ~50,000 daily active users. The main overview page has degraded to 8-10 second load times, far exceeding the 2-second target set by the PM. This is a trust and engagement problem: a slow analytics dashboard undermines confidence in the data platform itself, and users who wait 10 seconds to see charts are likely to disengage, check less frequently, or seek alternative tools.

The business intent is not to add features but to restore a performant baseline: sub-2-second chart rendering on the overview page, measured as wall-clock time from the user's perspective (click/navigate to visual chart render).

## User Scenarios

### Primary Scenario: Dashboard Page Load
A user navigates to the main overview page. Today, they see "Loading dashboard..." for 8-10 seconds while a single GraphQL query (`overviewPage`) fetches all 8 charts plus summary data sequentially from ClickHouse. Nothing renders until everything is ready.

**Expected outcome after optimization:** The user sees meaningful content (summary bar, at least some charts) within 2 seconds. Charts may appear progressively rather than all-at-once.

### Scenario: Page Refresh / Tab Return
A user refreshes the page or returns to an already-open tab. Because the `OVERVIEW_PAGE_QUERY` variables include a dynamic time range (`new Date().toISOString()`), Apollo's cache-first policy effectively never produces a cache hit on refresh — the ISO timestamp changes every millisecond. The full 8-10 second load repeats every time.

**Expected outcome:** Repeated visits within a reasonable window (e.g., 1-5 minutes) should serve cached data or at most do a background revalidation.

### Scenario: Large Tenant with >10M Events
Per the codebase comments (`queries.go` line 8-13), tenants with >10M events experience 2-5 second query times **per chart**. With 8 charts sequential, this alone accounts for 16-40 seconds on the backend before any frontend rendering begins.

**Expected outcome:** Even large tenants should see sub-2-second load times, which implies either pre-aggregation, caching, or parallel query execution (or all three).

### Scenario: Time Range Change
A user changes the time range filter. This triggers a new GraphQL query. Currently there is no mechanism to cancel in-flight queries or show partial results during re-fetch.

**Expected outcome:** Time range changes should feel responsive, potentially with stale-while-revalidate behavior.

### Scenario: Individual Chart Failure
The backend already handles this gracefully — `service.go` line 33-34 skips failed charts without failing the whole page. However, users cannot currently tell which chart failed or retry a single chart.

**Expected outcome:** Chart-level error states with retry capability (lower priority but worth noting).

## Expected Outcomes

| Metric | Current State | Target |
|--------|--------------|--------|
| Overview page load time (wall-clock) | 8-10 seconds | < 2 seconds |
| Time to first meaningful paint | 8-10 seconds (all-or-nothing) | < 1 second (summary bar or first chart) |
| Backend API response time (`overviewPage`) | 8-10 seconds (sum of sequential queries) | < 1.5 seconds |
| Individual chart query time (large tenant) | 2-5 seconds | < 500ms (via caching or pre-aggregation) |
| Repeat visit load time | Same as fresh load | Near-instant from cache |

## Edge Cases

1. **Empty tenant (new customer, no data):** 8 queries returning 0 rows should be fast, but the sequential execution pattern still adds latency. The current code handles empty gracefully (returns empty arrays).

2. **Very large time ranges:** A user selecting "last 365 days" could produce massive result sets. The `error_rate_over_time` query groups by minute — over 365 days that is 525,600 data points. No query-side `LIMIT` is applied (except `top_events_by_count` which has `LIMIT 10000`). The frontend renders every point as an SVG element.

3. **Concurrent requests from many users of the same tenant:** With no caching layer, every user hitting the same overview page generates 9 independent ClickHouse queries. For popular tenants, this means redundant identical query load.

4. **ClickHouse cold start / query queuing:** If ClickHouse is under load, query times could spike beyond the 2-5 second baseline, making the sequential pattern even more punishing.

5. **Clock skew in time range variables:** The frontend passes `new Date().toISOString()` as the `to` parameter. Even sub-second differences between requests make Apollo's cache key unique, defeating client-side caching.

6. **The `user_retention_cohort` query uses `tenant_id` placeholder 4 times** but the `ExecuteQuery` function only passes 3 parameters (`tenantID, TimeFrom, TimeTo`). This query likely fails silently at runtime (the subquery and JOIN each need separate tenant/time parameters). This is a potential correctness bug in addition to a performance issue.

## Success Signals

- **Quantitative:** P95 overview page load time under 2 seconds as measured by browser Performance API or equivalent frontend instrumentation.
- **Quantitative:** Backend `overviewPage` resolver response time under 1.5 seconds (logged via `total_time` in `service.go` line 47).
- **Quantitative:** Individual ClickHouse query times under 500ms at P95 (logged at `client.go` line 80-83).
- **Qualitative:** Users perceive the dashboard as "instant" — progressive rendering means they see something useful within 500ms-1s.
- **Engagement:** DAU and session frequency on the dashboard should stabilize or increase (longer-term signal).
- **Operational:** ClickHouse query load should decrease if caching is implemented (fewer redundant queries for the same tenant/time range).

## User Impact Assessment

This is a high-impact change affecting all 50,000 DAU. The change is entirely performance-focused — no new features, no UI changes beyond potentially introducing progressive loading states (skeleton screens). The risk of negative user impact is low: users will see the same charts faster. The main risk is regressions — if optimization introduces stale data or incorrect cache invalidation, users could see outdated numbers, which for an analytics product is worse than slow load times.
