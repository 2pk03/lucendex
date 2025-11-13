package kv

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

func TestMemoryStore_BasicOperations(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		key       string
		value     []byte
		ttl       time.Duration
		wantErr   bool
	}{
		{
			name:      "valid set and get",
			namespace: "test",
			key:       "key1",
			value:     []byte("value1"),
			ttl:       0,
			wantErr:   false,
		},
		{
			name:      "empty namespace",
			namespace: "",
			key:       "key1",
			value:     []byte("value1"),
			ttl:       0,
			wantErr:   true,
		},
		{
			name:      "empty key",
			namespace: "test",
			key:       "",
			value:     []byte("value1"),
			ttl:       0,
			wantErr:   true,
		},
		{
			name:      "key too long",
			namespace: "test",
			key:       string(make([]byte, 257)),
			value:     []byte("value1"),
			ttl:       0,
			wantErr:   true,
		},
		{
			name:      "value too large",
			namespace: "test",
			key:       "key1",
			value:     make([]byte, 1024*1024+1),
			ttl:       0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMemoryStore()
			defer store.Close()

			err := store.Set(tt.namespace, tt.key, tt.value, tt.ttl)
			if (err != nil) != tt.wantErr {
				t.Errorf("Set() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				val, ok := store.Get(tt.namespace, tt.key)
				if !ok {
					t.Error("Get() returned false, expected true")
					return
				}
				if !bytes.Equal(val, tt.value) {
					t.Errorf("Get() = %v, want %v", val, tt.value)
				}
			}
		})
	}
}

