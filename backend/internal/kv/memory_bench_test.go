package kv

import (
	"testing"
	"time"
)

func BenchmarkMemoryStore_Get(b *testing.B) {
	store := NewMemoryStore()
	defer store.Close()

	namespace := "bench"
	key := "key1"
	value := []byte("value1")

	_ = store.Set(namespace, key, value, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Get(namespace, key)
	}
}

func BenchmarkMemoryStore_Set(b *testing.B) {
	store := NewMemoryStore()
	defer store.Close()

	namespace := "bench"
	value := []byte("value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := string(rune('a' + (i % 26)))
		_ = store.Set(namespace, key, value, 0)
	}
}

func BenchmarkMemoryStore_SetWithTTL(b *testing.B) {
	store := NewMemoryStore()
	defer store.Close()

	namespace := "bench"
	value := []byte("value")
	ttl := 1 * time.Minute

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := string(rune('a' + (i % 26)))
		_ = store.Set(namespace, key, value, ttl)
	}
}

func BenchmarkMemoryStore_Delete(b *testing.B) {
	store := NewMemoryStore()
	defer store.Close()

	namespace := "bench"
	value := []byte("value")

	for i := 0; i < b.N; i++ {
		key := string(rune('a' + (i % 26)))
		_ = store.Set(namespace, key, value, 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := string(rune('a' + (i % 26)))
		_ = store.Delete(namespace, key)
	}
}

func BenchmarkMemoryStore_IncrementRateLimit(b *testing.B) {
	store := NewMemoryStore()
	defer store.Close()

	partnerID := "partner-123"
	ttl := 1 * time.Minute

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.IncrementRateLimit(partnerID, ttl)
	}
}

func BenchmarkMemoryStore_GetQuote(b *testing.B) {
	store := NewMemoryStore()
	defer store.Close()

	var hash [32]byte
	for i := 0; i < 32; i++ {
		hash[i] = byte(i)
	}
	route := []byte("serialized-route-data")
	_ = store.SetQuote(hash, route, 1*time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.GetQuote(hash)
	}
}

func BenchmarkMemoryStore_SetQuote(b *testing.B) {
	store := NewMemoryStore()
	defer store.Close()

	route := []byte("serialized-route-data")
	ttl := 1 * time.Minute

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var hash [32]byte
		hash[0] = byte(i)
		hash[1] = byte(i >> 8)
		_ = store.SetQuote(hash, route, ttl)
	}
}

func BenchmarkMemoryStore_ConcurrentReads(b *testing.B) {
	store := NewMemoryStore()
	defer store.Close()

	namespace := "bench"
	for i := 0; i < 100; i++ {
		key := string(rune('a' + (i % 26)))
		_ = store.Set(namespace, key, []byte("value"), 0)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := string(rune('a' + (i % 26)))
			_, _ = store.Get(namespace, key)
			i++
		}
	})
}

func BenchmarkMemoryStore_ConcurrentWrites(b *testing.B) {
	store := NewMemoryStore()
	defer store.Close()

	namespace := "bench"
	value := []byte("value")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := string(rune('a' + (i % 26)))
			_ = store.Set(namespace, key, value, 0)
			i++
		}
	})
}

func BenchmarkMemoryStore_ConcurrentMixed(b *testing.B) {
	store := NewMemoryStore()
	defer store.Close()

	namespace := "bench"
	value := []byte("value")

	for i := 0; i < 50; i++ {
		key := string(rune('a' + (i % 26)))
		_ = store.Set(namespace, key, value, 0)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := string(rune('a' + (i % 26)))
			if i%3 == 0 {
				_ = store.Set(namespace, key, value, 0)
			} else {
				_, _ = store.Get(namespace, key)
			}
			i++
		}
	})
}

func BenchmarkMemoryStore_EvictionUnderPressure(b *testing.B) {
	maxBytes := int64(1024 * 100)
	store := NewMemoryStoreWithConfig(maxBytes, 256, 1024)
	defer store.Close()

	namespace := "bench"
	largeValue := make([]byte, 500)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := string(rune('a' + (i % 26)))
		_ = store.Set(namespace, key, largeValue, 0)
	}
}

func BenchmarkMemoryStore_Stats(b *testing.B) {
	store := NewMemoryStore()
	defer store.Close()

	namespace := "bench"
	for i := 0; i < 1000; i++ {
		key := string(rune('a' + (i % 26)))
		_ = store.Set(namespace, key, []byte("value"), 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Stats()
	}
}

func BenchmarkMemoryStore_Keys(b *testing.B) {
	store := NewMemoryStore()
	defer store.Close()

	namespace := "bench"
	for i := 0; i < 1000; i++ {
		key := string(rune('a' + (i % 26)))
		_ = store.Set(namespace, key, []byte("value"), 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Keys(namespace)
	}
}
