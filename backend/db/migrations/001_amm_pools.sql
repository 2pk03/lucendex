-- Migration: 001_amm_pools.sql
-- Description: Create AMM pools table for tracking automated market maker pools
-- Author: Lucendex Team
-- Date: 2025-10-31

-- AMM Pools table
CREATE TABLE IF NOT EXISTS core.amm_pools (
    id BIGSERIAL PRIMARY KEY,
    
    -- Asset identifiers (currency.issuer format, e.g., "XRP" or "USD.rN7...")
    asset1 TEXT NOT NULL,
    asset2 TEXT NOT NULL,
    
    -- AMM account address
    account TEXT NOT NULL UNIQUE,
    
    -- LP token details
    lp_token TEXT NOT NULL,
    
    -- Pool reserves (stored as text to preserve precision)
    asset1_reserve TEXT NOT NULL DEFAULT '0',
    asset2_reserve TEXT NOT NULL DEFAULT '0',
    
    -- Trading fee (in basis points, e.g., 30 = 0.3%)
    trading_fee INTEGER NOT NULL DEFAULT 0,
    
    -- Auction slot (optional, for discounted fees)
    auction_slot JSONB,
    
    -- Ledger tracking
    ledger_index BIGINT NOT NULL,
    ledger_hash TEXT,
    
    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    -- Constraints
    CONSTRAINT amm_pools_unique_pair UNIQUE (asset1, asset2),
    CONSTRAINT amm_pools_positive_reserves CHECK (
        asset1_reserve::NUMERIC >= 0 AND asset2_reserve::NUMERIC >= 0
    )
);

-- Indexes for performance
CREATE INDEX idx_amm_pools_asset1 ON core.amm_pools(asset1);
CREATE INDEX idx_amm_pools_asset2 ON core.amm_pools(asset2);
CREATE INDEX idx_amm_pools_account ON core.amm_pools(account);
CREATE INDEX idx_amm_pools_ledger_index ON core.amm_pools(ledger_index);
CREATE INDEX idx_amm_pools_updated_at ON core.amm_pools(updated_at);

-- Composite index for pair lookups
CREATE INDEX idx_amm_pools_pair ON core.amm_pools(asset1, asset2);

-- Trigger for automatic updated_at
CREATE TRIGGER amm_pools_audit_trigger
    BEFORE INSERT OR UPDATE ON core.amm_pools
    FOR EACH ROW EXECUTE FUNCTION core.audit_trigger_func();

-- Comments
COMMENT ON TABLE core.amm_pools IS 'AMM pool state tracking';
COMMENT ON COLUMN core.amm_pools.asset1 IS 'First asset in format: currency.issuer or XRP';
COMMENT ON COLUMN core.amm_pools.asset2 IS 'Second asset in format: currency.issuer or XRP';
COMMENT ON COLUMN core.amm_pools.account IS 'AMM account address on XRPL';
COMMENT ON COLUMN core.amm_pools.lp_token IS 'LP token currency code';
COMMENT ON COLUMN core.amm_pools.trading_fee IS 'Trading fee in basis points';
COMMENT ON COLUMN core.amm_pools.auction_slot IS 'Current auction slot holder and discount';

-- Grant permissions (from setup-db.sql roles)
GRANT ALL PRIVILEGES ON core.amm_pools TO indexer_rw;
GRANT SELECT ON core.amm_pools TO router_ro;
GRANT SELECT ON core.amm_pools TO api_ro;