func TestMemoryStore_TTLExpiration(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	namespace := "test"
	key := "expiring-key"
	value := []byte("value")
	ttl := 100 * time.Millisecond

	if err := store.Set(namespace, key, value, ttl); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	val, ok := store.Get(namespace, key)
	if !ok {
		t.Error("Get() immediately after Set() returned false")
	}
	if !bytes.Equal(val, value) {
		t.Errorf("Get() = %v, want %v", val, value)
	}

	time.Sleep(150 * time.Millisecond)

	val, ok = store.Get(namespace, key)
	if ok {
		t.Error("Get() after TTL expiration returned true, expected false")
	}
	if val != nil {
		t.Errorf("Get() after TTL = %v, want nil", val)
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	namespace := "test"
	key := "key1"
	value := []byte("value1")

	if err := store.Set(namespace, key, value, 0); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if err := store.Delete(namespace, key); err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	_, ok := store.Get(namespace, key)
	if ok {
		t.Error("Get() after Delete() returned true, expected false")
	}

	err := store.Delete(namespace, "nonexistent")
	if err != ErrKeyNotFound {
		t.Errorf("Delete() error = %v, want %v", err, ErrKeyNotFound)
	}
}

func TestMemoryStore_NamespaceIsolation(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	key := "shared-key"
	value1 := []byte("value-ns1")
	value2 := []byte("value-ns2")

	if err := store.Set("ns1", key, value1, 0); err != nil {
		t.Fatalf("Set(ns1) error = %v", err)
	}
	if err := store.Set("ns2", key, value2, 0); err != nil {
		t.Fatalf("Set(ns2) error = %v", err)
	}

	val1, ok1 := store.Get("ns1", key)
	val2, ok2 := store.Get("ns2", key)

	if !ok1 || !ok2 {
		t.Error("Get() returned false for namespaces")
	}
	if !bytes.Equal(val1, value1) {
		t.Errorf("Get(ns1) = %v, want %v", val1, value1)
	}
	if !bytes.Equal(val2, value2) {
		t.Errorf("Get(ns2) = %v, want %v", val2, value2)
	}
}

func TestMemoryStore_IncrementRateLimit(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	partnerID := "partner-123"
	ttl := 1 * time.Second

	count1, err := store.IncrementRateLimit(partnerID, ttl)
	if err != nil {
		t.Fatalf("IncrementRateLimit() error = %v", err)
	}
	if count1 != 1 {
		t.Errorf("IncrementRateLimit() = %d, want 1", count1)
	}

	count2, err := store.IncrementRateLimit(partnerID, ttl)
	if err != nil {
		t.Fatalf("IncrementRateLimit() error = %v", err)
	}
	if count2 != 2 {
		t.Errorf("IncrementRateLimit() = %d, want 2", count2)
	}

	time.Sleep(1100 * time.Millisecond)

	count3, err := store.IncrementRateLimit(partnerID, ttl)
	if err != nil {
		t.Fatalf("IncrementRateLimit() after expiry error = %v", err)
	}
	if count3 != 1 {
		t.Errorf("IncrementRateLimit() after expiry = %d, want 1", count3)
	}
}

func TestMemoryStore_QuoteOperations(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	var hash [32]byte
	copy(hash[:], []byte("test-quote-hash-1234567890123456"))
	route := []byte("serialized-route-data")

	if err := store.SetQuote(hash, route, 1*time.Second); err != nil {
		t.Fatalf("SetQuote() error = %v", err)
	}

	val, ok := store.GetQuote(hash)
	if !ok {
		t.Error("GetQuote() returned false")
	}
	if !bytes.Equal(val, route) {
		t.Errorf("GetQuote() = %v, want %v", val, route)
	}

	time.Sleep(1100 * time.Millisecond)

	val, ok = store.GetQuote(hash)
	if ok {
		t.Error("GetQuote() after expiry returned true")
	}
	if val != nil {
		t.Errorf("GetQuote() after expiry = %v, want nil", val)
	}
}

func TestMemoryStore_MemoryLimit(t *testing.T) {
	maxBytes := int64(1024)
	store := NewMemoryStoreWithConfig(maxBytes, 256, 512)
	defer store.Close()

	namespace := "test"
	largeValue := make([]byte, 400)

	for i := 0; i < 10; i++ {
		key := string(rune('a' + i))
		err := store.Set(namespace, key, largeValue, 0)
		if err == ErrMemoryLimit {
			stats := store.Stats()
			if stats.Evictions == 0 {
				t.Error("Expected evictions to occur")
			}
			return
		}
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}
	}

	stats := store.Stats()
	if stats.CurrentBytes > maxBytes {
		t.Errorf("CurrentBytes = %d, exceeds maxBytes = %d", stats.CurrentBytes, maxBytes)
	}
}

func TestMemoryStore_LRUEviction(t *testing.T) {
	maxBytes := int64(500)
	store := NewMemoryStoreWithConfig(maxBytes, 256, 200)
	defer store.Close()

	namespace := "test"
	value := make([]byte, 100)

	if err := store.Set(namespace, "key1", value, 0); err != nil {
		t.Fatalf("Set(key1) error = %v", err)
	}
	if err := store.Set(namespace, "key2", value, 0); err != nil {
		t.Fatalf("Set(key2) error = %v", err)
	}

	_, _ = store.Get(namespace, "key1")

	if err := store.Set(namespace, "key3", value, 0); err != nil {
		t.Fatalf("Set(key3) error = %v", err)
	}

	_, ok1 := store.Get(namespace, "key1")
	_, ok2 := store.Get(namespace, "key2")
	_, ok3 := store.Get(namespace, "key3")

	if !ok1 {
		t.Error("key1 should still exist (recently accessed)")
	}
	if ok2 {
		t.Error("key2 should have been evicted (LRU)")
	}
	if !ok3 {
		t.Error("key3 should exist (just added)")
	}
}

