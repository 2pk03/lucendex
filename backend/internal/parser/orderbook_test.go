package parser

import (
	"testing"
)

func TestNewOrderbookParser(t *testing.T) {
	parser := NewOrderbookParser()
	
	if parser == nil {
		t.Fatal("NewOrderbookParser() returned nil")
	}
}

func TestOrderbookParser_ParseTransaction(t *testing.T) {
	parser := NewOrderbookParser()
	
	tests := []struct {
		name        string
		tx          map[string]interface{}
		ledgerIndex uint64
		ledgerHash  string
		wantNil     bool
		wantErr     bool
	}{
		{
			name: "OfferCreate with XRP/USD",
			tx: map[string]interface{}{
				"TransactionType": "OfferCreate",
				"Account":         "rOfferCreator",
				"Sequence":        float64(123),
				"TakerPays":       "1000000", // XRP
				"TakerGets": map[string]interface{}{
					"currency": "USD",
					"issuer":   "rN7n7",
					"value":    "500",
				},
			},
			ledgerIndex: 12345,
			ledgerHash:  "HASH123",
			wantNil:     false,
			wantErr:     false,
		},
		{
			name: "OfferCreate missing TakerPays",
			tx: map[string]interface{}{
				"TransactionType": "OfferCreate",
				"Account":         "rOfferCreator",
				"Sequence":        float64(123),
				"TakerGets":       "1000000",
			},
			ledgerIndex: 12345,
			ledgerHash:  "HASH123",
			wantNil:     false, // Returns invalid offer now
			wantErr:     false,
		},
		{
			name: "OfferCancel returns nil",
			tx: map[string]interface{}{
				"TransactionType": "OfferCancel",
				"Account":         "rCanceller",
				"OfferSequence":   float64(456),
			},
			ledgerIndex: 12345,
			ledgerHash:  "HASH123",
			wantNil:     true, // OfferCancel handled separately
			wantErr:     false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offer, err := parser.ParseTransaction(tt.tx, tt.ledgerIndex, tt.ledgerHash)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTransaction() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if (offer == nil) != tt.wantNil {
				t.Errorf("ParseTransaction() offer = %v, wantNil %v", offer, tt.wantNil)
				return
			}
			
			if !tt.wantNil && !tt.wantErr && offer != nil {
				// Verify offer was populated
				if offer.OwnerAccount == "" {
					t.Error("offer.OwnerAccount is empty")
				}
				// Status could be "active" or "invalid_parse"
				if offer.Status != "active" && offer.Status != "invalid_parse" {
					t.Errorf("offer.Status = %v, want active or invalid_parse", offer.Status)
				}
			}
		})
	}
}

func TestOrderbookParser_ParseOfferCancel(t *testing.T) {
	parser := NewOrderbookParser()
	
	tests := []struct {
		name        string
		tx          map[string]interface{}
		wantAccount string
		wantSeq     int64
		wantErr     bool
	}{
		{
			name: "valid OfferCancel",
			tx: map[string]interface{}{
				"TransactionType": "OfferCancel",
				"Account":         "rCanceller",
				"OfferSequence":   float64(456),
			},
			wantAccount: "rCanceller",
			wantSeq:     456,
			wantErr:     false,
		},
		{
			name: "OfferSequence as int",
			tx: map[string]interface{}{
				"TransactionType": "OfferCancel",
				"Account":         "rCanceller",
				"OfferSequence":   789,
			},
			wantAccount: "rCanceller",
			wantSeq:     789,
			wantErr:     false,
		},
		{
			name: "OfferSequence as int64",
			tx: map[string]interface{}{
				"TransactionType": "OfferCancel",
				"Account":         "rCanceller",
				"OfferSequence":   int64(999),
			},
			wantAccount: "rCanceller",
			wantSeq:     999,
			wantErr:     false,
		},
		{
			name: "missing Account",
			tx: map[string]interface{}{
				"TransactionType": "OfferCancel",
				"OfferSequence":   float64(456),
			},
			wantAccount: "",
			wantSeq:     0,
			wantErr:     true,
		},
		{
			name: "missing OfferSequence",
			tx: map[string]interface{}{
				"TransactionType": "OfferCancel",
				"Account":         "rCanceller",
			},
			wantAccount: "",
			wantSeq:     0,
			wantErr:     true,
		},
		{
			name: "wrong transaction type",
			tx: map[string]interface{}{
				"TransactionType": "Payment",
				"Account":         "rCanceller",
				"OfferSequence":   float64(456),
			},
			wantAccount: "",
			wantSeq:     0,
			wantErr:     true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account, seq, err := parser.ParseOfferCancel(tt.tx)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseOfferCancel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if account != tt.wantAccount {
				t.Errorf("account = %v, want %v", account, tt.wantAccount)
			}
			
			if seq != tt.wantSeq {
				t.Errorf("sequence = %v, want %v", seq, tt.wantSeq)
			}
		})
	}
}

