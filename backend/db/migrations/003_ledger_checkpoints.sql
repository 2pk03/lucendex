-- Migration: 003_ledger_checkpoints.sql
-- Description: Create ledger checkpoints table for tracking indexer progress
-- Author: Lucendex Team
-- Date: 2025-10-31

-- Ledger Checkpoints table
CREATE TABLE IF NOT EXISTS core.ledger_checkpoints (
    ledger_index BIGINT PRIMARY KEY,
    
    -- Ledger identification
    ledger_hash TEXT NOT NULL,
    parent_hash TEXT,
    
    -- Timing
    close_time BIGINT NOT NULL,
    close_time_human TIMESTAMPTZ NOT NULL,
    
    -- Transaction stats
    transaction_count INTEGER NOT NULL DEFAULT 0,
    
    -- Processing state
    indexed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processing_duration_ms INTEGER,
    
    -- Metadata
    metadata JSONB,
    
    -- Constraints
    CONSTRAINT ledger_checkpoints_positive_close_time CHECK (close_time > 0)
);

-- Indexes
CREATE INDEX idx_ledger_checkpoints_hash ON core.ledger_checkpoints(ledger_hash);
CREATE INDEX idx_ledger_checkpoints_close_time ON core.ledger_checkpoints(close_time_human);
CREATE INDEX idx_ledger_checkpoints_indexed_at ON core.ledger_checkpoints(indexed_at);

-- View for latest checkpoint
CREATE OR REPLACE VIEW core.latest_checkpoint AS
SELECT * FROM core.ledger_checkpoints
ORDER BY ledger_index DESC
LIMIT 1;

-- Function to get indexer lag
CREATE OR REPLACE FUNCTION core.get_indexer_lag()
RETURNS TABLE(
    latest_indexed_ledger BIGINT,
    indexed_at TIMESTAMPTZ,
    lag_seconds INTEGER
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        lc.ledger_index,
        lc.indexed_at,
        EXTRACT(EPOCH FROM (now() - lc.indexed_at))::INTEGER AS lag_seconds
    FROM core.ledger_checkpoints lc
    ORDER BY lc.ledger_index DESC
    LIMIT 1;
END;
$$ LANGUAGE plpgsql;

-- Comments
COMMENT ON TABLE core.ledger_checkpoints IS 'Ledger processing checkpoints for indexer resumability';
COMMENT ON COLUMN core.ledger_checkpoints.ledger_index IS 'XRPL ledger sequence number';
COMMENT ON COLUMN core.ledger_checkpoints.ledger_hash IS 'Ledger hash for verification';
COMMENT ON COLUMN core.ledger_checkpoints.close_time IS 'Ledger close time (Ripple epoch)';
COMMENT ON COLUMN core.ledger_checkpoints.processing_duration_ms IS 'Time taken to process ledger in milliseconds';
COMMENT ON COLUMN core.ledger_checkpoints.metadata IS 'Additional ledger metadata (fees, reserve, etc)';

-- Grant permissions
GRANT ALL PRIVILEGES ON core.ledger_checkpoints TO indexer_rw;
GRANT SELECT ON core.ledger_checkpoints TO router_ro;
GRANT SELECT ON core.ledger_checkpoints TO api_ro;
GRANT SELECT ON core.latest_checkpoint TO router_ro;
GRANT SELECT ON core.latest_checkpoint TO api_ro;
GRANT EXECUTE ON FUNCTION core.get_indexer_lag() TO api_ro;
