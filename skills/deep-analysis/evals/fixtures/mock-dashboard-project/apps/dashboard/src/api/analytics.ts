import { gql } from '@apollo/client';

/**
 * Main query used by the Overview page.
 * Fetches ALL charts + summary in a single request.
 *
 * The server resolves each chart sequentially — total response time
 * is the sum of all individual chart queries.
 *
 * All data points are included in the response — no pagination,
 * no limit on the number of points per chart.
 */
export const OVERVIEW_PAGE_QUERY = gql`
  query OverviewPage($tenantId: ID!, $from: DateTime, $to: DateTime) {
    overviewPage(tenantId: $tenantId, from: $from, to: $to) {
      charts {
        chartId
        title
        chartType
        dataPoints {
          timestamp
          values
          labels
        }
        metadata {
          totalRows
          queryTimeMs
          dataRange {
            from
            to
          }
        }
      }
      summary {
        totalUsers
        activeUsers
        totalEvents
        conversionRate
      }
    }
  }
`;

/**
 * Query for loading a single chart's data.
 * Used for chart refresh or drill-down (not currently used on Overview page).
 */
export const CHART_DATA_QUERY = gql`
  query ChartData($tenantId: ID!, $chartId: String!, $from: DateTime, $to: DateTime) {
    chartData(tenantId: $tenantId, chartId: $chartId, from: $from, to: $to) {
      chartId
      title
      chartType
      dataPoints {
        timestamp
        values
        labels
      }
      metadata {
        totalRows
        queryTimeMs
      }
    }
  }
`;
