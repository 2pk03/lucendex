-- Migration 006: Router Security Infrastructure
-- Creates metering schema and security-related tables for M1
-- Date: 2025-11-10
-- Dependencies: Migrations 001-005 (core schema)

-- Create metering schema for router security and compliance
CREATE SCHEMA IF NOT EXISTS metering;

-- Grant schema usage to router and API roles
GRANT USAGE ON SCHEMA metering TO router_ro;
GRANT USAGE ON SCHEMA metering TO api_ro;

-- ============================================================================
-- Rate Limits Table
-- Purpose: Persist partner rate limit quotas across restarts
-- Strategy: PostgreSQL as source of truth, KV as cache
-- ============================================================================

CREATE TABLE metering.rate_limits (
    partner_id UUID NOT NULL,
    bucket BIGINT NOT NULL,             -- Unix timestamp / window size
    count INT NOT NULL CHECK (count >= 0),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (partner_id, bucket)
);

CREATE INDEX idx_rate_limits_expires ON metering.rate_limits(expires_at);
CREATE INDEX idx_rate_limits_partner ON metering.rate_limits(partner_id);

COMMENT ON TABLE metering.rate_limits IS 'Partner rate limit quotas - persisted across restarts';
COMMENT ON COLUMN metering.rate_limits.bucket IS 'Window identifier: unix_timestamp / window_size';
COMMENT ON COLUMN metering.rate_limits.count IS 'Request count in this window';
COMMENT ON COLUMN metering.rate_limits.expires_at IS 'When this bucket expires and can be cleaned up';

-- ============================================================================
-- Used Quotes Table
-- Purpose: Prevent quote replay attacks
-- Strategy: Track all used quotes to ensure single-use only
-- ============================================================================

CREATE TABLE metering.used_quotes (
    quote_hash BYTEA PRIMARY KEY CHECK (length(quote_hash) = 32),
    ledger_index BIGINT NOT NULL,
    used_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    partner_id UUID,                    -- Optional: track which partner used it
    tx_hash TEXT,                       -- XRPL transaction hash
    route_summary JSONB                 -- Brief route info for debugging
);

CREATE INDEX idx_used_quotes_expires ON metering.used_quotes(used_at);
CREATE INDEX idx_used_quotes_partner ON metering.used_quotes(partner_id) WHERE partner_id IS NOT NULL;
CREATE INDEX idx_used_quotes_ledger ON metering.used_quotes(ledger_index);

COMMENT ON TABLE metering.used_quotes IS 'Track used quotes to prevent replay attacks';
COMMENT ON COLUMN metering.used_quotes.quote_hash IS 'Blake2b-256 hash of quote (32 bytes)';
COMMENT ON COLUMN metering.used_quotes.used_at IS 'When quote was used - for cleanup (TTL + 1 hour retention)';
COMMENT ON COLUMN metering.used_quotes.route_summary IS 'Brief route info for audit purposes';

-- ============================================================================
-- Circuit Breaker State Table
-- Purpose: Persist circuit breaker state across restarts
-- Strategy: State persisted every 30s, loaded on startup
-- ============================================================================

CREATE TABLE metering.circuit_breaker_state (
    pair TEXT PRIMARY KEY,                          -- e.g., "XRP-USD.rXYZ"
    recent_prices JSONB NOT NULL DEFAULT '[]'::jsonb, -- Last 100 trade prices
    last_trade_ts TIMESTAMPTZ,
    state TEXT NOT NULL DEFAULT 'closed' CHECK (state IN ('closed', 'open', 'half-open')),
    failures INT NOT NULL DEFAULT 0 CHECK (failures >= 0),
    opened_at TIMESTAMPTZ,                          -- When breaker opened
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata JSONB                                  -- Additional state data
);

CREATE INDEX idx_breaker_updated ON metering.circuit_breaker_state(updated_at);
CREATE INDEX idx_breaker_state ON metering.circuit_breaker_state(state) WHERE state != 'closed';

COMMENT ON TABLE metering.circuit_breaker_state IS 'Circuit breaker state persistence';
COMMENT ON COLUMN metering.circuit_breaker_state.recent_prices IS 'Last 100 trades for anomaly detection';
COMMENT ON COLUMN metering.circuit_breaker_state.state IS 'closed (normal), open (blocking), half-open (testing)';
COMMENT ON COLUMN metering.circuit_breaker_state.failures IS 'Consecutive failures before opening';

-- ============================================================================
-- Router Audit Log Table
-- Purpose: Compliance-grade audit trail for all router operations
-- Strategy: No PII, hashed identifiers only, 90-day retention
-- ============================================================================

CREATE TABLE metering.router_audit (
    id BIGSERIAL PRIMARY KEY,
    ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    event TEXT NOT NULL,                    -- quote_request, circuit_break, validation_error, etc.
    partner_id UUID,                        -- Partner (if authenticated request)
    severity TEXT NOT NULL DEFAULT 'info' CHECK (severity IN ('info', 'warn', 'error', 'critical')),
    duration_ms INT CHECK (duration_ms >= 0),
    outcome TEXT NOT NULL CHECK (outcome IN ('success', 'rejected', 'error')),
    metadata JSONB,                         -- Event-specific data (NO PII)
    error_code TEXT,                        -- Error code if applicable
    user_agent_hash TEXT                    -- SHA256 of user agent (for abuse detection)
);

