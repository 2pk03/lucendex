-- Add connection audit table for fintech compliance
-- Migration 005: Connection audit trail

CREATE TABLE IF NOT EXISTS core.connection_events (
    id BIGSERIAL PRIMARY KEY,
    ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    service TEXT NOT NULL,      -- 'postgres', 'rippled-ws', 'rippled-http'
    event TEXT NOT NULL,        -- 'attempt', 'success', 'failure', 'retry'
    attempt INT NOT NULL,       -- retry number (1-based)
    error TEXT,                 -- error message if failed
    duration_ms INT,            -- connection time if successful
    metadata JSONB DEFAULT '{}'::jsonb
);

-- Index for queries by service + timestamp
CREATE INDEX idx_connection_events_service_ts ON core.connection_events(service, ts DESC);

-- Index for querying failures
CREATE INDEX idx_connection_events_failures ON core.connection_events(event, ts DESC) WHERE event = 'failure';

-- Comment
COMMENT ON TABLE core.connection_events IS 'Complete audit trail of all connection attempts for fintech compliance';
