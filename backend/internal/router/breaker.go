package router

import (
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

const (
	StateClosed   = "closed"
	StateOpen     = "open"
	StateHalfOpen = "half-open"

	DefaultThreshold      = 0.05
	DefaultMaxPrices      = 100
	DefaultFailureLimit   = 5
	DefaultCautionDuration = 60 * time.Second
)

type CircuitBreaker struct {
	mu              sync.RWMutex
	states          map[string]*breakerState
	threshold       float64
	cautionMode     bool
	cautionUntil    time.Time
	persistCallback func(pair string, state *breakerState)
}

type breakerState struct {
	pair         string
	recentPrices []decimal.Decimal
	lastTradeTs  time.Time
	state        string
	failures     int
	openedAt     time.Time
}

func NewCircuitBreaker(threshold float64) *CircuitBreaker {
	cb := &CircuitBreaker{
		states:    make(map[string]*breakerState),
		threshold: threshold,
	}
	cb.EnableCautionMode(DefaultCautionDuration)
	return cb
}

func (cb *CircuitBreaker) EnableCautionMode(duration time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.cautionMode = true
	cb.cautionUntil = time.Now().Add(duration)

	time.AfterFunc(duration, func() {
		cb.mu.Lock()
		defer cb.mu.Unlock()
		cb.cautionMode = false
	})
}

func (cb *CircuitBreaker) SetPersistCallback(fn func(pair string, state *breakerState)) {
	cb.persistCallback = fn
}

func (cb *CircuitBreaker) CheckPrice(pair string, price decimal.Decimal) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.getOrCreateState(pair)

	if state.state == StateOpen {
		if time.Since(state.openedAt) > 30*time.Second {
			state.state = StateHalfOpen
			state.failures = 0
		} else {
			return ErrCircuitBreakerOpen
		}
	}

	threshold := cb.threshold
	if cb.cautionMode && time.Now().Before(cb.cautionUntil) {
		threshold = threshold * 0.5
	}

	if len(state.recentPrices) == 0 {
		cb.recordPrice(state, price)
		return nil
	}

	avgPrice := cb.calculateAverage(state.recentPrices)
	deviation := price.Sub(avgPrice).Div(avgPrice).Abs()

	if deviation.GreaterThan(decimal.NewFromFloat(threshold)) {
		state.failures++
		if state.failures >= DefaultFailureLimit {
			state.state = StateOpen
			state.openedAt = time.Now()
		}
		return ErrCircuitBreakerOpen
	}

	if state.state == StateHalfOpen {
		state.state = StateClosed
		state.failures = 0
	}

	cb.recordPrice(state, price)
	return nil
}

func (cb *CircuitBreaker) RecordTrade(pair string, price decimal.Decimal) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.getOrCreateState(pair)
	cb.recordPrice(state, price)
	state.lastTradeTs = time.Now()
}

func (cb *CircuitBreaker) GetState(pair string) string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	state, ok := cb.states[pair]
	if !ok {
		return StateClosed
	}
	return state.state
}

func (cb *CircuitBreaker) getOrCreateState(pair string) *breakerState {
	state, ok := cb.states[pair]
	if !ok {
		state = &breakerState{
			pair:         pair,
			recentPrices: make([]decimal.Decimal, 0, DefaultMaxPrices),
			state:        StateClosed,
		}
		cb.states[pair] = state
	}
	return state
}

func (cb *CircuitBreaker) recordPrice(state *breakerState, price decimal.Decimal) {
	state.recentPrices = append(state.recentPrices, price)
	if len(state.recentPrices) > DefaultMaxPrices {
		state.recentPrices = state.recentPrices[1:]
	}

	if cb.persistCallback != nil {
		cb.persistCallback(state.pair, state)
	}
}

func (cb *CircuitBreaker) calculateAverage(prices []decimal.Decimal) decimal.Decimal {
	if len(prices) == 0 {
		return decimal.Zero
	}

	sum := decimal.Zero
	for _, p := range prices {
		sum = sum.Add(p)
	}

	return sum.Div(decimal.NewFromInt(int64(len(prices))))
}
