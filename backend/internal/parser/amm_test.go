package parser

import (
	"testing"
)

func TestNewAMMParser(t *testing.T) {
	parser := NewAMMParser()
	
	if parser == nil {
		t.Fatal("NewAMMParser() returned nil")
	}
}

func TestAMMParser_ParseTransaction(t *testing.T) {
	parser := NewAMMParser()
	
	tests := []struct {
		name        string
		tx          map[string]interface{}
		ledgerIndex uint64
		ledgerHash  string
		wantNil     bool
		wantErr     bool
		check       func(*testing.T, map[string]interface{})
	}{
		{
			name: "AMMCreate with XRP and IOU",
			tx: map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Account":         "rAMMCreator",
				"Amount":          "1000000", // XRP
				"Amount2": map[string]interface{}{
					"currency": "USD",
					"issuer":   "rN7n7otQDd6FczFgLdlqtyMVrn3HMfWi",
					"value":    "500.50",
				},
				"TradingFee": float64(30),
			},
			ledgerIndex: 12345,
			ledgerHash:  "HASH123",
			wantNil:     false,
			wantErr:     false,
		},
		{
			name: "AMMCreate missing Account",
			tx: map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Amount":          "1000000",
				"Amount2": map[string]interface{}{
					"currency": "USD",
					"issuer":   "rN7n7",
					"value":    "500",
				},
			},
			ledgerIndex: 12345,
			ledgerHash:  "HASH123",
			wantNil:     true, // Error case returns nil pool
			wantErr:     true,
		},
		{
			name: "AMMCreate missing Amount",
			tx: map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Account":         "rAMMCreator",
				"Amount2": map[string]interface{}{
					"currency": "USD",
					"issuer":   "rN7n7",
					"value":    "500",
				},
			},
			ledgerIndex: 12345,
			ledgerHash:  "HASH123",
			wantNil:     true, // Error case returns nil pool
			wantErr:     true,
		},
		{
			name: "AMMDeposit (not fully implemented)",
			tx: map[string]interface{}{
				"TransactionType": "AMMDeposit",
				"Account":         "rDepositor",
			},
			ledgerIndex: 12345,
			ledgerHash:  "HASH123",
			wantNil:     true, // Returns nil until metadata parsing implemented
			wantErr:     false,
		},
		{
			name: "Non-AMM transaction",
			tx: map[string]interface{}{
				"TransactionType": "Payment",
				"Account":         "rSender",
			},
			ledgerIndex: 12345,
			ledgerHash:  "HASH123",
			wantNil:     true, // Not an AMM transaction
			wantErr:     false,
		},
		{
			name: "Missing TransactionType",
			tx: map[string]interface{}{
				"Account": "rSender",
			},
			ledgerIndex: 12345,
			ledgerHash:  "HASH123",
			wantNil:     true, // Error case returns nil pool
			wantErr:     true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := parser.ParseTransaction(tt.tx, tt.ledgerIndex, tt.ledgerHash)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTransaction() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if (pool == nil) != tt.wantNil {
				t.Errorf("ParseTransaction() pool = %v, wantNil %v", pool, tt.wantNil)
				return
			}
			
			if !tt.wantNil && !tt.wantErr && pool != nil {
				// Verify pool was populated
				if pool.Account == "" {
					t.Error("pool.Account is empty")
				}
				if pool.LedgerIndex != int64(tt.ledgerIndex) {
					t.Errorf("pool.LedgerIndex = %v, want %v", pool.LedgerIndex, tt.ledgerIndex)
				}
				if pool.LedgerHash != tt.ledgerHash {
					t.Errorf("pool.LedgerHash = %v, want %v", pool.LedgerHash, tt.ledgerHash)
				}
			}
		})
	}
}

