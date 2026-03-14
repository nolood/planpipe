import { useQuery } from '@apollo/client';
import { OVERVIEW_PAGE_QUERY } from '../api/analytics';
import { ChartGrid } from '../components/ChartGrid';
import { Chart } from '../components/Chart';

interface OverviewProps {
  tenantId: string;
}

/**
 * Main overview page — shows all charts in a grid layout.
 * This is the page users see when they open the dashboard.
 *
 * Known performance issue: the single GraphQL query fetches ALL chart
 * data at once. The API resolves 8 charts sequentially, each hitting
 * ClickHouse. Total load time is the sum of all chart queries.
 *
 * The page waits for ALL data before rendering anything — no progressive
 * loading or skeleton states.
 */
export function Overview({ tenantId }: OverviewProps) {
  const { data, loading, error } = useQuery(OVERVIEW_PAGE_QUERY, {
    variables: {
      tenantId,
      // Default: last 30 days
      from: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString(),
      to: new Date().toISOString(),
    },
    // No caching strategy configured — uses Apollo's default
    // which is cache-first, but the cache key includes the time range
    // so it effectively never hits cache on page refresh
  });

  if (loading) {
    return <div className="loading">Loading dashboard...</div>;
  }

  if (error) {
    return <div className="error">Failed to load dashboard: {error.message}</div>;
  }

  const { charts, summary } = data.overviewPage;

  return (
    <div className="overview-page">
      <div className="summary-bar">
        <div className="stat">
          <span className="label">Total Users</span>
          <span className="value">{summary.totalUsers.toLocaleString()}</span>
        </div>
        <div className="stat">
          <span className="label">Active Users</span>
          <span className="value">{summary.activeUsers.toLocaleString()}</span>
        </div>
        <div className="stat">
          <span className="label">Total Events</span>
          <span className="value">{summary.totalEvents.toLocaleString()}</span>
        </div>
        <div className="stat">
          <span className="label">Conversion Rate</span>
          <span className="value">{(summary.conversionRate * 100).toFixed(1)}%</span>
        </div>
      </div>

      <ChartGrid>
        {charts.map((chart: any) => (
          <Chart
            key={chart.chartId}
            chartId={chart.chartId}
            title={chart.title}
            type={chart.chartType}
            dataPoints={chart.dataPoints}
            metadata={chart.metadata}
          />
        ))}
      </ChartGrid>
    </div>
  );
}
