package clickhouse

import "fmt"

// Predefined queries for each chart type.
// All queries filter by tenant_id and time range.
//
// PERFORMANCE NOTES:
// - events table has ~500M rows total across all tenants
// - Partitioned by (tenant_id, toYYYYMM(timestamp))
// - Primary key: (tenant_id, event_type, timestamp)
// - No materialized views currently in use
// - Queries on large tenants (>10M events) can take 2-5 seconds each
var queries = map[string]string{

	"active_users_over_time": `
		SELECT
			toStartOfDay(timestamp) as ts,
			map('active_users', uniq(user_id)) as values,
			map() as labels
		FROM events
		WHERE tenant_id = ?
			AND timestamp BETWEEN ? AND ?
		GROUP BY ts
		ORDER BY ts`,

	"events_volume": `
		SELECT
			toStartOfHour(timestamp) as ts,
			map('count', count()) as values,
			map('event_type', event_type) as labels
		FROM events
		WHERE tenant_id = ?
			AND timestamp BETWEEN ? AND ?
		GROUP BY ts, event_type
		ORDER BY ts`,

	"top_events_by_count": `
		SELECT
			toStartOfDay(timestamp) as ts,
			map('count', count()) as values,
			map('event_type', event_type) as labels
		FROM events
		WHERE tenant_id = ?
			AND timestamp BETWEEN ? AND ?
		GROUP BY ts, event_type
		ORDER BY count() DESC
		LIMIT 10000`,

	"conversion_funnel": `
		SELECT
			toStartOfDay(timestamp) as ts,
			map(
				'page_views', countIf(event_type = 'page_view'),
				'signups', countIf(event_type = 'signup'),
				'activations', countIf(event_type = 'activation'),
				'conversions', countIf(event_type = 'conversion')
			) as values,
			map() as labels
		FROM events
		WHERE tenant_id = ?
			AND timestamp BETWEEN ? AND ?
		GROUP BY ts
		ORDER BY ts`,

	"user_retention_cohort": `
		SELECT
			toStartOfWeek(first_seen) as ts,
			map('retention_rate', count(DISTINCT returning.user_id) / count(DISTINCT cohort.user_id)) as values,
			map('cohort_week', toString(toStartOfWeek(first_seen))) as labels
		FROM (
			SELECT user_id, min(timestamp) as first_seen
			FROM events
			WHERE tenant_id = ?
				AND timestamp BETWEEN ? AND ?
			GROUP BY user_id
		) as cohort
		LEFT JOIN (
			SELECT DISTINCT user_id, toStartOfWeek(timestamp) as active_week
			FROM events
			WHERE tenant_id = ?
				AND timestamp BETWEEN ? AND ?
		) as returning ON cohort.user_id = returning.user_id
		GROUP BY ts
		ORDER BY ts`,

	"users_by_region": `
		SELECT
			toStartOfDay(now()) as ts,
			map('users', uniq(user_id)) as values,
			map('region', region) as labels
		FROM events
		WHERE tenant_id = ?
			AND timestamp BETWEEN ? AND ?
			AND region != ''
		GROUP BY region
		ORDER BY uniq(user_id) DESC`,

	"avg_session_duration": `
		SELECT
			toStartOfHour(timestamp) as ts,
			map('avg_duration', avg(session_duration_ms) / 1000) as values,
			map() as labels
		FROM events
		WHERE tenant_id = ?
			AND timestamp BETWEEN ? AND ?
			AND event_type = 'session_end'
			AND session_duration_ms > 0
		GROUP BY ts
		ORDER BY ts`,

	"error_rate_over_time": `
		SELECT
			toStartOfMinute(timestamp) as ts,
			map(
				'error_rate', countIf(is_error = 1) / count(),
				'total', count()
			) as values,
			map() as labels
		FROM events
		WHERE tenant_id = ?
			AND timestamp BETWEEN ? AND ?
		GROUP BY ts
		ORDER BY ts`,
}

// GetQuery returns the SQL for a named query.
func (c *Client) GetQuery(name string) (string, error) {
	q, ok := queries[name]
	if !ok {
		return "", fmt.Errorf("unknown query: %s", name)
	}
	return q, nil
}