CREATE INDEX idx_router_audit_ts ON metering.router_audit(ts DESC);
CREATE INDEX idx_router_audit_partner ON metering.router_audit(partner_id) WHERE partner_id IS NOT NULL;
CREATE INDEX idx_router_audit_event ON metering.router_audit(event);
CREATE INDEX idx_router_audit_severity ON metering.router_audit(severity) WHERE severity IN ('error', 'critical');
CREATE INDEX idx_router_audit_outcome ON metering.router_audit(outcome) WHERE outcome != 'success';

COMMENT ON TABLE metering.router_audit IS 'Router audit log - compliance-grade, no PII';
COMMENT ON COLUMN metering.router_audit.event IS 'Event type: quote_request, circuit_break, etc.';
COMMENT ON COLUMN metering.router_audit.metadata IS 'Event data - amounts hashed, no IP addresses';
COMMENT ON COLUMN metering.router_audit.user_agent_hash IS 'SHA256 of user agent for abuse detection';

-- ============================================================================
-- Cleanup Functions
-- Purpose: Scheduled cleanup of expired data
-- ============================================================================

-- Cleanup expired rate limit buckets
CREATE OR REPLACE FUNCTION cleanup_expired_rate_limits()
RETURNS TABLE (deleted_count BIGINT) AS $$
DECLARE
    rows_deleted BIGINT;
BEGIN
    DELETE FROM metering.rate_limits WHERE expires_at < now();
    GET DIAGNOSTICS rows_deleted = ROW_COUNT;
    RETURN QUERY SELECT rows_deleted;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION cleanup_expired_rate_limits IS 'Delete expired rate limit buckets - run hourly';

-- Cleanup used quotes older than TTL
CREATE OR REPLACE FUNCTION cleanup_used_quotes()
RETURNS TABLE (deleted_count BIGINT) AS $$
DECLARE
    rows_deleted BIGINT;
BEGIN
    -- Delete quotes older than 1 hour (max TTL is ~5 min)
    DELETE FROM metering.used_quotes WHERE used_at < now() - interval '1 hour';
    GET DIAGNOSTICS rows_deleted = ROW_COUNT;
    RETURN QUERY SELECT rows_deleted;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION cleanup_used_quotes IS 'Delete quotes older than 1 hour - run hourly';

-- Cleanup old audit logs (90-day retention)
CREATE OR REPLACE FUNCTION cleanup_router_audit()
RETURNS TABLE (deleted_count BIGINT) AS $$
DECLARE
    rows_deleted BIGINT;
BEGIN
    DELETE FROM metering.router_audit WHERE ts < now() - interval '90 days';
    GET DIAGNOSTICS rows_deleted = ROW_COUNT;
    RETURN QUERY SELECT rows_deleted;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION cleanup_router_audit IS 'Delete audit logs older than 90 days - run daily';

-- ============================================================================
-- Row-Level Security (RLS)
-- Purpose: Multi-tenant isolation for partner data
-- ============================================================================

-- Enable RLS on all metering tables
ALTER TABLE metering.rate_limits ENABLE ROW LEVEL SECURITY;
ALTER TABLE metering.used_quotes ENABLE ROW LEVEL SECURITY;
ALTER TABLE metering.circuit_breaker_state ENABLE ROW LEVEL SECURITY;
ALTER TABLE metering.router_audit ENABLE ROW LEVEL SECURITY;

-- Policy: Router can read/write all data (read-only role, but needs full visibility)
CREATE POLICY router_rate_limits ON metering.rate_limits
    FOR ALL TO router_ro
    USING (true)
    WITH CHECK (true);

CREATE POLICY router_used_quotes ON metering.used_quotes
    FOR ALL TO router_ro
    USING (true)
    WITH CHECK (true);

CREATE POLICY router_breaker_state ON metering.circuit_breaker_state
    FOR ALL TO router_ro
    USING (true)
    WITH CHECK (true);

CREATE POLICY router_audit_write ON metering.router_audit
    FOR INSERT TO router_ro
    WITH CHECK (true);

CREATE POLICY router_audit_read ON metering.router_audit
    FOR SELECT TO router_ro
    USING (true);

-- Policy: API can only see partner's own audit logs
CREATE POLICY partner_audit_logs ON metering.router_audit
    FOR SELECT TO api_ro
    USING (
        partner_id IS NOT NULL AND 
        partner_id::text = current_setting('lucendex.partner_id', true)
    );

COMMENT ON POLICY router_rate_limits ON metering.rate_limits IS 'Router has full access to rate limits';
COMMENT ON POLICY partner_audit_logs ON metering.router_audit IS 'Partners can only see their own audit logs';

