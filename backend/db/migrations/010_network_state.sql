-- Track most recent validated ledger observed by the network

CREATE TABLE IF NOT EXISTS core.network_state (
    id SMALLINT PRIMARY KEY,
    validated_ledger BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO core.network_state (id, validated_ledger)
VALUES (1, 0)
ON CONFLICT (id) DO NOTHING;

GRANT SELECT, UPDATE ON core.network_state TO api_ro;
