package router

import (
	"context"
	"sync"
	"time"
)

type Router struct {
	quoteEngine      *QuoteEngine
	validator        *Validator
	pathfinder       *Pathfinder
	breaker          *CircuitBreaker
	store            RouterStoreInterface
	kv               KVStore
	mu               sync.RWMutex
	stopped          bool
	currentLedgerIdx uint32
}

type RouterStoreInterface interface {
	GetCircuitBreakerState(ctx context.Context, pair string) (interface{}, error)
	SaveCircuitBreakerState(ctx context.Context, cb interface{}) error
	LogAudit(ctx context.Context, log interface{}) error
}

func NewRouter(quoteEngine *QuoteEngine, store RouterStoreInterface) *Router {
	return &Router{
		quoteEngine: quoteEngine,
		store:       store,
	}
}

func (r *Router) Quote(ctx context.Context, req *QuoteRequest, ledgerIndex uint32) (*QuoteResponse, error) {
	start := time.Now()

	quote, err := r.quoteEngine.GenerateQuote(ctx, req, ledgerIndex)
	
	durationMs := int(time.Since(start).Milliseconds())
	
	outcome := "success"
	severity := "info"
	var errorCode *string
	
	if err != nil {
		outcome = "rejected"
		severity = "warn"
		code := err.Error()
		errorCode = &code
	}

	auditLog := map[string]interface{}{
		"event":       "quote_request",
		"severity":    severity,
		"duration_ms": durationMs,
		"outcome":     outcome,
		"metadata": map[string]interface{}{
			"pair": req.In.String() + "-" + req.Out.String(),
		},
	}
	if errorCode != nil {
		auditLog["error_code"] = *errorCode
	}
	
	_ = r.store.LogAudit(ctx, auditLog)

	return quote, err
}

func (r *Router) GenerateQuote(ctx context.Context, req *QuoteRequest, ledgerIndex uint32) (*QuoteResponse, error) {
	return r.Quote(ctx, req, ledgerIndex)
}

func (r *Router) GetCurrentLedgerIndex() uint32 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentLedgerIdx
}

func (r *Router) SetCurrentLedgerIndex(idx uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.currentLedgerIdx = idx
}

func (r *Router) GetAvailablePairs(ctx context.Context) ([]TradingPairInfo, error) {
	return []TradingPairInfo{}, nil
}

func (r *Router) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.stopped {
		return nil
	}
	r.stopped = true

	return nil
}
