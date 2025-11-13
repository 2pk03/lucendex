package router

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestCircuitBreaker_NormalOperation(t *testing.T) {
	cb := NewCircuitBreaker(DefaultThreshold)
	cb.mu.Lock()
	cb.cautionMode = false
	cb.mu.Unlock()

	pair := "XRP-USD"

	basePrice := decimal.NewFromFloat(1.5)

	for i := 0; i < 10; i++ {
		price := basePrice.Add(decimal.NewFromFloat(float64(i) * 0.001))
		err := cb.CheckPrice(pair, price)
		if err != nil {
			t.Errorf("CheckPrice(%d) error = %v", i, err)
		}
	}

	if cb.GetState(pair) != StateClosed {
		t.Errorf("State = %s, want %s", cb.GetState(pair), StateClosed)
	}
}

func TestCircuitBreaker_PriceDeviation(t *testing.T) {
	cb := NewCircuitBreaker(0.05)
	cb.mu.Lock()
	cb.cautionMode = false
	cb.mu.Unlock()

	pair := "XRP-USD"

	basePrice := decimal.NewFromFloat(1.0)
	for i := 0; i < 10; i++ {
		cb.RecordTrade(pair, basePrice)
	}

	normalPrice := decimal.NewFromFloat(1.04)
	if err := cb.CheckPrice(pair, normalPrice); err != nil {
		t.Errorf("Normal price rejected: %v", err)
	}

	extremePrice := decimal.NewFromFloat(1.20)
	err := cb.CheckPrice(pair, extremePrice)
	if err != ErrCircuitBreakerOpen {
		t.Errorf("Extreme price error = %v, want %v", err, ErrCircuitBreakerOpen)
	}
}

func TestCircuitBreaker_FailureLimit(t *testing.T) {
	cb := NewCircuitBreaker(0.05)
	cb.mu.Lock()
	cb.cautionMode = false
	cb.mu.Unlock()

	pair := "XRP-USD"

	basePrice := decimal.NewFromFloat(1.0)
	for i := 0; i < 20; i++ {
		cb.RecordTrade(pair, basePrice)
	}

	extremePrice := decimal.NewFromFloat(1.20)

	for i := 0; i < DefaultFailureLimit-1; i++ {
		err := cb.CheckPrice(pair, extremePrice)
		if err != ErrCircuitBreakerOpen {
			t.Errorf("Check %d error = %v, want %v", i, err, ErrCircuitBreakerOpen)
		}
		if cb.GetState(pair) != StateClosed {
			t.Errorf("State after %d failures = %s, want %s", i+1, cb.GetState(pair), StateClosed)
		}
	}

	err := cb.CheckPrice(pair, extremePrice)
	if err != ErrCircuitBreakerOpen {
		t.Errorf("Final check error = %v, want %v", err, ErrCircuitBreakerOpen)
	}

	if cb.GetState(pair) != StateOpen {
		t.Errorf("State after failure limit = %s, want %s", cb.GetState(pair), StateOpen)
	}
}

func TestCircuitBreaker_Recovery(t *testing.T) {
	cb := NewCircuitBreaker(0.05)
	cb.mu.Lock()
	cb.cautionMode = false
	cb.mu.Unlock()

	pair := "XRP-USD"

	basePrice := decimal.NewFromFloat(1.0)
	for i := 0; i < 10; i++ {
		cb.RecordTrade(pair, basePrice)
	}

	extremePrice := decimal.NewFromFloat(1.20)
	for i := 0; i < DefaultFailureLimit; i++ {
		_ = cb.CheckPrice(pair, extremePrice)
	}

	if cb.GetState(pair) != StateOpen {
		t.Fatal("Circuit breaker should be open")
	}

	time.Sleep(31 * time.Second)

	err := cb.CheckPrice(pair, basePrice)
	if err != nil {
		t.Errorf("CheckPrice() during half-open error = %v", err)
	}

	if cb.GetState(pair) != StateClosed {
		t.Errorf("State after recovery = %s, want %s", cb.GetState(pair), StateClosed)
	}
}

func TestCircuitBreaker_CautionMode(t *testing.T) {
	cb := NewCircuitBreaker(0.10)
	cb.EnableCautionMode(500 * time.Millisecond)

	pair := "XRP-USD"
	basePrice := decimal.NewFromFloat(1.0)

	for i := 0; i < 10; i++ {
		cb.RecordTrade(pair, basePrice)
	}

	moderatePrice := decimal.NewFromFloat(1.07)

	err := cb.CheckPrice(pair, moderatePrice)
	if err != ErrCircuitBreakerOpen {
		t.Errorf("Caution mode should reject 7%% deviation with 10%% threshold (5%% in caution)")
	}

	time.Sleep(600 * time.Millisecond)

	err = cb.CheckPrice(pair, moderatePrice)
	if err != nil {
		t.Errorf("After caution mode, 7%% deviation should be accepted with 10%% threshold")
	}
}

func TestCircuitBreaker_PersistCallback(t *testing.T) {
	cb := NewCircuitBreaker(DefaultThreshold)
	cb.mu.Lock()
	cb.cautionMode = false
	cb.mu.Unlock()

	var persistedPair string
	var persistedState *breakerState
	var callCount int

	cb.SetPersistCallback(func(pair string, state *breakerState) {
		persistedPair = pair
		persistedState = state
		callCount++
	})

	pair := "XRP-USD"
	price := decimal.NewFromFloat(1.5)

	cb.RecordTrade(pair, price)

	if callCount == 0 {
		t.Error("Persist callback not called")
	}
	if persistedPair != pair {
		t.Errorf("Persisted pair = %s, want %s", persistedPair, pair)
	}
	if persistedState == nil {
		t.Error("Persisted state is nil")
	}
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(DefaultThreshold)
	cb.mu.Lock()
	cb.cautionMode = false
	cb.mu.Unlock()

	pair := "XRP-USD"

	basePrice := decimal.NewFromFloat(1.0)
	for i := 0; i < 20; i++ {
		cb.RecordTrade(pair, basePrice)
	}

	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(id int) {
			price := basePrice.Add(decimal.NewFromFloat(float64(id % 10) * 0.001))
			_ = cb.CheckPrice(pair, price)
			cb.RecordTrade(pair, price)
			_ = cb.GetState(pair)
			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}
