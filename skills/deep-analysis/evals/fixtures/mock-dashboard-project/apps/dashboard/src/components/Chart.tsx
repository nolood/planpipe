import {
  LineChart,
  Line,
  BarChart,
  Bar,
  AreaChart,
  Area,
  PieChart,
  Pie,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Cell,
} from 'recharts';
import { format } from 'date-fns';

interface ChartProps {
  chartId: string;
  title: string;
  type: string;
  dataPoints: DataPoint[];
  metadata: {
    totalRows: number;
    queryTimeMs: number;
  };
}

interface DataPoint {
  timestamp: string;
  values: Record<string, number>;
  labels?: Record<string, string>;
}

const COLORS = ['#8884d8', '#82ca9d', '#ffc658', '#ff7300', '#0088FE', '#00C49F'];

/**
 * Renders a single chart. Accepts ALL data points and renders them directly.
 *
 * WARNING: No data downsampling or virtualization.
 * For charts with 10k+ data points, recharts renders every single SVG element,
 * causing significant rendering time (2-5 seconds for large datasets).
 *
 * The `values` map from each DataPoint is flattened into individual series.
 * Each key in the values map becomes a separate line/bar/area.
 */
export function Chart({ chartId, title, type, dataPoints, metadata }: ChartProps) {
  // Transform data for recharts — flatten the values map
  const chartData = dataPoints.map((dp) => ({
    time: format(new Date(dp.timestamp), 'MMM dd HH:mm'),
    timestamp: dp.timestamp,
    ...dp.values,
    ...(dp.labels || {}),
  }));

  // Extract value keys for series
  const valueKeys = dataPoints.length > 0
    ? Object.keys(dataPoints[0].values)
    : [];

  return (
    <div className="chart-container" data-chart-id={chartId}>
      <div className="chart-header">
        <h3>{title}</h3>
        <span className="chart-meta">
          {metadata.totalRows.toLocaleString()} rows | {metadata.queryTimeMs}ms
        </span>
      </div>

      <ResponsiveContainer width="100%" height={300}>
        {renderChart(type, chartData, valueKeys)}
      </ResponsiveContainer>
    </div>
  );
}

function renderChart(type: string, data: any[], valueKeys: string[]) {
  switch (type) {
    case 'line':
      return (
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="time" />
          <YAxis />
          <Tooltip />
          {valueKeys.map((key, i) => (
            <Line
              key={key}
              type="monotone"
              dataKey={key}
              stroke={COLORS[i % COLORS.length]}
              dot={false}  // dots disabled for performance, but still slow with many points
            />
          ))}
        </LineChart>
      );

    case 'bar':
      return (
        <BarChart data={data}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="time" />
          <YAxis />
          <Tooltip />
          {valueKeys.map((key, i) => (
            <Bar key={key} dataKey={key} fill={COLORS[i % COLORS.length]} />
          ))}
        </BarChart>
      );

    case 'area':
      return (
        <AreaChart data={data}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="time" />
          <YAxis />
          <Tooltip />
          {valueKeys.map((key, i) => (
            <Area
              key={key}
              type="monotone"
              dataKey={key}
              fill={COLORS[i % COLORS.length]}
              stroke={COLORS[i % COLORS.length]}
            />
          ))}
        </AreaChart>
      );

    case 'pie':
      return (
        <PieChart>
          <Pie data={data} dataKey={valueKeys[0] || 'value'} nameKey="region" cx="50%" cy="50%">
            {data.map((_, i) => (
              <Cell key={i} fill={COLORS[i % COLORS.length]} />
            ))}
          </Pie>
          <Tooltip />
        </PieChart>
      );

    default:
      return (
        <LineChart data={data}>
          <XAxis dataKey="time" />
          <YAxis />
          <Line type="monotone" dataKey={valueKeys[0] || 'value'} />
        </LineChart>
      );
  }
}
