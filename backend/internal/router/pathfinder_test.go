package router

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestPathfinder_DirectAMMRoute(t *testing.T) {
	pools := []AMMPool{
		{
			Asset1:        Asset{Currency: "XRP"},
			Asset2:        Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
			Asset1Reserve: decimal.NewFromInt(10000),
			Asset2Reserve: decimal.NewFromInt(15000),
			TradingFeeBps: 30,
		},
	}

	pf := NewPathfinder(pools, nil)

	route, err := pf.FindBestRoute(
		Asset{Currency: "XRP"},
		Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
		decimal.NewFromInt(100),
	)

	if err != nil {
		t.Fatalf("FindBestRoute() error = %v", err)
	}
	if route == nil {
		t.Fatal("Expected route, got nil")
	}
	if len(route.Hops) != 1 {
		t.Errorf("Hops = %d, want 1", len(route.Hops))
	}
	if route.Hops[0].Type != "amm" {
		t.Errorf("Hop type = %s, want amm", route.Hops[0].Type)
	}
}

func TestPathfinder_DirectOrderbookRoute(t *testing.T) {
	offers := []Offer{
		{
			TakerPays: Asset{Currency: "XRP"},
			TakerGets: Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
			Quality:   decimal.NewFromFloat(1.5),
		},
	}

	pf := NewPathfinder(nil, offers)

	route, err := pf.FindBestRoute(
		Asset{Currency: "XRP"},
		Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
		decimal.NewFromInt(100),
	)

	if err != nil {
		t.Fatalf("FindBestRoute() error = %v", err)
	}
	if len(route.Hops) != 1 {
		t.Errorf("Hops = %d, want 1", len(route.Hops))
	}
	if route.Hops[0].Type != "orderbook" {
		t.Errorf("Hop type = %s, want orderbook", route.Hops[0].Type)
	}
}

func TestPathfinder_MultiHopRoute(t *testing.T) {
	pools := []AMMPool{
		{
			Asset1:        Asset{Currency: "XRP"},
			Asset2:        Asset{Currency: "BTC", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
			Asset1Reserve: decimal.NewFromInt(10000),
			Asset2Reserve: decimal.NewFromInt(100),
			TradingFeeBps: 30,
		},
		{
			Asset1:        Asset{Currency: "BTC", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
			Asset2:        Asset{Currency: "USD", Issuer: "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq"},
			Asset1Reserve: decimal.NewFromInt(100),
			Asset2Reserve: decimal.NewFromInt(3000000),
			TradingFeeBps: 30,
		},
	}

	pf := NewPathfinder(pools, nil)

	route, err := pf.FindBestRoute(
		Asset{Currency: "XRP"},
		Asset{Currency: "USD", Issuer: "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq"},
		decimal.NewFromInt(100),
	)

	if err != nil {
		t.Fatalf("FindBestRoute() error = %v", err)
	}
	if len(route.Hops) != 2 {
		t.Errorf("Hops = %d, want 2", len(route.Hops))
	}
}

func TestPathfinder_NoRoute(t *testing.T) {
	pools := []AMMPool{
		{
			Asset1:        Asset{Currency: "XRP"},
			Asset2:        Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
			Asset1Reserve: decimal.NewFromInt(10000),
			Asset2Reserve: decimal.NewFromInt(15000),
			TradingFeeBps: 30,
		},
	}

	pf := NewPathfinder(pools, nil)

	_, err := pf.FindBestRoute(
		Asset{Currency: "XRP"},
		Asset{Currency: "EUR", Issuer: "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq"},
		decimal.NewFromInt(100),
	)

	if err != ErrNoRoute {
		t.Errorf("FindBestRoute() error = %v, want %v", err, ErrNoRoute)
	}
}

func TestPathfinder_AMMCalculation(t *testing.T) {
	pool := AMMPool{
		Asset1:        Asset{Currency: "XRP"},
		Asset2:        Asset{Currency: "USD", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
		Asset1Reserve: decimal.NewFromInt(10000),
		Asset2Reserve: decimal.NewFromInt(15000),
		TradingFeeBps: 30,
	}

	pf := &Pathfinder{pools: []AMMPool{pool}}

	amountIn := decimal.NewFromInt(100)
	amountOut := pf.calculateAMMOutput(&pool, amountIn, true)

	if amountOut.LessThanOrEqual(decimal.Zero) {
		t.Error("AMM output should be positive")
	}

	expectedMinOut := decimal.NewFromInt(140)
	if amountOut.LessThan(expectedMinOut) {
		t.Errorf("AMM output %s less than expected min %s", amountOut, expectedMinOut)
	}
}

func TestPathfinder_MaxHopsLimit(t *testing.T) {
	pools := []AMMPool{
		{
			Asset1:        Asset{Currency: "AAA"},
			Asset2:        Asset{Currency: "BBB", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
			Asset1Reserve: decimal.NewFromInt(1000),
			Asset2Reserve: decimal.NewFromInt(1000),
			TradingFeeBps: 30,
		},
		{
			Asset1:        Asset{Currency: "BBB", Issuer: "rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B"},
			Asset2:        Asset{Currency: "CCC", Issuer: "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq"},
			Asset1Reserve: decimal.NewFromInt(1000),
			Asset2Reserve: decimal.NewFromInt(1000),
			TradingFeeBps: 30,
		},
		{
			Asset1:        Asset{Currency: "CCC", Issuer: "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq"},
			Asset2:        Asset{Currency: "DDD", Issuer: "rN7n7otQDd6FczFgLdSqtcsAUxDkw6fzRH"},
			Asset1Reserve: decimal.NewFromInt(1000),
			Asset2Reserve: decimal.NewFromInt(1000),
			TradingFeeBps: 30,
		},
		{
			Asset1:        Asset{Currency: "DDD", Issuer: "rN7n7otQDd6FczFgLdSqtcsAUxDkw6fzRH"},
			Asset2:        Asset{Currency: "EEE", Issuer: "rLHzPsX6oXkzU9rFxyZMSbF4ApdQnXPZy4"},
			Asset1Reserve: decimal.NewFromInt(1000),
			Asset2Reserve: decimal.NewFromInt(1000),
			TradingFeeBps: 30,
		},
	}

	pf := NewPathfinder(pools, nil)

	_, err := pf.FindBestRoute(
		Asset{Currency: "AAA"},
		Asset{Currency: "EEE", Issuer: "rLHzPsX6oXkzU9rFxyZMSbF4ApdQnXPZy4"},
		decimal.NewFromInt(10),
	)

	if err != ErrNoRoute {
		t.Errorf("Long path error = %v, want %v (max %d hops)", err, ErrNoRoute, MaxHops)
	}
}
