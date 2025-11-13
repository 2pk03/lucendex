-- Partner API Schema
-- Creates tables for partner management, API authentication, and usage tracking

-- Partners table
CREATE TABLE IF NOT EXISTS partners (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    plan TEXT NOT NULL CHECK (plan IN ('free', 'pro', 'enterprise')),
    router_bps INT NOT NULL DEFAULT 20,  -- 0.2% = 20 basis points
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- API keys (Ed25519 public keys)
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    partner_id UUID NOT NULL REFERENCES partners(id) ON DELETE CASCADE,
    public_key TEXT NOT NULL UNIQUE,  -- Ed25519 public key (hex encoded)
    label TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_api_keys_partner ON api_keys(partner_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_pubkey ON api_keys(public_key) WHERE NOT revoked;

-- Usage events (populated by indexer when it detects QuoteHash in memos)
CREATE TABLE IF NOT EXISTS usage_events (
    id BIGSERIAL PRIMARY KEY,
    partner_id UUID NOT NULL REFERENCES partners(id),
    quote_hash BYTEA NOT NULL,
    pair TEXT NOT NULL,  -- e.g., "XRP-USD.rXYZ"
    amount_in NUMERIC NOT NULL,
    amount_out NUMERIC NOT NULL,
    router_bps INT NOT NULL,
    fee_amount NUMERIC NOT NULL,  -- calculated fee in USD equivalent
    tx_hash TEXT NOT NULL,
    ledger_index BIGINT NOT NULL,
    ts TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_usage_events_partner ON usage_events(partner_id, ts DESC);
CREATE INDEX IF NOT EXISTS idx_usage_events_quote_hash ON usage_events(quote_hash);
CREATE INDEX IF NOT EXISTS idx_usage_events_tx_hash ON usage_events(tx_hash);
CREATE INDEX IF NOT EXISTS idx_usage_events_ledger ON usage_events(ledger_index);

-- Quote registry (tracks quote -> partner mapping for attribution)
CREATE TABLE IF NOT EXISTS quote_registry (
    quote_hash BYTEA PRIMARY KEY,
    partner_id UUID NOT NULL REFERENCES partners(id),
    route JSONB NOT NULL,
    amount_in NUMERIC NOT NULL,
    amount_out NUMERIC NOT NULL,
    router_bps INT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,  -- TTL based on ledger
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_quote_registry_partner ON quote_registry(partner_id);
CREATE INDEX IF NOT EXISTS idx_quote_registry_expires ON quote_registry(expires_at);

-- Request ID tracking for replay protection
CREATE TABLE IF NOT EXISTS request_ids (
    request_id UUID PRIMARY KEY,
    partner_id UUID NOT NULL REFERENCES partners(id),
    timestamp TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_request_ids_expires ON request_ids(expires_at);

-- Monthly billing view
CREATE OR REPLACE VIEW monthly_billing AS
SELECT 
    p.id as partner_id,
    p.name as partner_name,
    p.plan,
    date_trunc('month', ue.ts) as billing_month,
    count(*) as tx_count,
    sum(ue.amount_out) as total_volume,
    sum(ue.fee_amount) as total_fees_usd
FROM usage_events ue
JOIN partners p ON p.id = ue.partner_id
GROUP BY p.id, p.name, p.plan, date_trunc('month', ue.ts);

-- Grant permissions to roles
GRANT SELECT, INSERT ON partners TO api_ro;
GRANT SELECT ON api_keys TO api_ro;
GRANT SELECT, INSERT ON usage_events TO indexer_rw;
GRANT SELECT, INSERT ON quote_registry TO api_ro;
GRANT SELECT, INSERT, DELETE ON request_ids TO api_ro;
GRANT SELECT ON monthly_billing TO api_ro;

-- Cleanup function for expired quotes
CREATE OR REPLACE FUNCTION cleanup_expired_quotes()
RETURNS void AS $$
BEGIN
    DELETE FROM quote_registry WHERE expires_at < now();
    DELETE FROM request_ids WHERE expires_at < now();
END;
$$ LANGUAGE plpgsql;

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER partners_updated_at
    BEFORE UPDATE ON partners
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
