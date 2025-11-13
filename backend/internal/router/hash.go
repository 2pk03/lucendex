package router

import (
	"encoding/json"
	"sort"

	"golang.org/x/crypto/blake2b"
)

type hashInput struct {
	In          string  `json:"in"`
	Out         string  `json:"out"`
	Amount      string  `json:"amount"`
	RouterBps   int     `json:"router_bps"`
	TradingFees string  `json:"trading_fees"`
	EstOutFee   string  `json:"est_out_fee"`
	LedgerIndex uint32  `json:"ledger_index"`
	TTL         uint16  `json:"ttl"`
}

func ComputeQuoteHash(req *QuoteRequest, fees Fees, ledgerIndex uint32, ttl uint16) ([32]byte, error) {
	input := hashInput{
		In:          req.In.String(),
		Out:         req.Out.String(),
		Amount:      req.Amount.String(),
		RouterBps:   fees.RouterBps,
		TradingFees: fees.TradingFees.String(),
		EstOutFee:   fees.EstOutFee.String(),
		LedgerIndex: ledgerIndex,
		TTL:         ttl,
	}

	canonical, err := canonicalJSON(input)
	if err != nil {
		return [32]byte{}, err
	}

	return blake2b.Sum256(canonical), nil
}

func canonicalJSON(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	sorted := make(map[string]interface{})
	for _, k := range keys {
		sorted[k] = obj[k]
	}

	return json.Marshal(sorted)
}
