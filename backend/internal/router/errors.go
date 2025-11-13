package router

import "errors"

var (
	ErrInvalidAmount     = errors.New("invalid amount")
	ErrAmountTooLarge    = errors.New("amount too large")
	ErrInvalidAsset      = errors.New("invalid asset format")
	ErrSameAssets        = errors.New("input and output assets cannot be the same")
	ErrInvalidAddress    = errors.New("invalid XRPL address")
	ErrNoRoute           = errors.New("no route found")
	ErrInsufficientLiquidity = errors.New("insufficient liquidity")
	ErrCircuitBreakerOpen = errors.New("circuit breaker open")
)
