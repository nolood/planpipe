import { ReactNode } from 'react';

interface ChartGridProps {
  children: ReactNode;
}

/**
 * Simple responsive grid layout for charts.
 * 2 columns on desktop, 1 on mobile.
 *
 * NOTE: All charts are rendered simultaneously — no lazy loading
 * or virtualization. If the overview has 8 charts with large datasets,
 * all 8 render at once, which compounds the rendering performance issue.
 */
export function ChartGrid({ children }: ChartGridProps) {
  return (
    <div
      className="chart-grid"
      style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(500px, 1fr))',
        gap: '24px',
        padding: '24px',
      }}
    >
      {children}
    </div>
  );
}
