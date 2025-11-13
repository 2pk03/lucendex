-- Migration: 002_orderbook_state.sql
-- Description: Create orderbook state table for tracking DEX order books
-- Author: Lucendex Team
-- Date: 2025-10-31

-- Orderbook State table
CREATE TABLE IF NOT EXISTS core.orderbook_state (
    id BIGSERIAL PRIMARY KEY,
    
    -- Asset pair identifiers
    base_asset TEXT NOT NULL,
    quote_asset TEXT NOT NULL,
    
    -- Order side ('bid' or 'ask')
    side TEXT NOT NULL CHECK (side IN ('bid', 'ask')),
    
    -- Price and amount (stored as text to preserve precision)
    price TEXT NOT NULL,
    amount TEXT NOT NULL,
    
    -- Offer details
    offer_sequence BIGINT NOT NULL,
    owner_account TEXT NOT NULL,
    
    -- Expiration (optional)
    expiration BIGINT,
    
    -- Quality (for pathfinding optimization)
    quality TEXT,
    
    -- Ledger tracking
    ledger_index BIGINT NOT NULL,
    ledger_hash TEXT,
    
    -- Status (active, filled, cancelled, expired)
    status TEXT NOT NULL DEFAULT 'active',
    
    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    -- Constraints
    CONSTRAINT orderbook_state_unique_offer UNIQUE (owner_account, offer_sequence),
    CONSTRAINT orderbook_state_positive_values CHECK (
        price::NUMERIC > 0 AND amount::NUMERIC >= 0
    )
);

-- Indexes for performance
CREATE INDEX idx_orderbook_pair ON core.orderbook_state(base_asset, quote_asset);
CREATE INDEX idx_orderbook_side ON core.orderbook_state(side);
CREATE INDEX idx_orderbook_price ON core.orderbook_state(price);
CREATE INDEX idx_orderbook_owner ON core.orderbook_state(owner_account);
CREATE INDEX idx_orderbook_status ON core.orderbook_state(status);
CREATE INDEX idx_orderbook_ledger_index ON core.orderbook_state(ledger_index);
CREATE INDEX idx_orderbook_expiration ON core.orderbook_state(expiration) WHERE expiration IS NOT NULL;

-- Composite index for order book queries
CREATE INDEX idx_orderbook_active_orders ON core.orderbook_state(base_asset, quote_asset, side, price)
    WHERE status = 'active';

-- Index for pathfinding
CREATE INDEX idx_orderbook_quality ON core.orderbook_state(quality)
    WHERE status = 'active' AND quality IS NOT NULL;

-- Trigger for automatic updated_at
CREATE TRIGGER orderbook_state_audit_trigger
    BEFORE INSERT OR UPDATE ON core.orderbook_state
    FOR EACH ROW EXECUTE FUNCTION core.audit_trigger_func();

-- Comments
COMMENT ON TABLE core.orderbook_state IS 'DEX orderbook state tracking';
COMMENT ON COLUMN core.orderbook_state.base_asset IS 'Base asset in format: currency.issuer or XRP';
COMMENT ON COLUMN core.orderbook_state.quote_asset IS 'Quote asset in format: currency.issuer or XRP';
COMMENT ON COLUMN core.orderbook_state.side IS 'Order side: bid or ask';
COMMENT ON COLUMN core.orderbook_state.price IS 'Order price (quote/base)';
COMMENT ON COLUMN core.orderbook_state.amount IS 'Order amount in base asset';
COMMENT ON COLUMN core.orderbook_state.offer_sequence IS 'XRPL offer sequence number';
COMMENT ON COLUMN core.orderbook_state.quality IS 'Quality metric for pathfinding';
COMMENT ON COLUMN core.orderbook_state.status IS 'Order status: active, filled, cancelled, expired';

-- Grant permissions
GRANT ALL PRIVILEGES ON core.orderbook_state TO indexer_rw;
GRANT SELECT ON core.orderbook_state TO router_ro;
GRANT SELECT ON core.orderbook_state TO api_ro;