-- ============================================================================
-- Monitoring Views
-- Purpose: Operational visibility for monitoring dashboards
-- ============================================================================

CREATE OR REPLACE VIEW metering.rate_limit_summary AS
SELECT 
    partner_id,
    COUNT(*) as active_buckets,
    SUM(count) as total_requests,
    MAX(updated_at) as last_request,
    MIN(expires_at) as next_expiry
FROM metering.rate_limits
WHERE expires_at > now()
GROUP BY partner_id;

COMMENT ON VIEW metering.rate_limit_summary IS 'Summary of active rate limits per partner';

CREATE OR REPLACE VIEW metering.circuit_breaker_summary AS
SELECT 
    pair,
    state,
    failures,
    last_trade_ts,
    updated_at,
    CASE 
        WHEN state = 'open' THEN EXTRACT(EPOCH FROM (now() - opened_at))
        ELSE NULL
    END as open_duration_seconds
FROM metering.circuit_breaker_state
WHERE state != 'closed' OR updated_at > now() - interval '1 hour';

COMMENT ON VIEW metering.circuit_breaker_summary IS 'Active circuit breakers and recent activity';

CREATE OR REPLACE VIEW metering.audit_event_summary AS
SELECT 
    event,
    outcome,
    COUNT(*) as count,
    AVG(duration_ms) as avg_duration_ms,
    MAX(ts) as last_occurrence
FROM metering.router_audit
WHERE ts > now() - interval '1 hour'
GROUP BY event, outcome
ORDER BY count DESC;

COMMENT ON VIEW metering.audit_event_summary IS 'Hourly summary of audit events';

-- Grant SELECT on views to monitoring roles
GRANT SELECT ON metering.rate_limit_summary TO router_ro, api_ro;
GRANT SELECT ON metering.circuit_breaker_summary TO router_ro, api_ro;
GRANT SELECT ON metering.audit_event_summary TO router_ro, api_ro;

-- ============================================================================
-- Helper Functions for Router
-- Purpose: Convenience functions for common operations
-- ============================================================================

-- Check if quote has been used
CREATE OR REPLACE FUNCTION metering.is_quote_used(
    p_quote_hash BYTEA
)
RETURNS BOOLEAN AS $$
BEGIN
    RETURN EXISTS (
        SELECT 1 FROM metering.used_quotes 
        WHERE quote_hash = p_quote_hash
    );
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION metering.is_quote_used IS 'Check if quote hash has been used (for replay prevention)';

-- Get current rate limit for partner
CREATE OR REPLACE FUNCTION metering.get_rate_limit_count(
    p_partner_id UUID,
    p_bucket BIGINT
)
RETURNS INT AS $$
DECLARE
    current_count INT;
BEGIN
    SELECT count INTO current_count
    FROM metering.rate_limits
    WHERE partner_id = p_partner_id 
      AND bucket = p_bucket
      AND expires_at > now();
    
    RETURN COALESCE(current_count, 0);
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION metering.get_rate_limit_count IS 'Get current request count for partner in bucket';

-- ============================================================================
-- Permissions
-- ============================================================================

-- Router needs full access to metering schema
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA metering TO router_ro;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA metering TO router_ro;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA metering TO router_ro;

-- API only needs read access to audit logs (via RLS policy)
GRANT SELECT ON metering.router_audit TO api_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA metering TO api_ro;

-- Future tables in metering schema inherit permissions
ALTER DEFAULT PRIVILEGES IN SCHEMA metering 
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO router_ro;
ALTER DEFAULT PRIVILEGES IN SCHEMA metering 
    GRANT USAGE, SELECT ON SEQUENCES TO router_ro;

-- ============================================================================
-- Initial Data / Examples (Optional - for development/testing)
-- ============================================================================

-- Example: Insert initial circuit breaker states for common pairs
-- (Uncomment for development environment)
/*
INSERT INTO metering.circuit_breaker_state (pair, state) VALUES
    ('XRP-USD.rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B', 'closed'),
    ('XRP-EUR.rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq', 'closed')
ON CONFLICT (pair) DO NOTHING;
*/

-- ============================================================================
-- Verification Queries
-- ============================================================================

-- Verify schema creation
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = 'metering') THEN
        RAISE EXCEPTION 'metering schema not created';
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'metering' AND table_name = 'rate_limits') THEN
        RAISE EXCEPTION 'rate_limits table not created';
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'metering' AND table_name = 'used_quotes') THEN
        RAISE EXCEPTION 'used_quotes table not created';
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'metering' AND table_name = 'circuit_breaker_state') THEN
        RAISE EXCEPTION 'circuit_breaker_state table not created';
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'metering' AND table_name = 'router_audit') THEN
        RAISE EXCEPTION 'router_audit table not created';
    END IF;
    
    RAISE NOTICE 'Migration 006 completed successfully';
END $$;

-- Display table information
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
FROM pg_tables
WHERE schemaname = 'metering'
ORDER BY tablename;
