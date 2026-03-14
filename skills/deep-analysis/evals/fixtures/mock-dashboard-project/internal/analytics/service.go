package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/acme/analytics/internal/clickhouse"
)

type Service struct {
	ch *clickhouse.Client
}

func NewService(ch *clickhouse.Client) *Service {
	return &Service{ch: ch}
}

// GetOverviewPage loads ALL charts for the main overview page.
// Each chart triggers a separate ClickHouse query.
// NOTE: No caching, no parallelism — charts are loaded sequentially.
func (s *Service) GetOverviewPage(ctx context.Context, tenantID string, timeRange TimeRange) (*OverviewPage, error) {
	start := time.Now()

	charts := make([]ChartData, 0, len(DefaultOverviewCharts))

	// Load each chart one by one
	for _, cfg := range DefaultOverviewCharts {
		chartData, err := s.loadChart(ctx, tenantID, cfg, timeRange)
		if err != nil {
			log.Error().Err(err).Str("chart", cfg.ID).Msg("failed to load chart")
			continue // Skip failed charts, don't fail the whole page
		}
		charts = append(charts, *chartData)
	}

	// Load summary separately
	summary, err := s.loadSummary(ctx, tenantID, timeRange)
	if err != nil {
		log.Error().Err(err).Msg("failed to load summary")
		summary = &SummaryData{} // Return empty summary on error
	}

	log.Info().
		Dur("total_time", time.Since(start)).
		Int("charts_loaded", len(charts)).
		Msg("overview page loaded")

	return &OverviewPage{
		Charts:  charts,
		Summary: *summary,
	}, nil
}

// loadChart executes a single chart query and transforms the result.
func (s *Service) loadChart(ctx context.Context, tenantID string, cfg ChartConfig, tr TimeRange) (*ChartData, error) {
	queryStart := time.Now()

	// Get the query for this chart type
	query, err := s.ch.GetQuery(cfg.Query)
	if err != nil {
		return nil, fmt.Errorf("unknown query %s: %w", cfg.Query, err)
	}

	// Execute query — returns raw rows
	rows, err := s.ch.ExecuteQuery(ctx, query, clickhouse.QueryParams{
		TenantID:  tenantID,
		TimeFrom:  tr.From,
		TimeTo:    tr.To,
	})
	if err != nil {
		return nil, fmt.Errorf("query execution failed for %s: %w", cfg.ID, err)
	}

	// Transform raw rows into DataPoints
	// NOTE: All rows are loaded into memory at once. For charts with large
	// time ranges this can be tens of thousands of points.
	dataPoints := make([]DataPoint, 0, len(rows))
	for _, row := range rows {
		dp := DataPoint{
			Timestamp: row.Timestamp,
			Values:    row.Values,
			Labels:    row.Labels,
		}
		dataPoints = append(dataPoints, dp)
	}

	queryTime := time.Since(queryStart).Milliseconds()

	return &ChartData{
		ChartID:    cfg.ID,
		Title:      cfg.Title,
		ChartType:  cfg.ChartType,
		DataPoints: dataPoints,
		Metadata: ChartMeta{
			TotalRows:   len(dataPoints),
			QueryTimeMs: queryTime,
			DataRange:   tr,
		},
	}, nil
}

func (s *Service) loadSummary(ctx context.Context, tenantID string, tr TimeRange) (*SummaryData, error) {
	return s.ch.GetSummaryData(ctx, tenantID, tr.From, tr.To)
}

// GetChartData loads a single chart by ID.
func (s *Service) GetChartData(ctx context.Context, tenantID, chartID string, tr TimeRange) (*ChartData, error) {
	for _, cfg := range DefaultOverviewCharts {
		if cfg.ID == chartID {
			return s.loadChart(ctx, tenantID, cfg, tr)
		}
	}
	return nil, fmt.Errorf("chart %s not found", chartID)
}
