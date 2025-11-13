package router

import (
	"github.com/shopspring/decimal"
)

type Asset struct {
	Currency string
	Issuer   string
}

func (a Asset) String() string {
	if a.Issuer == "" {
		return a.Currency
	}
	return a.Currency + "." + a.Issuer
}

func (a Asset) IsXRP() bool {
	return a.Currency == "XRP" && a.Issuer == ""
}

type QuoteRequest struct {
	In     Asset
	Out    Asset
	Amount decimal.Decimal
}

type QuoteResponse struct {
	Route       Route
	Out         decimal.Decimal
	Price       decimal.Decimal
	Fees        Fees
	LedgerIndex uint32
	QuoteHash   [32]byte
	TTLLedgers  uint16
}

type Route struct {
	Hops        []Hop
	TotalFees   Fees
	PriceImpact decimal.Decimal
}

type Hop struct {
	Type      string
	In        Asset
	Out       Asset
	AmountIn  decimal.Decimal
	AmountOut decimal.Decimal
}

type Fees struct {
	RouterBps   int
	TradingFees decimal.Decimal
	EstOutFee   decimal.Decimal
}

type AMMPool struct {
	Asset1        Asset
	Asset2        Asset
	Asset1Reserve decimal.Decimal
	Asset2Reserve decimal.Decimal
	TradingFeeBps int
	LPToken       string
	Account       string
}

type Offer struct {
	TakerPays Asset
	TakerGets Asset
	Quality   decimal.Decimal
	Account   string
	Sequence  uint32
}

type TradingPairInfo struct {
	In          Asset
	Out         Asset
	Liquidity   decimal.Decimal
	AvgSpread   decimal.Decimal
	DailyVolume decimal.Decimal
}
