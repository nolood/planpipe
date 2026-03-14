package analytics

import "time"

// ChartData represents the data for a single chart on the dashboard.
type ChartData struct {
	ChartID    string      `json:"chartId"`
	Title      string      `json:"title"`
	ChartType  string      `json:"chartType"` // "line", "bar", "pie", "area", "table"
	DataPoints []DataPoint `json:"dataPoints"`
	Metadata   ChartMeta   `json:"metadata"`
}

type DataPoint struct {
	Timestamp time.Time      `json:"timestamp"`
	Values    map[string]any `json:"values"`
	Labels    map[string]any `json:"labels"`
}

type ChartMeta struct {
	TotalRows    int           `json:"totalRows"`
	QueryTimeMs  int64         `json:"queryTimeMs"`
	DataRange    TimeRange     `json:"dataRange"`
}

type TimeRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// OverviewPage represents the full data for the main dashboard overview.
type OverviewPage struct {
	Charts  []ChartData `json:"charts"`
	Summary SummaryData `json:"summary"`
}

type SummaryData struct {
	TotalUsers     int64   `json:"totalUsers"`
	ActiveUsers    int64   `json:"activeUsers"`
	TotalEvents    int64   `json:"totalEvents"`
	ConversionRate float64 `json:"conversionRate"`
}

// DashboardConfig defines which charts appear on the overview page.
// Currently hardcoded — no per-user or per-tenant customization.
type DashboardConfig struct {
	Charts []ChartConfig
}

type ChartConfig struct {
	ID        string
	Title     string
	ChartType string
	Query     string // ClickHouse SQL template
	TimeRange string // "24h", "7d", "30d"
}

// DefaultOverviewCharts defines the charts shown on the main overview page.
var DefaultOverviewCharts = []ChartConfig{
	{ID: "active-users", Title: "Active Users", ChartType: "line", Query: "active_users_over_time", TimeRange: "30d"},
	{ID: "events-volume", Title: "Event Volume", ChartType: "area", Query: "events_volume", TimeRange: "7d"},
	{ID: "top-events", Title: "Top Events", ChartType: "bar", Query: "top_events_by_count", TimeRange: "7d"},
	{ID: "conversion-funnel", Title: "Conversion Funnel", ChartType: "bar", Query: "conversion_funnel", TimeRange: "30d"},
	{ID: "user-retention", Title: "User Retention", ChartType: "line", Query: "user_retention_cohort", TimeRange: "30d"},
	{ID: "geo-distribution", Title: "Users by Region", ChartType: "pie", Query: "users_by_region", TimeRange: "30d"},
	{ID: "session-duration", Title: "Session Duration", ChartType: "line", Query: "avg_session_duration", TimeRange: "7d"},
	{ID: "error-rate", Title: "Error Rate", ChartType: "line", Query: "error_rate_over_time", TimeRange: "24h"},
}
