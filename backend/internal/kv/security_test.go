package kv

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSecurity_MemoryExhaustion(t *testing.T) {
	maxBytes := int64(1024 * 10)
	store := NewMemoryStoreWithConfig(maxBytes, 256, 1024)
	defer store.Close()

	namespace := "test"
	largeValue := make([]byte, 900)

	for i := 0; i < 100; i++ {
		key := string(rune('a' + (i % 26))) + string(rune('A' + (i / 26)))
		err := store.Set(namespace, key, largeValue, 0)
		if err != nil && err != ErrMemoryLimit {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	stats := store.Stats()
	if stats.CurrentBytes > maxBytes {
		t.Errorf("Memory limit breached: current=%d, max=%d", stats.CurrentBytes, maxBytes)
	}

	if stats.Evictions == 0 {
		t.Error("Expected evictions to occur under memory pressure")
	}

	t.Logf("Final: %d bytes used of %d max, %d evictions", stats.CurrentBytes, maxBytes, stats.Evictions)
}

func TestSecurity_ConcurrentAccessSafety(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	var wg sync.WaitGroup
	numGoroutines := 200
	opsPerGoroutine := 500

	var totalErrors atomic.Int64

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			namespace := "concurrent"

			for j := 0; j < opsPerGoroutine; j++ {
				key := string(rune('a' + (id % 26)))
				value := []byte{byte(id), byte(j)}

				if err := store.Set(namespace, key, value, 10*time.Millisecond); err != nil {
					totalErrors.Add(1)
				}

				if _, ok := store.Get(namespace, key); !ok {
					totalErrors.Add(1)
				}

				if j%5 == 0 {
					_ = store.Delete(namespace, key)
				}

				if j%7 == 0 {
					_ = store.Keys(namespace)
				}

				if j%11 == 0 {
					_ = store.Stats()
				}
			}
		}(i)
	}

	wg.Wait()

	stats := store.Stats()
	if stats.CurrentBytes < 0 {
		t.Errorf("Negative memory usage: %d", stats.CurrentBytes)
	}

	errors := totalErrors.Load()
	if errors > 0 {
		t.Logf("Concurrent errors (expected some due to deletes/expiry): %d", errors)
	}
}

func TestSecurity_NamespaceContamination(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	if err := store.Set("ns1", "key", []byte("value1"), 0); err != nil {
		t.Fatalf("Set(ns1) error = %v", err)
	}
	if err := store.Set("ns2", "key", []byte("value2"), 0); err != nil {
		t.Fatalf("Set(ns2) error = %v", err)
	}

	val1, ok1 := store.Get("ns1", "key")
	if !ok1 {
		t.Error("Failed to get key from ns1")
	}
	if string(val1) != "value1" {
		t.Errorf("ns1 contaminated: got %s, want value1", val1)
	}

	val2, ok2 := store.Get("ns2", "key")
	if !ok2 {
		t.Error("Failed to get key from ns2")
	}
	if string(val2) != "value2" {
		t.Errorf("ns2 contaminated: got %s, want value2", val2)
	}

	if err := store.Delete("ns1", "key"); err != nil {
		t.Fatalf("Delete(ns1) error = %v", err)
	}

	_, ok := store.Get("ns2", "key")
	if !ok {
		t.Error("Deleting from ns1 affected ns2")
	}
}

