-- Migration: 007_completed_trades.sql
-- Description: Track Lucendex-executed trades (quote-bound transactions only)
-- Author: Lucendex Team
-- Date: 2025-11-11

-- Completed Trades table (quote-bound transactions only)
CREATE TABLE IF NOT EXISTS core.completed_trades (
    id BIGSERIAL PRIMARY KEY,
    
    -- Quote binding
    quote_hash BYTEA NOT NULL,
    
    -- Transaction details
    tx_hash TEXT NOT NULL UNIQUE,
    account TEXT NOT NULL,
    
    -- Assets and amounts
    in_asset TEXT NOT NULL,
    out_asset TEXT NOT NULL,
    amount_in TEXT NOT NULL,
    amount_out TEXT NOT NULL,
    
    -- Route and fees
    route JSONB NOT NULL,
    router_fee_bps INTEGER NOT NULL,
    
    -- Ledger tracking
    ledger_index BIGINT NOT NULL,
    ledger_hash TEXT,
    
    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    -- Constraints
    CONSTRAINT completed_trades_positive_amounts CHECK (
        amount_in::NUMERIC > 0 AND amount_out::NUMERIC > 0
    )
);

-- Indexes for performance
CREATE INDEX idx_completed_trades_quote_hash ON core.completed_trades(quote_hash);
CREATE INDEX idx_completed_trades_account ON core.completed_trades(account);
CREATE INDEX idx_completed_trades_ledger_index ON core.completed_trades(ledger_index);
CREATE INDEX idx_completed_trades_created_at ON core.completed_trades(created_at);
CREATE INDEX idx_completed_trades_assets ON core.completed_trades(in_asset, out_asset);

-- Comments
COMMENT ON TABLE core.completed_trades IS 'Lucendex-executed trades (quote-bound only)';
COMMENT ON COLUMN core.completed_trades.quote_hash IS 'Blake2b-256 quote hash from transaction memo';
COMMENT ON COLUMN core.completed_trades.route IS 'Execution route (AMM pools and/or orderbook hops)';
COMMENT ON COLUMN core.completed_trades.router_fee_bps IS 'Lucendex routing fee in basis points';

-- Grant permissions
GRANT ALL PRIVILEGES ON core.completed_trades TO indexer_rw;
GRANT SELECT ON core.completed_trades TO router_ro;
GRANT SELECT ON core.completed_trades TO api_ro;

-- Cleanup function for 90-day retention
CREATE OR REPLACE FUNCTION core.cleanup_old_trades()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM core.completed_trades
    WHERE created_at < now() - interval '90 days';
    
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION core.cleanup_old_trades IS 'Delete completed trades older than 90 days';
