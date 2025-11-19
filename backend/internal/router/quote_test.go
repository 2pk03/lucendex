package router

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

type mockKV struct {
	data      map[[32]byte][]byte
	ledgerIdx uint32
}

func (m *mockKV) GetQuote(hash [32]byte) ([]byte, bool) {
	val, ok := m.data[hash]
	return val, ok
}

func (m *mockKV) SetQuote(hash [32]byte, route []byte, ttl time.Duration) error {
	if m.data == nil {
		m.data = make(map[[32]byte][]byte)
	}
	m.data[hash] = route
	return nil
}

func (m *mockKV) SetLedgerIndex(idx uint32) error {
	m.ledgerIdx = idx
	return nil
}

func (m *mockKV) GetLedgerIndex() (uint32, bool) {
	if m.ledgerIdx == 0 {
		return 0, false
	}
	return m.ledgerIdx, true
}

func TestQuoteEngine_GenerateQuote(t *testing.T) {
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

	qe := NewQuoteEngine(validator, pathfinder, breaker, kv, 20)

	req := &QuoteRequest{
		In:     Asset{Currency: "XRP"},
		Out:    Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
		Amount: decimal.NewFromInt(100),
	}

	quote, err := qe.GenerateQuote(context.Background(), req, 12345)
	if err != nil {
		t.Fatalf("GenerateQuote() error = %v", err)
	}

	if quote == nil {
		t.Fatal("Expected quote, got nil")
	}
	if quote.Out.LessThanOrEqual(decimal.Zero) {
		t.Error("Quote output should be positive")
	}
	if quote.Price.LessThanOrEqual(decimal.Zero) {
		t.Error("Price should be positive")
	}
	if quote.Fees.RouterBps != 20 {
		t.Errorf("RouterBps = %d, want 20", quote.Fees.RouterBps)
	}
	if quote.LedgerIndex != 12345 {
		t.Errorf("LedgerIndex = %d, want 12345", quote.LedgerIndex)
	}
}

func TestQuoteEngine_ValidationFailure(t *testing.T) {
	qe := NewQuoteEngine(NewValidator(), nil, nil, nil, 20)

	req := &QuoteRequest{
		In:     Asset{Currency: "XRP"},
		Out:    Asset{Currency: "XRP"},
		Amount: decimal.NewFromInt(100),
	}

	_, err := qe.GenerateQuote(context.Background(), req, 12345)
	if err != ErrSameAssets {
		t.Errorf("GenerateQuote() error = %v, want %v", err, ErrSameAssets)
	}
}

func TestQuoteEngine_CircuitBreakerRejection(t *testing.T) {
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
	breaker := NewCircuitBreaker(0.01)
	breaker.mu.Lock()
	breaker.cautionMode = false
	breaker.mu.Unlock()
	kv := &mockKV{}

	qe := NewQuoteEngine(validator, pathfinder, breaker, kv, 20)

	pair := "XRP-USD.rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"
	normalPrice := decimal.NewFromFloat(1.5)
	for i := 0; i < 20; i++ {
		breaker.RecordTrade(pair, normalPrice)
	}

	req := &QuoteRequest{
		In:     Asset{Currency: "XRP"},
		Out:    Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
		Amount: decimal.NewFromInt(100),
	}

	_, err := qe.GenerateQuote(context.Background(), req, 12345)
	if err != ErrCircuitBreakerOpen {
		t.Errorf("GenerateQuote() error = %v, want %v", err, ErrCircuitBreakerOpen)
	}
}

func TestQuoteEngine_FeeCalculation(t *testing.T) {
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

	qe := NewQuoteEngine(validator, pathfinder, breaker, kv, 20)

	req := &QuoteRequest{
		In:     Asset{Currency: "XRP"},
		Out:    Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
		Amount: decimal.NewFromInt(100),
	}

	quote, err := qe.GenerateQuote(context.Background(), req, 12345)
	if err != nil {
		t.Fatalf("GenerateQuote() error = %v", err)
	}

	if quote.Fees.RouterBps != 20 {
		t.Errorf("RouterBps = %d, want 20", quote.Fees.RouterBps)
	}

	t.Logf("TradingFees: %s", quote.Fees.TradingFees)
}
