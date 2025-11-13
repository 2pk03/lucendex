package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/lucendex/backend/internal/router"
)

type Handlers struct {
	router *router.Router
	db     DB
	kv     KVStore
}

func NewHandlers(r *router.Router, db DB, kv KVStore) *Handlers {
	return &Handlers{
		router: r,
		db:     db,
		kv:     kv,
	}
}

// QuoteHandler handles POST /partner/v1/quote
func (h *Handlers) QuoteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx := r.Context()
	partnerID := ctx.Value(ContextKeyPartnerID).(uuid.UUID)
	partner := ctx.Value(ContextKeyPartner).(*Partner)

	// Parse request
	var req QuoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Validate input
	if req.In == "" || req.Out == "" || req.Amount == "" {
		writeError(w, http.StatusBadRequest, "missing required fields")
		return
	}

	// Parse amount
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid amount format")
		return
	}

	if amount.LessThanOrEqual(decimal.Zero) {
		writeError(w, http.StatusBadRequest, "amount must be positive")
		return
	}

	// Convert to router types
	inAsset := parseAsset(req.In)
	outAsset := parseAsset(req.Out)

	routerReq := &router.QuoteRequest{
		In:     inAsset,
		Out:    outAsset,
		Amount: amount,
	}

	// Get current ledger index
	ledgerIndex := h.router.GetCurrentLedgerIndex()

	// Generate quote
	quote, err := h.router.GenerateQuote(ctx, routerReq, ledgerIndex)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Store quote registry for later attribution
	expiresAt := time.Now().Add(time.Duration(quote.TTLLedgers) * 4 * time.Second)
	if err := h.storeQuoteRegistry(ctx, quote, partnerID, partner.RouterBps, expiresAt); err != nil {
		// Log error but don't fail - quote is still valid
		// In production would use proper logging
	}

	// Build response
	resp := h.buildQuoteResponse(quote, expiresAt)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// PairsHandler handles GET /partner/v1/pairs
func (h *Handlers) PairsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx := r.Context()

	// Get available pairs from router
	pairs, err := h.router.GetAvailablePairs(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch pairs")
		return
	}

	// Convert to API response format
	apiPairs := make([]TradingPair, len(pairs))
	for i, p := range pairs {
		apiPairs[i] = TradingPair{
			In:          p.In.String(),
			Out:         p.Out.String(),
			Liquidity:   p.Liquidity.String(),
			AvgSpread:   p.AvgSpread.String(),
			DailyVolume: p.DailyVolume.String(),
		}
	}

	resp := PairsResponse{
		Pairs: apiPairs,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// UsageHandler handles GET /partner/v1/usage
func (h *Handlers) UsageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx := r.Context()
	partnerID := ctx.Value(ContextKeyPartnerID).(uuid.UUID)

	// Parse query parameters
	month := r.URL.Query().Get("month")
	if month == "" {
		month = time.Now().Format("2006-01")
	}

	// Validate month format
	if _, err := time.Parse("2006-01", month); err != nil {
		writeError(w, http.StatusBadRequest, "invalid month format (use YYYY-MM)")
		return
	}

	// Fetch usage data
	usage, err := h.db.GetPartnerUsage(ctx, partnerID, month)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch usage")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(usage)
}

// HealthHandler handles GET /partner/v1/health
func (h *Handlers) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx := r.Context()

	// Get system health metrics
	health, err := h.getSystemHealth(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch health")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(health)
}

// Helper functions

func (h *Handlers) storeQuoteRegistry(ctx context.Context, quote *router.QuoteResponse, partnerID uuid.UUID, routerBps int, expiresAt time.Time) error {
	routeJSON, err := json.Marshal(quote.Route)
	if err != nil {
		return err
	}

	registry := &QuoteRegistry{
		QuoteHash: quote.QuoteHash[:],
		PartnerID: partnerID,
		Route:     string(routeJSON),
		AmountIn:  quote.Route.Hops[0].AmountIn,
		AmountOut: quote.Out,
		RouterBps: routerBps,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}

	return h.db.StoreQuoteRegistry(ctx, registry)
}

func (h *Handlers) buildQuoteResponse(quote *router.QuoteResponse, expiresAt time.Time) QuoteResponse {
	hops := make([]HopResponse, len(quote.Route.Hops))
	for i, hop := range quote.Route.Hops {
		hops[i] = HopResponse{
			Type:      hop.Type,
			In:        hop.In.String(),
			Out:       hop.Out.String(),
			AmountIn:  hop.AmountIn.String(),
			AmountOut: hop.AmountOut.String(),
		}
	}

	return QuoteResponse{
		QuoteHash: hex.EncodeToString(quote.QuoteHash[:]),
		Route: RouteResponse{
			Hops:        hops,
			PriceImpact: quote.Route.PriceImpact.String(),
		},
		AmountOut:   quote.Out.String(),
		Price:       quote.Price.String(),
		Fees: FeesResponse{
			RouterBps:   quote.Fees.RouterBps,
			TradingFees: quote.Fees.TradingFees.String(),
			EstOutFee:   quote.Fees.EstOutFee.String(),
		},
		LedgerIndex: quote.LedgerIndex,
		TTL:         quote.TTLLedgers,
		ExpiresAt:   expiresAt.Format(time.RFC3339),
	}
}

func (h *Handlers) getSystemHealth(ctx context.Context) (*HealthResponse, error) {
	// Get indexer lag
	lag, err := h.db.GetIndexerLag(ctx)
	if err != nil {
		lag = -1 // Unknown
	}

	// Get last ledger index
	lastLedger := h.router.GetCurrentLedgerIndex()

	// Get cache stats
	hits, misses := h.getCacheStats()

	// Determine status
	status := "ok"
	if lag > 10 {
		status = "degraded"
	}
	if lag > 50 {
		status = "down"
	}

	return &HealthResponse{
		Status:           status,
		IndexerLag:       lag,
		LastLedgerIndex:  lastLedger,
		QuoteCacheHits:   hits,
		QuoteCacheMisses: misses,
	}, nil
}

func (h *Handlers) getCacheStats() (hits int64, misses int64) {
	// Placeholder - would get from KV metrics
	return 0, 0
}

func parseAsset(s string) router.Asset {
	parts := strings.Split(s, ".")
	if len(parts) == 1 {
		return router.Asset{
			Currency: parts[0],
			Issuer:   "",
		}
	}
	return router.Asset{
		Currency: parts[0],
		Issuer:   parts[1],
	}
}

// Additional DB interface methods needed
type DBExtended interface {
	DB
	GetPartnerUsage(ctx context.Context, partnerID uuid.UUID, month string) (*UsageResponse, error)
	StoreQuoteRegistry(ctx context.Context, registry *QuoteRegistry) error
	GetIndexerLag(ctx context.Context) (int, error)
}
