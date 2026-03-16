# Product / Business Analysis

## Business Intent

The analytics dashboard is the primary interface through which ~50,000 daily active users consume data from the platform. The main overview page -- the landing page users see on every visit -- has degraded to 8-10 second load times for charts, which is 4-5x slower than the 2-second target set by the PM.

This is not a new feature request. It is a repair task triggered by degraded performance that is actively harming user trust and engagement. Slow dashboards undermine the core value proposition of a data platform: if users cannot quickly see their analytics, they stop checking, stop trusting the numbers, and eventually disengage from the platform entirely. The business motivation is retention and engagement preservation -- the cost of inaction is users abandoning the dashboard for exported reports, third-party tools, or simply not looking at data at all.

The trigger is likely a combination of data growth (the events table now holds ~500M rows) and the absence of any performance optimization in the original implementation (no caching, no parallelism, no data downsampling). What was acceptable at smaller data volumes has become unacceptable at current scale.

## Actor & Scenario

**Primary Actor:** Analytics dashboard user -- a product manager, data analyst, or business stakeholder who checks the main overview page regularly (daily or multiple times per day) to monitor key metrics like active users, event volume, conversion funnel, and error rates.

**Main Scenario:**
1. Actor opens the analytics dashboard in their browser (or refreshes the page)
2. The overview page begins loading -- currently shows "Loading dashboard..." with no indication of progress
3. The system fetches all 8 charts + summary data from the GraphQL API in a single request
4. The backend sequentially queries ClickHouse for each of the 8 charts plus 4 summary aggregations -- total server-side time is the SUM of all individual query times (not the max)
5. After 8-10 seconds, the complete response arrives at the frontend
6. All 8 charts render simultaneously with no downsampling, adding additional rendering time for charts with large datasets
7. Actor finally sees the dashboard and can begin interpreting the data

**After optimization (target state):**
1. Actor opens the dashboard
2. Within 2 seconds, the actor sees charts with data and can begin interpreting
3. The experience feels responsive and trustworthy

**Secondary Actors:**
- **Platform engineering team:** Responsible for maintaining the Go backend and ClickHouse infrastructure. They need the optimization to be maintainable and not introduce operational complexity.
- **Other potential API consumers:** Unknown whether other services or tools consume the same GraphQL API. Changes to the API contract or response format could break them.

**Secondary Scenarios:**
- **Single chart refresh:** A user wants to see updated data for one specific chart without reloading the whole page. The `chartData` GraphQL query exists for this but is not currently used by the overview page.
- **Custom time range:** A user changes the time range for the overview page. This should still load within the target time.
- **First visit vs. repeat visit:** A returning user within a short window should see cached or faster results. Currently, every visit is a cold load because the time range in the query changes every millisecond.

## Expected Outcome

**What changes:**
- Chart load time on the main overview page drops from 8-10 seconds to under 2 seconds (wall-clock, from the user's perspective)
- Users perceive the dashboard as responsive and trustworthy
- The loading experience provides progressive feedback rather than a blank "Loading dashboard..." screen for 8+ seconds

**What stays the same:**
- The same 8 charts appear on the overview page with the same data
- The data accuracy and freshness expectations remain the same (or become explicitly defined -- e.g., data may be up to N minutes stale due to caching)
- The GraphQL API contract should remain backward-compatible for any existing consumers
- The dashboard's visual design and layout do not change
- Other dashboard pages (out of scope) continue to work as they do today

## Edge Cases

- **Large tenants (>10M events):** These are the worst case for query performance. The performance notes in `queries.go` explicitly call out 2-5 seconds per chart for large tenants. With 8 sequential queries, this alone accounts for 16-40 seconds of backend time. The optimization must specifically handle this case -- it is not an edge case but the primary driver of the problem.
- **Empty or new tenants:** A tenant with zero or very few events should still load quickly and display meaningful empty states. The current code handles failed charts by skipping them (`service.go:34`), but the frontend does not have empty-state handling per chart.
- **Concurrent page loads by many users:** With 50,000 DAU, there will be bursts of simultaneous page loads (e.g., at the start of the business day). If caching is introduced, concurrent requests for the same tenant should share cache entries rather than all hitting ClickHouse independently (thundering herd problem).
- **Stale data acceptability:** Introducing caching means users might see data that is minutes old. There is currently no explicit freshness requirement. If a user is monitoring real-time error rates and the data is cached for 5 minutes, they could miss an active incident. The error_rate chart uses a 24h time range and groups by minute -- this is the most time-sensitive chart.
- **Time range boundaries:** The frontend passes `new Date().toISOString()` as the `to` parameter, which changes every millisecond. This effectively prevents Apollo Client's cache from ever producing a hit, since the cache key includes the variables. Even if server-side caching is added, the client-side cache will remain useless until the time range is quantized.

## Success Signals

- **P95 page load time < 2 seconds:** The primary metric. Measured as wall-clock time from navigation to all charts being visible and interactive. Must be measured across tenant sizes, not just average.
- **Time to first chart visible:** A leading indicator. Even if total page load is 2s, showing the first chart in <500ms would significantly improve perceived performance. This requires progressive loading.
- **Dashboard engagement rate (daily active users / total users):** Lagging indicator. If the dashboard becomes fast, engagement should increase or at least stop declining. Baseline should be established before the change.
- **ClickHouse query load reduction:** If caching is effective, the total query volume against ClickHouse should decrease. This is an operational health signal, not a user signal, but it indicates whether the optimization is working at the infrastructure level.
- **Error rate on chart loads:** The current code silently skips failed charts (`service.go:34`). Monitoring how often charts fail to load is important -- an optimization that increases error rate is not a net improvement.

## Minimum Viable Outcome

The minimum viable outcome is reducing the P95 page load time to under 2 seconds for the main overview page, as measured from the user's perspective (browser). This requires at minimum addressing the sequential query execution on the backend, since even with fast individual queries, 8 sequential queries of 1 second each would still exceed the target.

The absolute core that cannot be cut: backend parallelization of chart queries. Without this, the 2-second target is mathematically unachievable for tenants with >10M events.

Everything else (caching, frontend downsampling, progressive loading, materialized views) adds value but is negotiable. If parallel queries alone bring load times under 2 seconds for the majority of tenants, that could be considered an MVO -- though caching would likely still be needed for the largest tenants.

## Critique Review

The critic assessed this analysis as SUFFICIENT. The business intent clearly connects slow dashboard performance to user trust and engagement erosion, going beyond a simple requirements restatement. The main scenario walks through the current flow step-by-step and explicitly identifies the cascade of sequential operations. Edge cases are specific to this task -- particularly the large-tenant case, thundering herd problem, and the cache-busting time range parameter in the frontend. The critic noted one minor observation: the analysis could have more explicitly addressed the tradeoff between data freshness and caching TTL, since no freshness SLA is defined anywhere in the requirements. This is captured as an open question below.

## Open Questions

- What is the acceptable data staleness for cached dashboard data? Real-time error monitoring suggests tight freshness needs for the error_rate chart, while retention cohorts are inherently backward-looking and could tolerate minutes or even hours of caching.
- Are there other consumers of the GraphQL API beyond the dashboard frontend? This determines whether API contract changes (e.g., adding pagination, changing response structure) are safe.
- Is the 2-second target measured at P50, P95, or P99? The difference matters significantly -- P50 under 2s is achievable with less effort than P99 under 2s.
- Should the optimization prioritize above-the-fold charts (the first 2-4 charts visible without scrolling) over below-the-fold charts that the user may never scroll to?
