-- ClickHouse schema for analytics events

CREATE TABLE events (
    tenant_id       String,
    user_id         String,
    event_type      LowCardinality(String),
    timestamp       DateTime64(3),
    session_id      String,
    session_duration_ms UInt32 DEFAULT 0,
    region          LowCardinality(String) DEFAULT '',
    is_error        UInt8 DEFAULT 0,
    properties      Map(String, String),
    created_at      DateTime DEFAULT now()
)
ENGINE = MergeTree()
PARTITION BY (tenant_id, toYYYYMM(timestamp))
ORDER BY (tenant_id, event_type, timestamp)
SETTINGS index_granularity = 8192;

-- NOTE: No materialized views exist for pre-aggregated data.
-- All dashboard queries hit the raw events table directly.
-- For tenants with >10M events, this causes slow queries (2-5s per chart).

-- NOTE: No secondary indexes on user_id or session_id.
-- Queries filtering by user_id require full partition scans.
