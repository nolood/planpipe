package analytics

import (
	"context"
	"time"
)

// Resolver is the GraphQL resolver for analytics queries.
// It delegates to the analytics Service for data loading.
type Resolver struct {
	svc *Service
}

func NewResolver(svc *Service) *Resolver {
	return &Resolver{svc: svc}
}

// QueryResolver implements the GraphQL Query type.
type queryResolver struct {
	*Resolver
}

func (r *Resolver) Query() QueryResolver {
	return &queryResolver{r}
}

// OverviewPage resolves the full overview page data.
// Called by the dashboard frontend on page load.
// NOTE: This is the main performance bottleneck endpoint.
// A single call triggers 8 sequential ClickHouse queries + 1 summary query.
func (r *queryResolver) OverviewPage(ctx context.Context, tenantID string, from *time.Time, to *time.Time) (*OverviewPage, error) {
	tr := TimeRange{
		From: defaultTimeFrom(from),
		To:   defaultTimeTo(to),
	}
	return r.svc.GetOverviewPage(ctx, tenantID, tr)
}

// ChartData resolves a single chart's data by ID.
func (r *queryResolver) ChartData(ctx context.Context, tenantID string, chartID string, from *time.Time, to *time.Time) (*ChartData, error) {
	tr := TimeRange{
		From: defaultTimeFrom(from),
		To:   defaultTimeTo(to),
	}
	return r.svc.GetChartData(ctx, tenantID, chartID, tr)
}

// Summary resolves just the summary numbers without full chart data.
func (r *queryResolver) Summary(ctx context.Context, tenantID string, from *time.Time, to *time.Time) (*SummaryData, error) {
	tr := TimeRange{
		From: defaultTimeFrom(from),
		To:   defaultTimeTo(to),
	}
	return r.svc.loadSummary(ctx, tenantID, tr)
}

func defaultTimeFrom(t *time.Time) time.Time {
	if t != nil {
		return *t
	}
	return time.Now().AddDate(0, 0, -30) // Default: 30 days ago
}

func defaultTimeTo(t *time.Time) time.Time {
	if t != nil {
		return *t
	}
	return time.Now()
}