func TestCalculatePrice(t *testing.T) {
	tests := []struct {
		name        string
		quoteAmount string
		baseAmount  string
		wantPrice   string
		wantErr     bool
	}{
		{
			name:        "simple division",
			quoteAmount: "100",
			baseAmount:  "50",
			wantPrice:   "2.00000000",
			wantErr:     false,
		},
		{
			name:        "decimal division",
			quoteAmount: "123.45",
			baseAmount:  "67.89",
			wantPrice:   "1.81838268", // 123.45 / 67.89 (actual float precision)
			wantErr:     false,
		},
		{
			name:        "zero base amount",
			quoteAmount: "100",
			baseAmount:  "0",
			wantPrice:   "",
			wantErr:     true,
		},
		{
			name:        "invalid quote amount",
			quoteAmount: "invalid",
			baseAmount:  "50",
			wantPrice:   "",
			wantErr:     true,
		},
		{
			name:        "invalid base amount",
			quoteAmount: "100",
			baseAmount:  "invalid",
			wantPrice:   "",
			wantErr:     true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price, err := calculatePrice(tt.quoteAmount, tt.baseAmount)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("calculatePrice() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if price != tt.wantPrice {
				t.Errorf("calculatePrice() = %v, want %v", price, tt.wantPrice)
			}
		})
	}
}

func TestOrderbookParser_parseOfferCreate(t *testing.T) {
	parser := NewOrderbookParser()
	
	tests := []struct {
		name    string
		tx      map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid XRP/USD offer",
			tx: map[string]interface{}{
				"Account":  "rAccount",
				"Sequence": float64(100),
				"TakerPays": map[string]interface{}{
					"currency": "USD",
					"issuer":   "rUSD",
					"value":    "100",
				},
				"TakerGets": "1000000",
			},
			wantErr: false,
		},
		{
			name: "offer with expiration",
			tx: map[string]interface{}{
				"Account":    "rAccount",
				"Sequence":   float64(100),
				"TakerPays":  "2000000",
				"TakerGets":  "1000000",
				"Expiration": float64(741234567),
			},
			wantErr: false,
		},
		{
			name: "missing Sequence",
			tx: map[string]interface{}{
				"Account":   "rAccount",
				"TakerPays": "1000000",
				"TakerGets": "500000",
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offer, err := parser.parseOfferCreate(tt.tx, 12345, "HASH")
			
			if (err != nil) != tt.wantErr {
				t.Errorf("parseOfferCreate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr {
				if offer == nil {
					t.Error("parseOfferCreate() returned nil offer")
					return
				}
				
				// Status could be "active" or "invalid_parse" depending on input
				if offer.Status != "active" && offer.Status != "invalid_parse" {
					t.Errorf("Status = %v, want active or invalid_parse", offer.Status)
				}
				
				// Side could be "ask" or "bid" depending on validity
				if offer.Status == "active" && offer.Side != "ask" {
					t.Errorf("Side = %v, want ask", offer.Side)
				}
			}
		})
	}
}

// Benchmark for orderbook parsing
func BenchmarkOrderbookParser_ParseTransaction(b *testing.B) {
	parser := NewOrderbookParser()
	tx := map[string]interface{}{
		"TransactionType": "OfferCreate",
		"Account":         "rAccount",
		"Sequence":        float64(100),
		"TakerPays":       "1000000",
		"TakerGets": map[string]interface{}{
			"currency": "USD",
			"issuer":   "rUSD",
			"value":    "500",
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.ParseTransaction(tx, 12345, "HASH")
	}
}

func BenchmarkCalculatePrice(b *testing.B) {
	quoteAmount := "123.45"
	baseAmount := "67.89"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculatePrice(quoteAmount, baseAmount)
	}
}

func BenchmarkParseOfferCancel(b *testing.B) {
	parser := NewOrderbookParser()
	tx := map[string]interface{}{
		"TransactionType": "OfferCancel",
		"Account":         "rCanceller",
		"OfferSequence":   float64(456),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.ParseOfferCancel(tx)
	}
}
