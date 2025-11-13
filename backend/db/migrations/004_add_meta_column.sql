-- Add meta column for storing transaction metadata and error details
-- Migration 004: Add meta JSONB column to orderbook_state

ALTER TABLE core.orderbook_state
ADD COLUMN meta JSONB DEFAULT '{}'::jsonb;

-- Add index for querying by status
CREATE INDEX idx_orderbook_status ON core.orderbook_state(status);

-- Add comment
COMMENT ON COLUMN core.orderbook_state.meta IS 'Transaction metadata and error details for audit trail';
