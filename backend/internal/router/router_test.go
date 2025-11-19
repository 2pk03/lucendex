package router

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
)

type mockStore struct {
	auditLogs []map[string]interface{}
}

func (m *mockStore) GetCircuitBreakerState(ctx context.Context, pair string) (interface{}, error) {
	return nil, nil
}

func (m *mockStore) SaveCircuitBreakerState(ctx context.Context, cb interface{}) error {
	return nil
}

func (m *mockStore) LogAudit(ctx context.Context, log interface{}) error {
	if m.auditLogs == nil {
		m.auditLogs = make([]map[string]interface{}, 0)
	}
	if logMap, ok := log.(map[string]interface{}); ok {
		m.auditLogs = append(m.auditLogs, logMap)
	}
	return nil
}

func TestRouter_Quote(t *testing.T) {
	pools := []AMMPool{
		{
			Asset1:        Asset{Currency: "XRP"},
			Asset2:        Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
			Asset1Reserve: decimal.NewFromInt(10000),
			Asset2Reserve: decimal.NewFromInt(15000),
			TradingFeeBps: 30,
		},
	}

	validator := NewValidator()
	pathfinder := NewPathfinder(pools, nil)
	breaker := NewCircuitBreaker(0.05)
	breaker.mu.Lock()
	breaker.cautionMode = false
	breaker.mu.Unlock()
	kv := &mockKV{}
	store := &mockStore{}

	qe := NewQuoteEngine(validator, pathfinder, breaker, kv, 20)
	r := NewRouter(qe, store, kv)
	defer r.Close()

	req := &QuoteRequest{
		In:     Asset{Currency: "XRP"},
		Out:    Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
		Amount: decimal.NewFromInt(100),
	}

	quote, err := r.Quote(context.Background(), req, 12345)
	if err != nil {
		t.Fatalf("Quote() error = %v", err)
	}

	if quote == nil {
		t.Fatal("Expected quote, got nil")
	}

	if len(store.auditLogs) == 0 {
		t.Error("No audit logs recorded")
	}

	if store.auditLogs[0]["event"] != "quote_request" {
		t.Errorf("Audit event = %v, want quote_request", store.auditLogs[0]["event"])
	}
	if store.auditLogs[0]["outcome"] != "success" {
		t.Errorf("Audit outcome = %v, want success", store.auditLogs[0]["outcome"])
	}
}

func TestRouter_QuoteValidationFailure(t *testing.T) {
	validator := NewValidator()
	store := &mockStore{}

	qe := NewQuoteEngine(validator, nil, nil, nil, 20)
	r := NewRouter(qe, store, nil)
	defer r.Close()

	req := &QuoteRequest{
		In:     Asset{Currency: "XRP"},
		Out:    Asset{Currency: "XRP"},
		Amount: decimal.NewFromInt(100),
	}

	_, err := r.Quote(context.Background(), req, 12345)
	if err == nil {
		t.Error("Expected validation error, got nil")
	}

	if len(store.auditLogs) == 0 {
		t.Error("No audit logs for failed quote")
	}

	if store.auditLogs[0]["outcome"] != "rejected" {
		t.Errorf("Audit outcome = %v, want rejected", store.auditLogs[0]["outcome"])
	}
}
