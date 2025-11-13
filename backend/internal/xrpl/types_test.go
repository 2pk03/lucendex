package xrpl

import (
	"encoding/json"
	"testing"
)

func TestAmount_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCurr  string
		wantValue string
		wantDrops string
		wantErr   bool
	}{
		{
			name:      "XRP drops as string",
			input:     `"1000000"`,
			wantCurr:  "XRP",
			wantValue: "",
			wantDrops: "1000000",
			wantErr:   false,
		},
		{
			name:      "IOU object",
			input:     `{"currency":"USD","issuer":"rN7n7otQDd6FczFgLdlqtyMVrn3HMfWi","value":"100.50"}`,
			wantCurr:  "USD",
			wantValue: "100.50",
			wantDrops: "",
			wantErr:   false,
		},
		{
			name:      "XRP zero",
			input:     `"0"`,
			wantCurr:  "XRP",
			wantValue: "",
			wantDrops: "0",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var amt Amount
			err := json.Unmarshal([]byte(tt.input), &amt)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if amt.Currency != tt.wantCurr {
				t.Errorf("Currency = %v, want %v", amt.Currency, tt.wantCurr)
			}
			
			if amt.Value != tt.wantValue {
				t.Errorf("Value = %v, want %v", amt.Value, tt.wantValue)
			}
			
			if amt.Drops != tt.wantDrops {
				t.Errorf("Drops = %v, want %v", amt.Drops, tt.wantDrops)
			}
		})
	}
}

func TestAmount_ParseAmount(t *testing.T) {
	tests := []struct {
		name   string
		amount Amount
		want   string
	}{
		{
			name: "XRP with drops",
			amount: Amount{
				Currency: "XRP",
				Drops:    "1000000",
			},
			want: "1000000",
		},
		{
			name: "IOU with value",
			amount: Amount{
				Currency: "USD",
				Issuer:   "rN7n7otQDd6FczFgLdlqtyMVrn3HMfWi",
				Value:    "100.50",
			},
			want: "100.50",
		},
		{
			name: "Empty drops defaults to value",
			amount: Amount{
				Currency: "EUR",
				Value:    "250.75",
				Drops:    "",
			},
			want: "250.75",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.amount.ParseAmount()
			if got != tt.want {
				t.Errorf("ParseAmount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatAsset(t *testing.T) {
	tests := []struct {
		name     string
		currency string
		issuer   string
		want     string
	}{
		{
			name:     "XRP",
			currency: "XRP",
			issuer:   "",
			want:     "XRP",
		},
		{
			name:     "XRP with issuer (should ignore)",
			currency: "XRP",
			issuer:   "rN7n7otQDd6FczFgLdlqtyMVrn3HMfWi",
			want:     "XRP",
		},
		{
			name:     "USD IOU",
			currency: "USD",
			issuer:   "rN7n7otQDd6FczFgLdlqtyMVrn3HMfWi",
			want:     "USD.rN7n7otQDd6FczFgLdlqtyMVrn3HMfWi",
		},
		{
			name:     "Custom token",
			currency: "SOLO",
			issuer:   "rsoLo2S1kiGeCcn6hCUXVrCpGMWLrRrLZz",
			want:     "SOLO.rsoLo2S1kiGeCcn6hCUXVrCpGMWLrRrLZz",
		},
		{
			name:     "Empty issuer",
			currency: "USD",
			issuer:   "",
			want:     "XRP", // Defaults to XRP
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatAsset(tt.currency, tt.issuer)
			if got != tt.want {
				t.Errorf("FormatAsset() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLedgerResponse_Unmarshal(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(*testing.T, *LedgerResponse)
	}{
		{
			name: "valid ledger closed message",
			input: `{
				"type": "ledgerClosed",
				"ledger_index": 87654321,
				"ledger_hash": "ABC123",
				"ledger_time": 741234567,
				"validated": true,
				"txn_count": 42
			}`,
			wantErr: false,
			check: func(t *testing.T, lr *LedgerResponse) {
				if lr.Type != "ledgerClosed" {
					t.Errorf("Type = %v, want ledgerClosed", lr.Type)
				}
				if lr.LedgerIndex != 87654321 {
					t.Errorf("LedgerIndex = %v, want 87654321", lr.LedgerIndex)
				}
				if lr.LedgerHash != "ABC123" {
					t.Errorf("LedgerHash = %v, want ABC123", lr.LedgerHash)
				}
				if !lr.Validated {
					t.Error("Validated = false, want true")
				}
				if lr.TxnCount != 42 {
					t.Errorf("TxnCount = %v, want 42", lr.TxnCount)
				}
			},
		},
		{
			name:    "invalid JSON",
			input:   `{invalid json}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var lr LedgerResponse
			err := json.Unmarshal([]byte(tt.input), &lr)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr && tt.check != nil {
				tt.check(t, &lr)
			}
		})
	}
}

func TestTransaction_Unmarshal(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(*testing.T, *Transaction)
	}{
		{
			name: "payment transaction",
			input: `{
				"TransactionType": "Payment",
				"Account": "rN7n7otQDd6FczFgLdlqtyMVrn3HMfWi",
				"Sequence": 12345,
				"Fee": "12",
				"hash": "ABCDEF123456"
			}`,
			wantErr: false,
			check: func(t *testing.T, tx *Transaction) {
				if tx.TransactionType != "Payment" {
					t.Errorf("TransactionType = %v, want Payment", tx.TransactionType)
				}
				if tx.Account != "rN7n7otQDd6FczFgLdlqtyMVrn3HMfWi" {
					t.Errorf("Account = %v, want rN7n7otQDd6FczFgLdlqtyMVrn3HMfWi", tx.Account)
				}
				if tx.Sequence != 12345 {
					t.Errorf("Sequence = %v, want 12345", tx.Sequence)
				}
			},
		},
		{
			name:    "malformed transaction",
			input:   `{"TransactionType": }`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tx Transaction
			err := json.Unmarshal([]byte(tt.input), &tx)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr && tt.check != nil {
				tt.check(t, &tx)
			}
		})
	}
}

// Edge cases and boundary conditions
func TestAmount_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "empty string",
			input:   `""`,
			wantErr: false, // Should parse as XRP with empty drops
		},
	{
		name:    "null",
		input:   `null`,
		wantErr: false, // JSON null unmarshals to zero value
	},
		{
			name:    "array",
			input:   `[]`,
			wantErr: true,
		},
		{
			name:    "number",
			input:   `123`,
			wantErr: true, // Should be string
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var amt Amount
			err := json.Unmarshal([]byte(tt.input), &amt)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
