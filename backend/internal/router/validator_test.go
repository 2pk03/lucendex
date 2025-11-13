package router

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestValidator_ValidateQuoteRequest(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name    string
		req     QuoteRequest
		wantErr error
	}{
		{
			name: "valid XRP to USD",
			req: QuoteRequest{
				In:     Asset{Currency: "XRP"},
				Out:    Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
				Amount: decimal.NewFromInt(100),
			},
			wantErr: nil,
		},
		{
			name: "zero amount",
			req: QuoteRequest{
				In:     Asset{Currency: "XRP"},
				Out:    Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
				Amount: decimal.Zero,
			},
			wantErr: ErrInvalidAmount,
		},
		{
			name: "negative amount",
			req: QuoteRequest{
				In:     Asset{Currency: "XRP"},
				Out:    Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
				Amount: decimal.NewFromInt(-100),
			},
			wantErr: ErrInvalidAmount,
		},
		{
			name: "amount too large",
			req: QuoteRequest{
				In:     Asset{Currency: "XRP"},
				Out:    Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
				Amount: decimal.NewFromInt(1e18).Add(decimal.NewFromInt(1)),
			},
			wantErr: ErrAmountTooLarge,
		},
		{
			name: "same assets",
			req: QuoteRequest{
				In:     Asset{Currency: "XRP"},
				Out:    Asset{Currency: "XRP"},
				Amount: decimal.NewFromInt(100),
			},
			wantErr: ErrSameAssets,
		},
		{
			name: "invalid in asset",
			req: QuoteRequest{
				In:     Asset{Currency: ""},
				Out:    Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
				Amount: decimal.NewFromInt(100),
			},
			wantErr: ErrInvalidAsset,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateQuoteRequest(&tt.req)
			if err != tt.wantErr {
				t.Errorf("ValidateQuoteRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_ValidateAsset(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name    string
		asset   Asset
		wantErr error
	}{
		{
			name:    "valid XRP",
			asset:   Asset{Currency: "XRP"},
			wantErr: nil,
		},
		{
			name:    "valid USD with issuer",
			asset:   Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
			wantErr: nil,
		},
		{
			name:    "empty currency",
			asset:   Asset{Currency: ""},
			wantErr: ErrInvalidAsset,
		},
		{
			name:    "XRP with issuer",
			asset:   Asset{Currency: "XRP", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
			wantErr: ErrInvalidAsset,
		},
		{
			name:    "non-XRP without issuer",
			asset:   Asset{Currency: "USD"},
			wantErr: ErrInvalidAsset,
		},
		{
			name:    "invalid currency format",
			asset:   Asset{Currency: "us"},
			wantErr: ErrInvalidAsset,
		},
		{
			name:    "invalid issuer address",
			asset:   Asset{Currency: "USD", Issuer: "invalid"},
			wantErr: ErrInvalidAddress,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateAsset(tt.asset)
			if err != tt.wantErr {
				t.Errorf("ValidateAsset() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_IsValidXRPLAddress(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name    string
		address string
		want    bool
	}{
		{
			name:    "valid address",
			address: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B",
			want:    true,
		},
		{
			name:    "valid address 2",
			address: "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
			want:    true,
		},
		{
			name:    "empty address",
			address: "",
			want:    false,
		},
		{
			name:    "too short",
			address: "r123",
			want:    false,
		},
		{
			name:    "wrong prefix",
			address: "xvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B",
			want:    false,
		},
		{
			name:    "invalid characters",
			address: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B!",
			want:    false,
		},
		{
			name:    "with whitespace",
			address: " rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B ",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.IsValidXRPLAddress(tt.address)
			if got != tt.want {
				t.Errorf("IsValidXRPLAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}
