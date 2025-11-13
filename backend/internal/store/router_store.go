package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type RouterStore struct {
	db *sql.DB
}

func NewRouterStore(connStr string) (*RouterStore, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &RouterStore{db: db}, nil
}

func (s *RouterStore) Close() error {
	return s.db.Close()
}

type RateLimit struct {
	PartnerID string
	Bucket    int64
	Count     int
	ExpiresAt time.Time
}

func (s *RouterStore) GetRateLimit(ctx context.Context, partnerID string, bucket int64) (*RateLimit, error) {
	query := `
		SELECT partner_id, bucket, count, expires_at
		FROM metering.rate_limits
		WHERE partner_id = $1 AND bucket = $2 AND expires_at > now()
	`

	rl := &RateLimit{}
	err := s.db.QueryRowContext(ctx, query, partnerID, bucket).Scan(
		&rl.PartnerID, &rl.Bucket, &rl.Count, &rl.ExpiresAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get rate limit: %w", err)
	}

	return rl, nil
}

func (s *RouterStore) UpsertRateLimit(ctx context.Context, rl *RateLimit) error {
	query := `
		INSERT INTO metering.rate_limits (partner_id, bucket, count, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (partner_id, bucket)
		DO UPDATE SET count = EXCLUDED.count, expires_at = EXCLUDED.expires_at
	`

	_, err := s.db.ExecContext(ctx, query, rl.PartnerID, rl.Bucket, rl.Count, rl.ExpiresAt)
	if err != nil {
		return fmt.Errorf("failed to upsert rate limit: %w", err)
	}

	return nil
}

func (s *RouterStore) IsQuoteUsed(ctx context.Context, quoteHash []byte) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM metering.used_quotes WHERE quote_hash = $1)`

	var exists bool
	err := s.db.QueryRowContext(ctx, query, quoteHash).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check quote: %w", err)
	}

	return exists, nil
}

type UsedQuote struct {
	QuoteHash    []byte
	LedgerIndex  int64
	UsedAt       time.Time
	PartnerID    *string
	TxHash       *string
	RouteSummary map[string]interface{}
}

func (s *RouterStore) MarkQuoteUsed(ctx context.Context, uq *UsedQuote) error {
	var summaryJSON []byte
	var err error
	if uq.RouteSummary != nil {
		summaryJSON, err = json.Marshal(uq.RouteSummary)
		if err != nil {
			return fmt.Errorf("failed to marshal route summary: %w", err)
		}
	}

	query := `
		INSERT INTO metering.used_quotes (quote_hash, ledger_index, partner_id, tx_hash, route_summary)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (quote_hash) DO NOTHING
	`

	_, err = s.db.ExecContext(ctx, query,
		uq.QuoteHash, uq.LedgerIndex, uq.PartnerID, uq.TxHash, summaryJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to mark quote used: %w", err)
	}

	return nil
}

type CircuitBreakerState struct {
	Pair         string
	RecentPrices []float64
	LastTradeTs  *time.Time
	State        string
	Failures     int
	OpenedAt     *time.Time
	UpdatedAt    time.Time
	Metadata     map[string]interface{}
}

func (s *RouterStore) GetCircuitBreakerState(ctx context.Context, pair string) (interface{}, error) {
	query := `
		SELECT pair, recent_prices, last_trade_ts, state, failures, opened_at, updated_at, metadata
		FROM metering.circuit_breaker_state
		WHERE pair = $1
	`

	cb := &CircuitBreakerState{}
	var pricesJSON, metaJSON []byte

	err := s.db.QueryRowContext(ctx, query, pair).Scan(
		&cb.Pair, &pricesJSON, &cb.LastTradeTs, &cb.State, &cb.Failures,
		&cb.OpenedAt, &cb.UpdatedAt, &metaJSON,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get circuit breaker state: %w", err)
	}

	if err := json.Unmarshal(pricesJSON, &cb.RecentPrices); err != nil {
		return nil, fmt.Errorf("failed to unmarshal prices: %w", err)
	}

	if metaJSON != nil {
		if err := json.Unmarshal(metaJSON, &cb.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return cb, nil
}

func (s *RouterStore) SaveCircuitBreakerState(ctx context.Context, cbInterface interface{}) error {
	cb, ok := cbInterface.(*CircuitBreakerState)
	if !ok {
		return fmt.Errorf("invalid circuit breaker state type")
	}
	pricesJSON, err := json.Marshal(cb.RecentPrices)
	if err != nil {
		return fmt.Errorf("failed to marshal prices: %w", err)
	}

	var metaJSON []byte
	if cb.Metadata != nil {
		metaJSON, err = json.Marshal(cb.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	query := `
		INSERT INTO metering.circuit_breaker_state
			(pair, recent_prices, last_trade_ts, state, failures, opened_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (pair)
		DO UPDATE SET
			recent_prices = EXCLUDED.recent_prices,
			last_trade_ts = EXCLUDED.last_trade_ts,
			state = EXCLUDED.state,
			failures = EXCLUDED.failures,
			opened_at = EXCLUDED.opened_at,
			metadata = EXCLUDED.metadata,
			updated_at = now()
	`

	_, err = s.db.ExecContext(ctx, query,
		cb.Pair, pricesJSON, cb.LastTradeTs, cb.State, cb.Failures, cb.OpenedAt, metaJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to save circuit breaker state: %w", err)
	}

	return nil
}

type RouterAuditLog struct {
	Event      string
	PartnerID  *string
	Severity   string
	DurationMs *int
	Outcome    string
	Metadata   map[string]interface{}
	ErrorCode  *string
}

func (s *RouterStore) LogAudit(ctx context.Context, logInterface interface{}) error {
	log, ok := logInterface.(*RouterAuditLog)
	if !ok {
		// If it's a map, convert it
		if logMap, isMap := logInterface.(map[string]interface{}); isMap {
			log = &RouterAuditLog{
				Event:    getString(logMap, "event"),
				Severity: getString(logMap, "severity"),
				Outcome:  getString(logMap, "outcome"),
				Metadata: getMap(logMap, "metadata"),
			}
			if ms, ok := logMap["duration_ms"].(int); ok {
				log.DurationMs = &ms
			}
			if ec, ok := logMap["error_code"].(string); ok {
				log.ErrorCode = &ec
			}
		} else {
			return fmt.Errorf("invalid audit log type")
		}
	}
	var metaJSON []byte
	var err error
	if log.Metadata != nil {
		metaJSON, err = json.Marshal(log.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	query := `
		INSERT INTO metering.router_audit
			(event, partner_id, severity, duration_ms, outcome, metadata, error_code)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = s.db.ExecContext(ctx, query,
		log.Event, log.PartnerID, log.Severity, log.DurationMs, log.Outcome, metaJSON, log.ErrorCode,
	)
	if err != nil {
		return fmt.Errorf("failed to log audit: %w", err)
	}

	return nil
}

func (s *RouterStore) CleanupExpiredRateLimits(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx, `SELECT cleanup_expired_rate_limits()`)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup rate limits: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rows, nil
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if v, ok := m[key].(map[string]interface{}); ok {
		return v
	}
	return nil
}

func (s *RouterStore) CleanupUsedQuotes(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx, `SELECT cleanup_used_quotes()`)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup used quotes: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rows, nil
}