func TestParseAmount(t *testing.T) {
	tests := []struct {
		name       string
		amount     interface{}
		wantAsset  string
		wantValue  string
		wantErr    bool
	}{
		{
			name:       "XRP as string",
			amount:     "1000000",
			wantAsset:  "XRP",
			wantValue:  "1000000",
			wantErr:    false,
		},
		{
			name: "IOU object",
			amount: map[string]interface{}{
				"currency": "USD",
				"issuer":   "rN7n7otQDd6FczFgLdlqtyMVrn3HMfWi",
				"value":    "500.50",
			},
			wantAsset: "USD.rN7n7otQDd6FczFgLdlqtyMVrn3HMfWi",
			wantValue: "500.50",
			wantErr:   false,
		},
		{
			name:       "XRP zero",
			amount:     "0",
			wantAsset:  "XRP",
			wantValue:  "0",
			wantErr:    false,
		},
		{
			name: "IOU missing currency",
			amount: map[string]interface{}{
				"issuer": "rN7n7",
				"value":  "500",
			},
			wantAsset: "",
			wantValue: "",
			wantErr:   true,
		},
		{
			name:       "invalid type (number)",
			amount:     123,
			wantAsset:  "",
			wantValue:  "",
			wantErr:    true,
		},
		{
			name:       "invalid type (array)",
			amount:     []string{"invalid"},
			wantAsset:  "",
			wantValue:  "",
			wantErr:    true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset, value, err := parseAmount(tt.amount)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAmount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if asset != tt.wantAsset {
				t.Errorf("asset = %v, want %v", asset, tt.wantAsset)
			}
			
			if value != tt.wantValue {
				t.Errorf("value = %v, want %v", value, tt.wantValue)
			}
		})
	}
}

func TestAMMParser_parseAMMCreate(t *testing.T) {
	parser := NewAMMParser()
	
	tests := []struct {
		name    string
		tx      map[string]interface{}
		wantErr bool
		check   func(*testing.T, map[string]interface{})
	}{
		{
			name: "valid XRP/USD pool",
			tx: map[string]interface{}{
				"Account": "rAMMAccount",
				"Amount":  "1000000",
				"Amount2": map[string]interface{}{
					"currency": "USD",
					"issuer":   "rN7n7",
					"value":    "500",
				},
				"TradingFee": float64(30),
			},
			wantErr: false,
			check: func(t *testing.T, tx map[string]interface{}) {
				// Test will verify via ParseTransaction
			},
		},
		{
			name: "valid IOU/IOU pool",
			tx: map[string]interface{}{
				"Account": "rAMMAccount",
				"Amount": map[string]interface{}{
					"currency": "EUR",
					"issuer":   "rEUR",
					"value":    "1000",
				},
				"Amount2": map[string]interface{}{
					"currency": "USD",
					"issuer":   "rUSD",
					"value":    "1200",
				},
			},
			wantErr: false,
		},
		{
			name: "missing Amount2",
			tx: map[string]interface{}{
				"Account": "rAMMAccount",
				"Amount":  "1000000",
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := parser.parseAMMCreate(tt.tx, 12345, "HASH")
			
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAMMCreate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr && pool == nil {
				t.Error("parseAMMCreate() returned nil pool")
			}
		})
	}
}

// Benchmark for AMM parsing
func BenchmarkAMMParser_ParseTransaction(b *testing.B) {
	parser := NewAMMParser()
	tx := map[string]interface{}{
		"TransactionType": "AMMCreate",
		"Account":         "rAMMAccount",
		"Amount":          "1000000",
		"Amount2": map[string]interface{}{
			"currency": "USD",
			"issuer":   "rN7n7",
			"value":    "500",
		},
		"TradingFee": float64(30),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.ParseTransaction(tx, 12345, "HASH")
	}
}

func BenchmarkParseAmount_XRP(b *testing.B) {
	amount := "1000000"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseAmount(amount)
	}
}

func BenchmarkParseAmount_IOU(b *testing.B) {
	amount := map[string]interface{}{
		"currency": "USD",
		"issuer":   "rN7n7otQDd6FczFgLdlqtyMVrn3HMfWi",
		"value":    "500.50",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseAmount(amount)
	}
}
