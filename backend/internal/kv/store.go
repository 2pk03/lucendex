package kv

import "time"

type Store interface {
	Get(namespace, key string) ([]byte, bool)
	Set(namespace, key string, value []byte, ttl time.Duration) error
	Delete(namespace, key string) error
	IncrementRateLimit(partnerID string, ttl time.Duration) (int64, error)
	GetQuote(hash [32]byte) ([]byte, bool)
	SetQuote(hash [32]byte, route []byte, ttl time.Duration) error
	Keys(namespace string) []string
	Stats() Stats
	Close() error
}

type Stats struct {
	TotalKeys       int64
	CurrentBytes    int64
	MaxBytes        int64
	Evictions       int64
	Hits            int64
	Misses          int64
	NamespaceCounts map[string]int64
}
