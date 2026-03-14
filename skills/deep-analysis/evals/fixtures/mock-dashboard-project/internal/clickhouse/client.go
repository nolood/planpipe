package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/rs/zerolog/log"
)

type Client struct {
	conn driver.Conn
}

func NewClient(dsn string) (*Client, error) {
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid ClickHouse DSN: %w", err)
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("ClickHouse connection failed: %w", err)
	}

	if err := conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ClickHouse ping failed: %w", err)
	}

	return &Client{conn: conn}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

type QueryParams struct {
	TenantID string
	TimeFrom time.Time
	TimeTo   time.Time
}

type Row struct {
	Timestamp time.Time
	Values    map[string]any
	Labels    map[string]any
}

// ExecuteQuery runs a named query against ClickHouse and returns raw rows.
// WARNING: No result size limit. Large time ranges can return 100k+ rows.
// The entire result set is loaded into memory before returning.
func (c *Client) ExecuteQuery(ctx context.Context, query string, params QueryParams) ([]Row, error) {
	start := time.Now()

	rows, err := c.conn.Query(ctx, query, params.TenantID, params.TimeFrom, params.TimeTo)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var result []Row
	for rows.Next() {
		var ts time.Time
		var values map[string]any
		var labels map[string]any

		if err := rows.Scan(&ts, &values, &labels); err != nil {
			return nil, fmt.Errorf("row scan failed: %w", err)
		}

		result = append(result, Row{
			Timestamp: ts,
			Values:    values,
			Labels:    labels,
		})
	}

	log.Debug().
		Dur("query_time", time.Since(start)).
		Int("rows", len(result)).
		Msg("ClickHouse query executed")

	return result, nil
}

// GetSummaryData runs aggregate queries for the dashboard summary section.
func (c *Client) GetSummaryData(ctx context.Context, tenantID string, from, to time.Time) (*SummaryData, error) {
	var summary SummaryData

	// Total users
	err := c.conn.QueryRow(ctx,
		`SELECT uniq(user_id) FROM events WHERE tenant_id = ? AND timestamp BETWEEN ? AND ?`,
		tenantID, from, to,
	).Scan(&summary.TotalUsers)
	if err != nil {
		return nil, err
	}

	// Active users (last 24h)
	err = c.conn.QueryRow(ctx,
		`SELECT uniq(user_id) FROM events WHERE tenant_id = ? AND timestamp > now() - INTERVAL 1 DAY`,
		tenantID,
	).Scan(&summary.ActiveUsers)
	if err != nil {
		return nil, err
	}

	// Total events
	err = c.conn.QueryRow(ctx,
		`SELECT count() FROM events WHERE tenant_id = ? AND timestamp BETWEEN ? AND ?`,
		tenantID, from, to,
	).Scan(&summary.TotalEvents)
	if err != nil {
		return nil, err
	}

	// Conversion rate
	err = c.conn.QueryRow(ctx,
		`SELECT countIf(event_type = 'conversion') / countIf(event_type = 'page_view')
		 FROM events WHERE tenant_id = ? AND timestamp BETWEEN ? AND ?`,
		tenantID, from, to,
	).Scan(&summary.ConversionRate)
	if err != nil {
		return nil, err
	}

	return &summary, nil
}

type SummaryData struct {
	TotalUsers     int64
	ActiveUsers    int64
	TotalEvents    int64
	ConversionRate float64
}
