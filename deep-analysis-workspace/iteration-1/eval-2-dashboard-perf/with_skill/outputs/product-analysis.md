# Product / Business Analysis

## Business Intent

The analytics dashboard is the primary interface for approximately 50,000 daily active users to monitor their product metrics. Chart load times have degraded to 8-10 seconds on the main overview page, which is the landing page users see on every visit. This degradation undermines trust in the data platform -- when a dashboard is slow, users question both the platform's reliability and the data's accuracy. The task exists because slow dashboards directly reduce engagement: users stop checking the dashboard, which means they stop making data-informed decisions, which undermines the entire value proposition of the analytics product.

The trigger is likely a combination of growing data volumes (the events table has ~500M rows and is growing) and a system architecture that was built for smaller scale (sequential queries, no caching, no aggregation). The problem will worsen over time without intervention.

The business value is retention and engagement: keeping existing users actively using the dashboard rather than abandoning it for manual exports, third-party tools, or simply not looking at their data.

## Actor & Scenario

**Primary Actor:** Analytics dashboard user -- a product manager, data analyst, or business operator who checks the dashboard to understand product health and make decisions. They use the overview page as their first stop, scanning summary stats and charts to identify trends or anomalies.

**Main Scenario:**
1. User opens the analytics dashboard in their browser (or refreshes the page)
2. The overview page begins loading, showing a loading spinner
3. The browser sends a single GraphQL query requesting all 8 charts and the summary data
4. The Go backend sequentially executes 8 ClickHouse queries (one per chart) plus 4 summary queries, taking 8-10 seconds total
5. The backend returns all data in a single response
6. The browser receives the response, then renders all 8 charts simultaneously using Recharts (potentially adding 2-5 seconds for large datasets)
7. User finally sees the complete overview page
8. **Target end state:** User sees the overview page with charts in under 2 seconds total

**Secondary Actors:**
- **Platform/engineering team:** Responsible for maintaining acceptable performance. They need observability into query times and rendering performance.
- **Potential API consumers:** Unknown whether other services consume the same GraphQL API. If they do, changes to the API contract or response format could break them.

**Secondary Scenarios:**
- User refreshes the page after a short interval (should benefit from caching, but currently doesn't due to dynamic time-range cache keys)
- User views the dashboard for a large tenant with >10M events (worst-case performance scenario)
- User views the dashboard for a small or new tenant with minimal data (should be fast already)
- User returns to the overview page after navigating to another page within the dashboard (should reuse cached data)

## Expected Outcome

**What changes:**
- The overview page loads in under 2 seconds from the user's perspective (wall-clock time from navigation to visible charts)
- Users perceive the dashboard as responsive and trustworthy
- The page may render progressively -- summary stats and some charts appearing before others -- which is acceptable and potentially preferable to waiting for everything

**What stays the same:**
- The 8 charts on the overview page remain the same (same data, same visualizations, same layout)
- The summary bar continues to show Total Users, Active Users, Total Events, and Conversion Rate
- The data accuracy remains identical -- users should see the same numbers they see today, just faster
- The GraphQL API contract should remain backward-compatible (if other consumers exist)
- The dashboard's URL structure, navigation, and user-facing behavior should be unchanged

## Edge Cases

- **Large tenant with >10M events over 30 days:** This is the worst-case scenario that most clearly exposes the bottleneck. The current architecture has no data aggregation or downsampling, so these tenants generate both the slowest queries and the largest payloads. If optimization only helps smaller tenants, the task has failed.

- **First page load vs. subsequent loads:** The first load has no cache to draw from. Subsequent loads within the same session (or within a cache TTL window) should be significantly faster if caching is introduced. The 2-second target needs to account for cold-start vs. warm scenarios -- clarify whether 2 seconds means cold or warm.

- **Time range at month boundary:** The default time range is "last 30 days," which means the query's time window crosses ClickHouse partition boundaries (partitioned by toYYYYMM). Near the start of a month, the query touches two partitions; mid-month, mostly one. This could cause variable performance that's hard to reproduce consistently.

- **Stale data tolerance:** If caching is introduced, there is a question of how stale the data can be. An overview dashboard with 1-minute-old data is fine. An overview dashboard showing 1-hour-old data during an incident might not be. The acceptable staleness window matters for cache TTL decisions.

- **Partial chart failure:** The current code already handles individual chart failures gracefully (skips failed charts). Any optimization must preserve this behavior -- a slow or failing chart should not block the rest of the page.

## Success Signals

- **P95 overview page load time < 2 seconds:** Measured end-to-end from the browser's perspective (navigation start to last chart rendered). This is the primary metric. P95 matters more than average because the worst-case users (large tenants) are the ones most affected.

- **Time to first meaningful paint < 1 second:** If progressive loading is implemented, the summary bar and first 1-2 charts should appear within 1 second, even if the remaining charts load shortly after. This is a leading indicator of perceived performance improvement.

- **No increase in dashboard error rate:** Performance optimization should not introduce new failure modes. Monitor the rate of failed chart loads before and after.

- **Dashboard session frequency stabilizes or increases:** If users were abandoning the dashboard due to slowness, improved performance should lead to more frequent visits. This is a lagging indicator measurable over weeks.

## Minimum Viable Outcome

The minimum viable outcome is that the overview page loads in under 2 seconds for the P95 case (large tenants). This requires addressing the dominant bottleneck, which is the sequential execution of 8 ClickHouse queries on the backend. If only one thing changes, it should be making these queries parallel -- that alone could reduce backend time from 8-10 seconds (sum of 8 queries at 1-2s each) to 2-5 seconds (max of 8 queries). Further improvements (caching, data downsampling, pre-aggregation) would push below the 2-second target, but parallelization is the non-negotiable core.

If frontend rendering is also a bottleneck (for charts with 10k+ points), data downsampling or limiting row counts server-side would be the second priority. Caching and materialized views are valuable but are enhancements on top of the core architectural fix.

## Self-Critique Notes

- The cold-start vs. warm-load distinction for the 2-second target is unresolved. The requirements say "under 2 seconds" but don't specify whether this is for a fresh visit or a revisit. This matters significantly for how aggressive the optimization needs to be.

- The assumption that sequential query execution is the dominant bottleneck is well-supported by the code analysis (8 queries * 1-2s each = 8-16s), but the actual time breakdown between backend query time, network transfer, and frontend rendering hasn't been measured. If frontend rendering of large datasets turns out to be the bottleneck (the code comments suggest 2-5 seconds for 10k+ points), parallelizing backend queries alone won't be sufficient.

- The "session frequency increases" success signal is an assumption about user behavior -- we're guessing that slow load times are causing reduced usage. This may or may not be true; the correlation hasn't been established.

- I haven't accounted for the possibility that some of the 8 charts are rarely looked at. If 6 of 8 charts get little attention, lazy loading below-the-fold charts would be a high-value optimization with low complexity. This is a product question that should be investigated.