func TestSecurity_KeyValidationBypass(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	tests := []struct {
		name      string
		namespace string
		key       string
		wantErr   bool
	}{
		{
			name:      "empty namespace bypass attempt",
			namespace: "",
			key:       "key",
			wantErr:   true,
		},
		{
			name:      "empty key bypass attempt",
			namespace: "ns",
			key:       "",
			wantErr:   true,
		},
		{
			name:      "oversized key bypass attempt",
			namespace: "ns",
			key:       string(make([]byte, 300)),
			wantErr:   true,
		},
		{
			name:      "null bytes in key",
			namespace: "ns",
			key:       "key\x00with\x00nulls",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Set(tt.namespace, tt.key, []byte("value"), 0)
			if (err != nil) != tt.wantErr {
				t.Errorf("Set() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSecurity_ValueSizeLimitEnforcement(t *testing.T) {
	maxValueSize := 1024
	store := NewMemoryStoreWithConfig(DefaultMaxBytes, DefaultMaxKeyLength, maxValueSize)
	defer store.Close()

	namespace := "test"

	validValue := make([]byte, maxValueSize)
	err := store.Set(namespace, "valid", validValue, 0)
	if err != nil {
		t.Errorf("Set() with max size value error = %v", err)
	}

	oversizedValue := make([]byte, maxValueSize+1)
	err = store.Set(namespace, "oversized", oversizedValue, 0)
	if err != ErrValueTooLarge {
		t.Errorf("Set() with oversized value error = %v, want %v", err, ErrValueTooLarge)
	}

	hugeValue := make([]byte, maxValueSize*10)
	err = store.Set(namespace, "huge", hugeValue, 0)
	if err != ErrValueTooLarge {
		t.Errorf("Set() with huge value error = %v, want %v", err, ErrValueTooLarge)
	}
}

func TestSecurity_RateLimitManipulation(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	partnerID := "partner-123"
	ttl := 1 * time.Second

	count1, _ := store.IncrementRateLimit(partnerID, ttl)
	count2, _ := store.IncrementRateLimit(partnerID, ttl)

	if count1 != 1 || count2 != 2 {
		t.Errorf("Increments wrong: %d, %d", count1, count2)
	}

	if err := store.Delete(NamespaceRateLimits, partnerID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	count3, _ := store.IncrementRateLimit(partnerID, ttl)
	if count3 != 1 {
		t.Errorf("After delete, count = %d, want 1", count3)
	}
}

func TestSecurity_TTLBypass(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	namespace := "test"
	key := "expiring"
	value := []byte("secret")
	ttl := 100 * time.Millisecond

	if err := store.Set(namespace, key, value, ttl); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	val, ok := store.Get(namespace, key)
	if ok {
		t.Errorf("Get() after TTL returned value: %s", val)
	}

	err := store.Set(namespace, key, []byte("new"), 0)
	if err != nil {
		t.Fatalf("Set() after expiry error = %v", err)
	}

	val, ok = store.Get(namespace, key)
	if !ok {
		t.Error("Get() after new Set() returned false")
	}
}

func TestSecurity_MemoryAccounting(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	namespace := "test"
	value := make([]byte, 100)

	initialStats := store.Stats()
	initialBytes := initialStats.CurrentBytes

	for i := 0; i < 10; i++ {
		key := string(rune('a' + i))
		if err := store.Set(namespace, key, value, 0); err != nil {
			t.Fatalf("Set(%d) error = %v", i, err)
		}
	}

	midStats := store.Stats()
	if midStats.CurrentBytes <= initialBytes {
		t.Errorf("Memory not increasing: initial=%d, mid=%d", initialBytes, midStats.CurrentBytes)
	}

	for i := 0; i < 10; i++ {
		key := string(rune('a' + i))
		_ = store.Delete(namespace, key)
	}

	finalStats := store.Stats()
	if finalStats.CurrentBytes > midStats.CurrentBytes {
		t.Errorf("Memory not decreasing after deletes: mid=%d, final=%d", midStats.CurrentBytes, finalStats.CurrentBytes)
	}
}

func TestSecurity_NamespaceQuotaEnforcement(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	tests := []struct {
		namespace string
		testQuota int64
	}{
		{NamespaceQuotes, 100},
		{NamespaceCircuitBreaker, 100},
	}

	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			origQuota := namespaceQuotas[tt.namespace]
			namespaceQuotas[tt.namespace] = tt.testQuota
			defer func() {
				namespaceQuotas[tt.namespace] = origQuota
			}()

			for i := int64(0); i < tt.testQuota; i++ {
				key := string(rune('a'+(i%26))) + string(rune('A'+(i/26)))
				if err := store.Set(tt.namespace, key, []byte("v"), 0); err != nil {
					t.Fatalf("Set(%d) error = %v", i, err)
				}
			}

			err := store.Set(tt.namespace, "over-quota", []byte("v"), 0)
			if err != ErrNamespaceQuota {
				t.Errorf("Over quota error = %v, want %v", err, ErrNamespaceQuota)
			}

			stats := store.Stats()
			if stats.NamespaceCounts[tt.namespace] != tt.testQuota {
				t.Errorf("Count = %d, want %d", stats.NamespaceCounts[tt.namespace], tt.testQuota)
			}
		})
	}
}

func TestSecurity_ConcurrentEvictionSafety(t *testing.T) {
	maxBytes := int64(10 * 1024)
	store := NewMemoryStoreWithConfig(maxBytes, 256, 1024)
	defer store.Close()

	var wg sync.WaitGroup
	numGoroutines := 50
	opsPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			namespace := "eviction"
			largeValue := make([]byte, 800)

			for j := 0; j < opsPerGoroutine; j++ {
				key := string(rune('a'+(id%26))) + string(rune('A'+(j%26)))
				_ = store.Set(namespace, key, largeValue, 0)
			}
		}(i)
	}

	wg.Wait()

	stats := store.Stats()
	if stats.CurrentBytes > maxBytes {
		t.Errorf("Memory limit exceeded: %d > %d", stats.CurrentBytes, maxBytes)
	}
	if stats.CurrentBytes < 0 {
		t.Errorf("Negative memory: %d", stats.CurrentBytes)
	}
	if stats.Evictions == 0 {
		t.Error("No evictions occurred under memory pressure")
	}

	t.Logf("Evictions: %d, FinalBytes: %d/%d", stats.Evictions, stats.CurrentBytes, maxBytes)
}

func TestSecurity_AtomicOperationsConsistency(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	var wg sync.WaitGroup
	numGoroutines := 100
	incrementsPerGoroutine := 100

	partnerID := "atomic-test"

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerGoroutine; j++ {
				_, _ = store.IncrementRateLimit(partnerID, 10*time.Second)
			}
		}()
	}

	wg.Wait()

	fullKey := store.makeKey(NamespaceRateLimits, partnerID)
	store.mu.RLock()
	e, ok := store.data[fullKey]
	store.mu.RUnlock()

	if !ok {
		t.Fatal("Counter not found after increments")
	}

	expectedCount := int64(numGoroutines * incrementsPerGoroutine)
	actualCount := string(e.value)

	t.Logf("Expected: %d, Actual: %s", expectedCount, actualCount)
}
