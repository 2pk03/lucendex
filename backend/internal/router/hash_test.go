package router

import (
	"bytes"
	"testing"

	"github.com/shopspring/decimal"
)

func TestComputeQuoteHash_Determinism(t *testing.T) {
	req := &QuoteRequest{
		In:     Asset{Currency: "XRP"},
		Out:    Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
		Amount: decimal.NewFromFloat(100.5),
	}

	fees := Fees{
		RouterBps:   20,
		TradingFees: decimal.NewFromFloat(0.3),
		EstOutFee:   decimal.NewFromFloat(0.1),
	}

	ledgerIndex := uint32(12345)
	ttl := uint16(100)

	var hashes [][32]byte
	for i := 0; i < 100; i++ {
		hash, err := ComputeQuoteHash(req, fees, ledgerIndex, ttl)
		if err != nil {
			t.Fatalf("ComputeQuoteHash() iteration %d error = %v", i, err)
		}
		hashes = append(hashes, hash)
	}

	firstHash := hashes[0]
	for i, hash := range hashes {
		if hash != firstHash {
			t.Errorf("Hash at iteration %d differs from first hash", i)
		}
	}
}

func TestComputeQuoteHash_Uniqueness(t *testing.T) {
	baseReq := &QuoteRequest{
		In:     Asset{Currency: "XRP"},
		Out:    Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
		Amount: decimal.NewFromInt(100),
	}

	baseFees := Fees{
		RouterBps:   20,
		TradingFees: decimal.NewFromFloat(0.3),
		EstOutFee:   decimal.NewFromFloat(0.1),
	}

	hash1, _ := ComputeQuoteHash(baseReq, baseFees, 12345, 100)

	reqDiffAmount := *baseReq
	reqDiffAmount.Amount = decimal.NewFromInt(101)
	hash2, _ := ComputeQuoteHash(&reqDiffAmount, baseFees, 12345, 100)

	if hash1 == hash2 {
		t.Error("Different amounts produced same hash")
	}

	feesDiff := baseFees
	feesDiff.RouterBps = 21
	hash3, _ := ComputeQuoteHash(baseReq, feesDiff, 12345, 100)

	if hash1 == hash3 {
		t.Error("Different router fees produced same hash")
	}

	hash4, _ := ComputeQuoteHash(baseReq, baseFees, 12346, 100)
	if hash1 == hash4 {
		t.Error("Different ledger index produced same hash")
	}

	hash5, _ := ComputeQuoteHash(baseReq, baseFees, 12345, 101)
	if hash1 == hash5 {
		t.Error("Different TTL produced same hash")
	}
}

func TestComputeQuoteHash_CanonicalFormat(t *testing.T) {
	req := &QuoteRequest{
		In:     Asset{Currency: "XRP"},
		Out:    Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
		Amount: decimal.NewFromInt(100),
	}

	fees := Fees{
		RouterBps:   20,
		TradingFees: decimal.NewFromFloat(0.3),
		EstOutFee:   decimal.NewFromFloat(0.1),
	}

	hash1, err := ComputeQuoteHash(req, fees, 12345, 100)
	if err != nil {
		t.Fatalf("ComputeQuoteHash() error = %v", err)
	}

	hash2, err := ComputeQuoteHash(req, fees, 12345, 100)
	if err != nil {
		t.Fatalf("ComputeQuoteHash() error = %v", err)
	}

	if !bytes.Equal(hash1[:], hash2[:]) {
		t.Error("Canonical format not consistent")
	}
}

func TestComputeQuoteHash_FeeInclusion(t *testing.T) {
	req := &QuoteRequest{
		In:     Asset{Currency: "XRP"},
		Out:    Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
		Amount: decimal.NewFromInt(100),
	}

	fees1 := Fees{
		RouterBps:   20,
		TradingFees: decimal.NewFromFloat(0.3),
		EstOutFee:   decimal.NewFromFloat(0.1),
	}

	fees2 := Fees{
		RouterBps:   20,
		TradingFees: decimal.NewFromFloat(0.3),
		EstOutFee:   decimal.NewFromFloat(0.2),
	}

	hash1, _ := ComputeQuoteHash(req, fees1, 12345, 100)
	hash2, _ := ComputeQuoteHash(req, fees2, 12345, 100)

	if hash1 == hash2 {
		t.Error("Different EstOutFee values produced same hash (fee tampering possible)")
	}
}

func TestComputeQuoteHash_AssetOrdering(t *testing.T) {
	req1 := &QuoteRequest{
		In:     Asset{Currency: "XRP"},
		Out:    Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
		Amount: decimal.NewFromInt(100),
	}

	req2 := &QuoteRequest{
		In:     Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
		Out:    Asset{Currency: "XRP"},
		Amount: decimal.NewFromInt(100),
	}

	fees := Fees{RouterBps: 20}

	hash1, _ := ComputeQuoteHash(req1, fees, 12345, 100)
	hash2, _ := ComputeQuoteHash(req2, fees, 12345, 100)

	if hash1 == hash2 {
		t.Error("Swapped assets produced same hash")
	}
}
