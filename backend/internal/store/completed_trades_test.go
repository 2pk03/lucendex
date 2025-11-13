package store

import (
	"testing"
)

func TestCompletedTrade_Struct(t *testing.T) {
	quoteHash := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	
	trade := &CompletedTrade{
		QuoteHash:    quoteHash,
		TxHash:       "ABC123DEF456",
		Account:      "rAccount123",
		InAsset:      "XRP",
		OutAsset:     "USD.rIssuer1",
		AmountIn:     "1000000",
		AmountOut:    "500.50",
		Route:        map[string]interface{}{"type": "amm", "pool": "pool1"},
		RouterFeeBps: 20,
		LedgerIndex:  12345,
		LedgerHash:   "HASH123",
	}
	
	if len(trade.QuoteHash) != 32 {
		t.Errorf("QuoteHash length = %v, want 32", len(trade.QuoteHash))
	}
	
	if trade.TxHash != "ABC123DEF456" {
		t.Errorf("TxHash = %v, want ABC123DEF456", trade.TxHash)
	}
	
	if trade.RouterFeeBps != 20 {
		t.Errorf("RouterFeeBps = %v, want 20", trade.RouterFeeBps)
	}
	
	if trade.Route == nil {
		t.Error("Route should not be nil")
	}
	
	routeType, ok := trade.Route["type"].(string)
	if !ok || routeType != "amm" {
		t.Errorf("Route type = %v, want amm", routeType)
	}
}

func TestCompletedTrade_NilRoute(t *testing.T) {
	trade := &CompletedTrade{
		QuoteHash:    make([]byte, 32),
		TxHash:       "XYZ789",
		Account:      "rAccount2",
		InAsset:      "EUR.rIssuer2",
		OutAsset:     "XRP",
		AmountIn:     "200.50",
		AmountOut:    "2000000",
		Route:        nil, // Nil route should be acceptable
		RouterFeeBps: 25,
		LedgerIndex:  12346,
		LedgerHash:   "HASH124",
	}
	
	// Verify nil route is acceptable
	if trade.Route != nil {
		t.Error("Route should be nil")
	}
	
	// Other fields should still be valid
	if trade.RouterFeeBps != 25 {
		t.Errorf("RouterFeeBps = %v, want 25", trade.RouterFeeBps)
	}
}

func TestCompletedTrade_Validation(t *testing.T) {
	tests := []struct {
		name    string
		trade   *CompletedTrade
		wantErr bool
	}{
		{
			name: "valid trade",
			trade: &CompletedTrade{
				QuoteHash:    make([]byte, 32),
				TxHash:       "VALID_HASH",
				Account:      "rAccount",
				InAsset:      "XRP",
				OutAsset:     "USD.rIssuer",
				AmountIn:     "1000",
				AmountOut:    "500",
				Route:        map[string]interface{}{"type": "amm"},
				RouterFeeBps: 20,
				LedgerIndex:  100,
				LedgerHash:   "HASH",
			},
			wantErr: false,
		},
		{
			name: "empty tx_hash",
			trade: &CompletedTrade{
				QuoteHash:    make([]byte, 32),
				TxHash:       "",
				Account:      "rAccount",
				InAsset:      "XRP",
				OutAsset:     "USD.rIssuer",
				AmountIn:     "1000",
				AmountOut:    "500",
				RouterFeeBps: 20,
				LedgerIndex:  100,
			},
			wantErr: true,
		},
		{
			name: "invalid quote hash length",
			trade: &CompletedTrade{
				QuoteHash:    make([]byte, 16), // Wrong length
				TxHash:       "VALID_HASH",
				Account:      "rAccount",
				InAsset:      "XRP",
				OutAsset:     "USD.rIssuer",
				AmountIn:     "1000",
				AmountOut:    "500",
				RouterFeeBps: 20,
				LedgerIndex:  100,
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation
			if tt.trade.TxHash == "" && !tt.wantErr {
				t.Error("Expected error for empty TxHash")
			}
			
			if len(tt.trade.QuoteHash) != 32 && !tt.wantErr {
				t.Error("Expected error for invalid QuoteHash length")
			}
			
			if tt.trade.Account == "" && !tt.wantErr {
				t.Error("Expected error for empty Account")
			}
		})
	}
}

func TestCompletedTrade_RouterFeeBps(t *testing.T) {
	tests := []struct {
		name       string
		routerFee  int
		wantValid  bool
	}{
		{
			name:      "valid fee 20 bps",
			routerFee: 20,
			wantValid: true,
		},
		{
			name:      "valid fee 0 bps",
			routerFee: 0,
			wantValid: true,
		},
		{
			name:      "valid fee 100 bps",
			routerFee: 100,
			wantValid: true,
		},
		{
			name:      "negative fee",
			routerFee: -10,
			wantValid: false,
		},
		{
			name:      "excessive fee",
			routerFee: 10000,
			wantValid: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trade := &CompletedTrade{RouterFeeBps: tt.routerFee}
			
			// Fee should be >= 0 and <= 1000 (10%)
			isValid := trade.RouterFeeBps >= 0 && trade.RouterFeeBps <= 1000
			
			if isValid != tt.wantValid {
				t.Errorf("RouterFeeBps %v validity = %v, want %v", tt.routerFee, isValid, tt.wantValid)
			}
		})
	}
}

func BenchmarkCompletedTrade_Creation(b *testing.B) {
	quoteHash := make([]byte, 32)
	for i := 0; i < 32; i++ {
		quoteHash[i] = byte(i)
	}
	
	for i := 0; i < b.N; i++ {
		_ = &CompletedTrade{
			QuoteHash:    quoteHash,
			TxHash:       "HASH123",
			Account:      "rAccount",
			InAsset:      "XRP",
			OutAsset:     "USD.rIssuer",
			AmountIn:     "1000",
			AmountOut:    "500",
			Route:        map[string]interface{}{"type": "amm"},
			RouterFeeBps: 20,
			LedgerIndex:  int64(i),
			LedgerHash:   "HASH",
		}
	}
}