func TestMemoryStore_Keys(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	namespace := "test"
	keys := []string{"key1", "key2", "key3"}

	for _, k := range keys {
		if err := store.Set(namespace, k, []byte("value"), 0); err != nil {
			t.Fatalf("Set(%s) error = %v", k, err)
		}
	}

	result := store.Keys(namespace)
	if len(result) != len(keys) {
		t.Errorf("Keys() returned %d keys, want %d", len(result), len(keys))
	}

	keyMap := make(map[string]bool)
	for _, k := range result {
		keyMap[k] = true
	}
	for _, k := range keys {
		if !keyMap[k] {
			t.Errorf("Keys() missing key %s", k)
		}
	}
}

func TestMemoryStore_Stats(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	if err := store.Set("ns1", "key1", []byte("value1"), 0); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := store.Set("ns2", "key2", []byte("value2"), 0); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	_, _ = store.Get("ns1", "key1")
	_, _ = store.Get("ns1", "nonexistent")

	stats := store.Stats()

	if stats.TotalKeys != 2 {
		t.Errorf("Stats.TotalKeys = %d, want 2", stats.TotalKeys)
	}
	if stats.Hits != 1 {
		t.Errorf("Stats.Hits = %d, want 1", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Stats.Misses = %d, want 1", stats.Misses)
	}
	if stats.NamespaceCounts["ns1"] != 1 {
		t.Errorf("Stats.NamespaceCounts[ns1] = %d, want 1", stats.NamespaceCounts["ns1"])
	}
	if stats.NamespaceCounts["ns2"] != 1 {
		t.Errorf("Stats.NamespaceCounts[ns2] = %d, want 1", stats.NamespaceCounts["ns2"])
	}
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	var wg sync.WaitGroup
	numGoroutines := 100
	numOpsPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			namespace := "test"
			for j := 0; j < numOpsPerGoroutine; j++ {
				key := string(rune('a' + (id % 26)))
				value := []byte{byte(id), byte(j)}

				_ = store.Set(namespace, key, value, 0)
				_, _ = store.Get(namespace, key)

				if j%10 == 0 {
					_ = store.Delete(namespace, key)
				}
			}
		}(i)
	}

	wg.Wait()

	stats := store.Stats()
	if stats.CurrentBytes < 0 {
		t.Errorf("Stats.CurrentBytes = %d, should be >= 0", stats.CurrentBytes)
	}
}

func TestMemoryStore_CleanupExpiredEntries(t *testing.T) {
	store := NewMemoryStoreWithConfig(DefaultMaxBytes, DefaultMaxKeyLength, DefaultMaxValueSize)
	defer store.Close()

	namespace := "test"
	ttl := 50 * time.Millisecond

	for i := 0; i < 10; i++ {
		key := string(rune('a' + i))
		if err := store.Set(namespace, key, []byte("value"), ttl); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
	}

	stats1 := store.Stats()
	if stats1.TotalKeys != 10 {
		t.Errorf("TotalKeys before expiry = %d, want 10", stats1.TotalKeys)
	}

	time.Sleep(100 * time.Millisecond)
	store.cleanup()

	stats2 := store.Stats()
	if stats2.TotalKeys != 0 {
		t.Errorf("TotalKeys after cleanup = %d, want 0", stats2.TotalKeys)
	}
}

func TestMemoryStore_NamespaceQuota(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	namespace := NamespaceCircuitBreaker
	quota := namespaceQuotas[namespace]

	for i := int64(0); i < quota; i++ {
		key := string(rune('a' + int(i%26))) + string(rune('A' + int(i/26)))
		if err := store.Set(namespace, key, []byte("value"), 0); err != nil {
			t.Fatalf("Set(%d) error = %v", i, err)
		}
	}

	err := store.Set(namespace, "over-quota", []byte("value"), 0)
	if err != ErrNamespaceQuota {
		t.Errorf("Set() over quota error = %v, want %v", err, ErrNamespaceQuota)
	}
}

func TestMemoryStore_Close(t *testing.T) {
	store := NewMemoryStore()

	if err := store.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if err := store.Close(); err != nil {
		t.Errorf("Close() second time error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)
}
