package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Store provides database operations for the indexer
type Store struct {
	db *sql.DB
}

// NewStore creates a new Store instance
func NewStore(connStr string) (*Store, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// AMMPool represents an AMM pool
type AMMPool struct {
	ID            int64
	Asset1        string
	Asset2        string
	Account       string
	LPToken       string
	Asset1Reserve string
	Asset2Reserve string
	TradingFee    int
	LedgerIndex   int64
	LedgerHash    string
}

// UpsertAMMPool inserts or updates an AMM pool
func (s *Store) UpsertAMMPool(ctx context.Context, pool *AMMPool) error {
	query := `
		INSERT INTO core.amm_pools 
			(asset1, asset2, account, lp_token, asset1_reserve, asset2_reserve, trading_fee, ledger_index, ledger_hash)
		VALUES 
			($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (account) 
		DO UPDATE SET
			asset1_reserve = EXCLUDED.asset1_reserve,
			asset2_reserve = EXCLUDED.asset2_reserve,
			trading_fee = EXCLUDED.trading_fee,
			ledger_index = EXCLUDED.ledger_index,
			ledger_hash = EXCLUDED.ledger_hash,
			updated_at = now()
		RETURNING id
	`

	err := s.db.QueryRowContext(ctx, query,
		pool.Asset1,
		pool.Asset2,
		pool.Account,
		pool.LPToken,
		pool.Asset1Reserve,
		pool.Asset2Reserve,
		pool.TradingFee,
		pool.LedgerIndex,
		pool.LedgerHash,
	).Scan(&pool.ID)

	if err != nil {
		return fmt.Errorf("failed to upsert AMM pool: %w", err)
	}

	return nil
}

// Offer represents a DEX offer
type Offer struct {
	ID            int64
	BaseAsset     string
	QuoteAsset    string
	Side          string
	Price         string
	Amount        string
	OfferSequence int64
	OwnerAccount  string
	Expiration    *int64
	Quality       *string
	LedgerIndex   int64
	LedgerHash    string
	Status        string
	Meta          map[string]interface{}
}

// UpsertOffer inserts or updates an offer
func (s *Store) UpsertOffer(ctx context.Context, offer *Offer) error {
	query := `
		INSERT INTO core.orderbook_state
			(base_asset, quote_asset, side, price, amount, offer_sequence, owner_account, expiration, quality, ledger_index, ledger_hash, status, meta)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb)
		ON CONFLICT (owner_account, offer_sequence)
		DO UPDATE SET
			price = EXCLUDED.price,
			amount = EXCLUDED.amount,
			status = EXCLUDED.status,
			ledger_index = EXCLUDED.ledger_index,
			ledger_hash = EXCLUDED.ledger_hash,
			meta = EXCLUDED.meta,
			updated_at = now()
		RETURNING id
	`

	// Convert Meta map to JSON
	var metaJSON []byte
	var err error
	if offer.Meta != nil {
		metaJSON, err = json.Marshal(offer.Meta)
		if err != nil {
			return fmt.Errorf("failed to marshal meta: %w", err)
		}
	} else {
		metaJSON = []byte("{}")
	}

	err = s.db.QueryRowContext(ctx, query,
		offer.BaseAsset,
		offer.QuoteAsset,
		offer.Side,
		offer.Price,
		offer.Amount,
		offer.OfferSequence,
		offer.OwnerAccount,
		offer.Expiration,
		offer.Quality,
		offer.LedgerIndex,
		offer.LedgerHash,
		offer.Status,
		string(metaJSON),
	).Scan(&offer.ID)

	if err != nil {
		return fmt.Errorf("failed to upsert offer: %w", err)
	}

	return nil
}

// CancelOffer marks an offer as cancelled
func (s *Store) CancelOffer(ctx context.Context, ownerAccount string, offerSequence int64, ledgerIndex int64) error {
	query := `
		UPDATE core.orderbook_state
		SET status = 'cancelled', ledger_index = $3, updated_at = now()
		WHERE owner_account = $1 AND offer_sequence = $2 AND status = 'active'
	`

	result, err := s.db.ExecContext(ctx, query, ownerAccount, offerSequence, ledgerIndex)
	if err != nil {
		return fmt.Errorf("failed to cancel offer: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no active offer found for account=%s sequence=%d", ownerAccount, offerSequence)
	}

	return nil
}

// LedgerCheckpoint represents a ledger checkpoint
type LedgerCheckpoint struct {
	LedgerIndex          int64
	LedgerHash           string
	ParentHash           string
	CloseTime            int64
	CloseTimeHuman       time.Time
	TransactionCount     int
	ProcessingDurationMs int
}

// SaveCheckpoint saves a ledger checkpoint
func (s *Store) SaveCheckpoint(ctx context.Context, cp *LedgerCheckpoint) error {
	query := `
		INSERT INTO core.ledger_checkpoints
			(ledger_index, ledger_hash, parent_hash, close_time, close_time_human, transaction_count, processing_duration_ms)
		VALUES
			($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (ledger_index)
		DO UPDATE SET
			ledger_hash = EXCLUDED.ledger_hash,
			parent_hash = EXCLUDED.parent_hash,
			transaction_count = EXCLUDED.transaction_count,
			processing_duration_ms = EXCLUDED.processing_duration_ms
	`

	_, err := s.db.ExecContext(ctx, query,
		cp.LedgerIndex,
		cp.LedgerHash,
		cp.ParentHash,
		cp.CloseTime,
		cp.CloseTimeHuman,
		cp.TransactionCount,
		cp.ProcessingDurationMs,
	)

	if err != nil {
		return fmt.Errorf("failed to save checkpoint: %w", err)
	}

	return nil
}

// GetLastCheckpoint retrieves the most recent checkpoint
func (s *Store) GetLastCheckpoint(ctx context.Context) (*LedgerCheckpoint, error) {
	query := `
		SELECT ledger_index, ledger_hash, parent_hash, close_time, close_time_human, transaction_count, processing_duration_ms
		FROM core.ledger_checkpoints
		ORDER BY ledger_index DESC
		LIMIT 1
	`

	cp := &LedgerCheckpoint{}
	err := s.db.QueryRowContext(ctx, query).Scan(
		&cp.LedgerIndex,
		&cp.LedgerHash,
		&cp.ParentHash,
		&cp.CloseTime,
		&cp.CloseTimeHuman,
		&cp.TransactionCount,
		&cp.ProcessingDurationMs,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No checkpoint exists yet
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get last checkpoint: %w", err)
	}

	return cp, nil
}

// GetCheckpoint retrieves a specific checkpoint by ledger index
func (s *Store) GetCheckpoint(ctx context.Context, ledgerIndex int64) (*LedgerCheckpoint, error) {
	query := `
		SELECT ledger_index, ledger_hash, parent_hash, close_time, close_time_human, transaction_count, processing_duration_ms
		FROM core.ledger_checkpoints
		WHERE ledger_index = $1
	`

	cp := &LedgerCheckpoint{}
	err := s.db.QueryRowContext(ctx, query, ledgerIndex).Scan(
		&cp.LedgerIndex,
		&cp.LedgerHash,
		&cp.ParentHash,
		&cp.CloseTime,
		&cp.CloseTimeHuman,
		&cp.TransactionCount,
		&cp.ProcessingDurationMs,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Checkpoint doesn't exist
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get checkpoint: %w", err)
	}

	return cp, nil
}

// CompletedTrade represents a Lucendex-executed trade
type CompletedTrade struct {
	ID           int64
	QuoteHash    []byte
	TxHash       string
	Account      string
	InAsset      string
	OutAsset     string
	AmountIn     string
	AmountOut    string
	Route        map[string]interface{}
	RouterFeeBps int
	LedgerIndex  int64
	LedgerHash   string
}

type QuoteRegistryEntry struct {
	QuoteHash []byte
	PartnerID string
	Route     []byte
	AmountIn  string
	AmountOut string
	RouterBps int
	ExpiresAt time.Time
	CreatedAt time.Time
}

type UsageEvent struct {
	PartnerID   string
	QuoteHash   []byte
	Pair        string
	AmountIn    string
	AmountOut   string
	RouterBps   int
	FeeAmount   string
	TxHash      string
	LedgerIndex int64
}

// InsertCompletedTrade records a completed Lucendex trade
func (s *Store) InsertCompletedTrade(ctx context.Context, trade *CompletedTrade) error {
	query := `
		INSERT INTO core.completed_trades
			(quote_hash, tx_hash, account, in_asset, out_asset, amount_in, amount_out, route, router_fee_bps, ledger_index, ledger_hash)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9, $10, $11)
		ON CONFLICT (tx_hash) DO NOTHING
		RETURNING id
	`

	// Convert Route map to JSON
	var routeJSON []byte
	var err error
	if trade.Route != nil {
		routeJSON, err = json.Marshal(trade.Route)
		if err != nil {
			return fmt.Errorf("failed to marshal route: %w", err)
		}
	} else {
		routeJSON = []byte("{}")
	}

	err = s.db.QueryRowContext(ctx, query,
		trade.QuoteHash,
		trade.TxHash,
		trade.Account,
		trade.InAsset,
		trade.OutAsset,
		trade.AmountIn,
		trade.AmountOut,
		string(routeJSON),
		trade.RouterFeeBps,
		trade.LedgerIndex,
		trade.LedgerHash,
	).Scan(&trade.ID)

	if err == sql.ErrNoRows {
		// Conflict - trade already exists, this is fine
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to insert completed trade: %w", err)
	}

	return nil
}

// LogConnectionEvent logs a connection event for compliance audit trail
// This method is non-blocking - if logging fails, it will not fail the calling operation
func (s *Store) LogConnectionEvent(service, event string, attempt int, err error, durationMs int, metadata map[string]interface{}) {
	// Fail silently if store or db is nil
	if s == nil || s.db == nil {
		return
	}

	// Short timeout for audit log - don't block normal operations
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	query := `
		INSERT INTO core.connection_events
			(service, event, attempt, error, duration_ms, metadata)
		VALUES
			($1, $2, $3, $4, $5, $6::jsonb)
	`

	var errStr *string
	if err != nil {
		errMsg := err.Error()
		errStr = &errMsg
	}

	var metaJSON []byte
	if metadata != nil {
		var jsonErr error
		metaJSON, jsonErr = json.Marshal(metadata)
		if jsonErr != nil {
			// Can't even log this properly, just return silently
			return
		}
	} else {
		metaJSON = []byte("{}")
	}

	_, execErr := s.db.ExecContext(ctx, query, service, event, attempt, errStr, durationMs, string(metaJSON))
	if execErr != nil {
		// Audit log failed - don't fail the operation, but we can't log it either
		// In production, this might be sent to an external monitoring system
		return
	}
}

func (s *Store) GetQuoteRegistryEntry(ctx context.Context, quoteHash []byte) (*QuoteRegistryEntry, error) {
	query := `
		SELECT quote_hash, partner_id::text, route, amount_in::text, amount_out::text, router_bps, expires_at, created_at
		FROM quote_registry
		WHERE quote_hash = $1
	`

	entry := &QuoteRegistryEntry{}
	err := s.db.QueryRowContext(ctx, query, quoteHash).Scan(
		&entry.QuoteHash,
		&entry.PartnerID,
		&entry.Route,
		&entry.AmountIn,
		&entry.AmountOut,
		&entry.RouterBps,
		&entry.ExpiresAt,
		&entry.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load quote registry entry: %w", err)
	}

	return entry, nil
}

func (s *Store) DeleteQuoteRegistryEntry(ctx context.Context, quoteHash []byte) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM quote_registry WHERE quote_hash = $1`, quoteHash)
	if err != nil {
		return fmt.Errorf("failed to delete quote registry entry: %w", err)
	}
	return nil
}

func (s *Store) InsertUsageEvent(ctx context.Context, event *UsageEvent) error {
	query := `
		INSERT INTO usage_events
			(partner_id, quote_hash, pair, amount_in, amount_out, router_bps, fee_amount, tx_hash, ledger_index)
		VALUES
			($1, $2, $3, $4::numeric, $5::numeric, $6, $7::numeric, $8, $9)
		ON CONFLICT DO NOTHING
	`

	_, err := s.db.ExecContext(ctx, query,
		event.PartnerID,
		event.QuoteHash,
		event.Pair,
		event.AmountIn,
		event.AmountOut,
		event.RouterBps,
		event.FeeAmount,
		event.TxHash,
		event.LedgerIndex,
	)
	if err != nil {
		return fmt.Errorf("failed to insert usage event: %w", err)
	}
	return nil
}
