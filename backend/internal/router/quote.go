package router

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

type QuoteEngine struct {
	validator  *Validator
	pathfinder *Pathfinder
	breaker    *CircuitBreaker
	kv         KVStore
	routerBps  int
}

type KVStore interface {
	GetQuote(hash [32]byte) ([]byte, bool)
	SetQuote(hash [32]byte, route []byte, ttl time.Duration) error
	SetLedgerIndex(idx uint32) error
	GetLedgerIndex() (uint32, bool)
}

func NewQuoteEngine(validator *Validator, pathfinder *Pathfinder, breaker *CircuitBreaker, kv KVStore, routerBps int) *QuoteEngine {
	return &QuoteEngine{
		validator:  validator,
		pathfinder: pathfinder,
		breaker:    breaker,
		kv:         kv,
		routerBps:  routerBps,
	}
}

func (qe *QuoteEngine) GenerateQuote(ctx context.Context, req *QuoteRequest, ledgerIndex uint32) (*QuoteResponse, error) {
	if err := qe.validator.ValidateQuoteRequest(req); err != nil {
		return nil, err
	}

	route, err := qe.pathfinder.FindBestRoute(req.In, req.Out, req.Amount)
	if err != nil {
		return nil, err
	}

	totalFees := qe.calculateTotalFees(route)
	totalFees.RouterBps = qe.routerBps

	finalAmount := route.Hops[len(route.Hops)-1].AmountOut

	price := finalAmount.Div(req.Amount)

	priceImpact := qe.calculatePriceImpact(route, req.Amount)
	route.PriceImpact = priceImpact

	pair := req.In.String() + "-" + req.Out.String()
	if err := qe.breaker.CheckPrice(pair, price); err != nil {
		return nil, err
	}

	ttl := uint16(100)
	quoteHash, err := ComputeQuoteHash(req, totalFees, ledgerIndex, ttl)
	if err != nil {
		return nil, err
	}

	resp := &QuoteResponse{
		Route:       *route,
		Out:         finalAmount,
		Price:       price,
		Fees:        totalFees,
		LedgerIndex: ledgerIndex,
		QuoteHash:   quoteHash,
		TTLLedgers:  ttl,
	}

	return resp, nil
}

func (qe *QuoteEngine) calculateTotalFees(route *Route) Fees {
	totalTradingFees := decimal.Zero

	for _, hop := range route.Hops {
		if hop.Type == "amm" {
			fee := hop.AmountIn.Sub(hop.AmountOut).Div(hop.AmountIn)
			totalTradingFees = totalTradingFees.Add(fee)
		}
	}

	return Fees{
		TradingFees: totalTradingFees,
		EstOutFee:   decimal.Zero,
	}
}

func (qe *QuoteEngine) calculatePriceImpact(route *Route, amountIn decimal.Decimal) decimal.Decimal {
	if len(route.Hops) == 0 {
		return decimal.Zero
	}

	finalAmount := route.Hops[len(route.Hops)-1].AmountOut
	if amountIn.IsZero() {
		return decimal.Zero
	}

	executionPrice := finalAmount.Div(amountIn)

	impact := decimal.NewFromInt(1).Sub(executionPrice).Abs()

	return impact
}
