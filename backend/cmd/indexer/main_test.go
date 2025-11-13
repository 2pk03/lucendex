package main

import (
	"reflect"
	"testing"
)

func TestHasLucendexQuoteHash(t *testing.T) {
	tests := []struct {
		name      string
		tx        map[string]interface{}
		wantHash  []byte
		wantFound bool
	}{
		{
			name: "valid lucendex memo",
			tx: map[string]interface{}{
				"Memos": []interface{}{
					map[string]interface{}{
						"Memo": map[string]interface{}{
							"MemoType": "6c7563656e6465782f71756f7465", // "lucendex/quote" in hex
							"MemoData": "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
						},
					},
				},
			},
			wantHash:  []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			wantFound: true,
		},
		{
			name: "no memos",
			tx: map[string]interface{}{
				"Account": "rAccount1",
			},
			wantHash:  nil,
			wantFound: false,
		},
		{
			name: "wrong memo type",
			tx: map[string]interface{}{
				"Memos": []interface{}{
					map[string]interface{}{
						"Memo": map[string]interface{}{
							"MemoType": "736f6d657468696e67656c7365", // "somethingelse"
							"MemoData": "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
						},
					},
				},
			},
			wantHash:  nil,
			wantFound: false,
		},
		{
			name: "invalid hash length",
			tx: map[string]interface{}{
				"Memos": []interface{}{
					map[string]interface{}{
						"Memo": map[string]interface{}{
							"MemoType": "6c7563656e6465782f71756f7465",
							"MemoData": "0102", // Too short
						},
					},
				},
			},
			wantHash:  nil,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHash, gotFound := hasLucendexQuoteHash(tt.tx)
			if gotFound != tt.wantFound {
				t.Errorf("hasLucendexQuoteHash() gotFound = %v, want %v", gotFound, tt.wantFound)
			}
			if !reflect.DeepEqual(gotHash, tt.wantHash) {
				t.Errorf("hasLucendexQuoteHash() gotHash = %v, want %v", gotHash, tt.wantHash)
			}
		})
	}
}

func TestExtractTradeDetails(t *testing.T) {
	tests := []struct {
		name          string
		tx            map[string]interface{}
		wantInAsset   string
		wantOutAsset  string
		wantAmountIn  string
		wantAmountOut string
		wantOk        bool
	}{
		{
			name: "XRP to IOU",
			tx: map[string]interface{}{
				"Account": "rAccount1",
				"Amount":  "1000000", // 1 XRP in drops
				"meta": map[string]interface{}{
					"delivered_amount": map[string]interface{}{
						"currency": "USD",
						"issuer":   "rIssuer1",
						"value":    "500",
					},
				},
			},
			wantInAsset:   "XRP",
			wantOutAsset:  "USD.rIssuer1",
			wantAmountIn:  "1000000",
			wantAmountOut: "500",
			wantOk:        true,
		},
		{
			name: "IOU to XRP",
			tx: map[string]interface{}{
				"Account": "rAccount2",
				"Amount": map[string]interface{}{
					"currency": "EUR",
					"issuer":   "rIssuer2",
					"value":    "100",
				},
				"meta": map[string]interface{}{
					"delivered_amount": "2000000", // 2 XRP
				},
			},
			wantInAsset:   "EUR.rIssuer2",
			wantOutAsset:  "XRP",
			wantAmountIn:  "100",
			wantAmountOut: "2000000",
			wantOk:        true,
		},
		{
			name: "IOU to IOU",
			tx: map[string]interface{}{
				"Account": "rAccount3",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   "rIssuer3",
					"value":    "200",
				},
				"meta": map[string]interface{}{
					"delivered_amount": map[string]interface{}{
						"currency": "EUR",
						"issuer":   "rIssuer4",
						"value":    "180",
					},
				},
			},
			wantInAsset:   "USD.rIssuer3",
			wantOutAsset:  "EUR.rIssuer4",
			wantAmountIn:  "200",
			wantAmountOut: "180",
			wantOk:        true,
		},
		{
			name: "missing meta",
			tx: map[string]interface{}{
				"Account": "rAccount4",
				"Amount":  "1000000",
			},
			wantOk: false,
		},
		{
			name: "missing account",
			tx: map[string]interface{}{
				"Amount": "1000000",
				"meta": map[string]interface{}{
					"delivered_amount": "2000000",
				},
			},
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotInAsset, gotOutAsset, gotAmountIn, gotAmountOut, gotOk := extractTradeDetails(tt.tx)
			if gotOk != tt.wantOk {
				t.Errorf("extractTradeDetails() gotOk = %v, want %v", gotOk, tt.wantOk)
			}
			if gotOk {
				if gotInAsset != tt.wantInAsset {
					t.Errorf("extractTradeDetails() gotInAsset = %v, want %v", gotInAsset, tt.wantInAsset)
				}
				if gotOutAsset != tt.wantOutAsset {
					t.Errorf("extractTradeDetails() gotOutAsset = %v, want %v", gotOutAsset, tt.wantOutAsset)
				}
				if gotAmountIn != tt.wantAmountIn {
					t.Errorf("extractTradeDetails() gotAmountIn = %v, want %v", gotAmountIn, tt.wantAmountIn)
				}
				if gotAmountOut != tt.wantAmountOut {
					t.Errorf("extractTradeDetails() gotAmountOut = %v, want %v", gotAmountOut, tt.wantAmountOut)
				}
			}
		})
	}
}
