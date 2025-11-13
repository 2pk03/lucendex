package api

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Request types
type QuoteRequest struct {
	In     string `json:"in"`
	Out    string `json:"out"`
	Amount string `json:"amount"`
}

type UsageQueryParams struct {
	Month string `json:"month"` // YYYY-MM format
	Limit int    `json:"limit"`
	Offset int   `json:"offset"`
}

// Response types
type QuoteResponse struct {
	QuoteHash   string          `json:"quote_hash"`
	Route       RouteResponse   `json:"route"`
	AmountOut   string          `json:"amount_out"`
	Price       string          `json:"price"`
	Fees        FeesResponse    `json:"fees"`
	LedgerIndex uint32          `json:"ledger_index"`
	TTL         uint16          `json:"ttl_ledgers"`
	ExpiresAt   string          `json:"expires_at"`
}

type RouteResponse struct {
	Hops        []HopResponse `json:"hops"`
	PriceImpact string        `json:"price_impact"`
}

type HopResponse struct {
	Type      string `json:"type"`
	In        string `json:"in"`
	Out       string `json:"out"`
	AmountIn  string `json:"amount_in"`
	AmountOut string `json:"amount_out"`
}

type FeesResponse struct {
	RouterBps   int    `json:"router_bps"`
	TradingFees string `json:"trading_fees"`
	EstOutFee   string `json:"est_out_fee"`
}

type PairsResponse struct {
	Pairs []TradingPair `json:"pairs"`
}

type TradingPair struct {
	In          string `json:"in"`
	Out         string `json:"out"`
	Liquidity   string `json:"liquidity"`
	AvgSpread   string `json:"avg_spread"`
	DailyVolume string `json:"daily_volume"`
}

type UsageResponse struct {
	Month       string        `json:"month"`
	TxCount     int64         `json:"tx_count"`
	TotalVolume string        `json:"total_volume"`
	TotalFees   string        `json:"total_fees_usd"`
	Details     []UsageDetail `json:"details"`
}

type UsageDetail struct {
	QuoteHash   string    `json:"quote_hash"`
	Pair        string    `json:"pair"`
	AmountIn    string    `json:"amount_in"`
	AmountOut   string    `json:"amount_out"`
	FeeAmount   string    `json:"fee_amount"`
	TxHash      string    `json:"tx_hash"`
	LedgerIndex int64     `json:"ledger_index"`
	Timestamp   time.Time `json:"timestamp"`
}

type HealthResponse struct {
	Status           string `json:"status"`
	IndexerLag       int    `json:"indexer_lag_ledgers"`
	LastLedgerIndex  uint32 `json:"last_ledger_index"`
	QuoteCacheHits   int64  `json:"quote_cache_hits"`
	QuoteCacheMisses int64  `json:"quote_cache_misses"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// Database models
type Partner struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	Plan      string    `db:"plan"`
	RouterBps int       `db:"router_bps"`
	Status    string    `db:"status"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type APIKey struct {
	ID         uuid.UUID  `db:"id"`
	PartnerID  uuid.UUID  `db:"partner_id"`
	PublicKey  string     `db:"public_key"`
	Label      string     `db:"label"`
	CreatedAt  time.Time  `db:"created_at"`
	Revoked    bool       `db:"revoked"`
	RevokedAt  *time.Time `db:"revoked_at"`
}

type QuoteRegistry struct {
	QuoteHash  []byte          `db:"quote_hash"`
	PartnerID  uuid.UUID       `db:"partner_id"`
	Route      string          `db:"route"` // JSONB stored as string
	AmountIn   decimal.Decimal `db:"amount_in"`
	AmountOut  decimal.Decimal `db:"amount_out"`
	RouterBps  int             `db:"router_bps"`
	ExpiresAt  time.Time       `db:"expires_at"`
	CreatedAt  time.Time       `db:"created_at"`
}

type UsageEvent struct {
	ID          int64           `db:"id"`
	PartnerID   uuid.UUID       `db:"partner_id"`
	QuoteHash   []byte          `db:"quote_hash"`
	Pair        string          `db:"pair"`
	AmountIn    decimal.Decimal `db:"amount_in"`
	AmountOut   decimal.Decimal `db:"amount_out"`
	RouterBps   int             `db:"router_bps"`
	FeeAmount   decimal.Decimal `db:"fee_amount"`
	TxHash      string          `db:"tx_hash"`
	LedgerIndex int64           `db:"ledger_index"`
	Timestamp   time.Time       `db:"ts"`
}

// Context keys
type contextKey string

const (
	ContextKeyPartnerID contextKey = "partner_id"
	ContextKeyPartner   contextKey = "partner"
)
