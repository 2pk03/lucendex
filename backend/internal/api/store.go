package api

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) GetPartnerByID(ctx context.Context, partnerID uuid.UUID) (*Partner, error) {
	var p Partner
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, plan, router_bps, status, created_at, updated_at
		FROM partners
		WHERE id = $1
	`, partnerID).Scan(&p.ID, &p.Name, &p.Plan, &p.RouterBps, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *PostgresStore) GetAPIKeyByPublicKey(ctx context.Context, publicKey string) (*APIKey, error) {
	var k APIKey
	err := s.db.QueryRowContext(ctx, `
		SELECT id, partner_id, public_key, label, created_at, revoked, revoked_at
		FROM api_keys
		WHERE public_key = $1 AND revoked = false
	`, publicKey).Scan(&k.ID, &k.PartnerID, &k.PublicKey, &k.Label, &k.CreatedAt, &k.Revoked, &k.RevokedAt)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (s *PostgresStore) GetActiveAPIKey(ctx context.Context, partnerID uuid.UUID) (*APIKey, error) {
	var apiKey APIKey
	err := s.db.QueryRowContext(ctx, `
		SELECT id, partner_id, public_key, label, created_at, revoked, revoked_at
		FROM api_keys
		WHERE partner_id = $1 AND revoked = false
		ORDER BY created_at DESC
		LIMIT 1
	`, partnerID).Scan(&apiKey.ID, &apiKey.PartnerID, &apiKey.PublicKey, &apiKey.Label, &apiKey.CreatedAt, &apiKey.Revoked, &apiKey.RevokedAt)
	if err != nil {
		return nil, err
	}
	return &apiKey, nil
}

func (s *PostgresStore) CheckRequestID(ctx context.Context, requestID uuid.UUID, partnerID uuid.UUID) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM request_ids WHERE request_id = $1 AND partner_id = $2)
	`, requestID, partnerID).Scan(&exists)
	return exists, err
}

func (s *PostgresStore) StoreRequestID(ctx context.Context, requestID uuid.UUID, partnerID uuid.UUID, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO request_ids (request_id, partner_id, timestamp, expires_at)
		VALUES ($1, $2, $3, $4)
	`, requestID, partnerID, time.Now(), expiresAt)
	return err
}

func (s *PostgresStore) StoreQuoteRegistry(ctx context.Context, registry *QuoteRegistry) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO quote_registry (quote_hash, partner_id, route, amount_in, amount_out, router_bps, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, registry.QuoteHash, registry.PartnerID, registry.Route, registry.AmountIn, registry.AmountOut, registry.RouterBps, registry.ExpiresAt, registry.CreatedAt)
	return err
}

func (s *PostgresStore) GetPartnerUsage(ctx context.Context, partnerID uuid.UUID, month string) (*UsageResponse, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT quote_hash, pair, amount_in, amount_out, fee_amount, tx_hash, ledger_index, ts
		FROM usage_events
		WHERE partner_id = $1 AND date_trunc('month', ts) = $2::date
		ORDER BY ts DESC
	`, partnerID, month+"-01")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var details []UsageDetail
	var totalVolume, totalFees string
	var txCount int64

	for rows.Next() {
		var d UsageDetail
		var quoteHash []byte
		if err := rows.Scan(&quoteHash, &d.Pair, &d.AmountIn, &d.AmountOut, &d.FeeAmount, &d.TxHash, &d.LedgerIndex, &d.Timestamp); err != nil {
			return nil, err
		}
		d.QuoteHash = string(quoteHash)
		details = append(details, d)
		txCount++
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	err = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_out), 0), COALESCE(SUM(fee_amount), 0)
		FROM usage_events
		WHERE partner_id = $1 AND date_trunc('month', ts) = $2::date
	`, partnerID, month+"-01").Scan(&totalVolume, &totalFees)
	if err != nil {
		return nil, err
	}

	return &UsageResponse{
		Month:       month,
		TxCount:     txCount,
		TotalVolume: totalVolume,
		TotalFees:   totalFees,
		Details:     details,
	}, nil
}

func (s *PostgresStore) GetIndexerLag(ctx context.Context) (int, error) {
	var lag sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		WITH latest AS (
			SELECT COALESCE(MAX(ledger_index), 0) AS indexed
			FROM core.ledger_checkpoints
		),
		network AS (
			SELECT validated_ledger
			FROM core.network_state
			WHERE id = 1
		)
		SELECT GREATEST(network.validated_ledger - latest.indexed, 0)
		FROM latest, network
	`).Scan(&lag)
	if err == sql.ErrNoRows {
		return -1, nil
	}
	if err != nil {
		return 0, err
	}
	if !lag.Valid {
		return -1, nil
	}
	return int(lag.Int64), nil
}

func (s *PostgresStore) UpdateNetworkLedger(ctx context.Context, ledger uint32) error {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO core.network_state (id, validated_ledger, updated_at)
		VALUES (1, $1, now())
		ON CONFLICT (id)
		DO UPDATE SET validated_ledger = EXCLUDED.validated_ledger,
		             updated_at = EXCLUDED.updated_at
	`, ledger)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("failed to update network ledger")
	}
	return nil
}
