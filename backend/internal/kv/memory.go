package kv

import (
	"container/list"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultMaxBytes        = 512 * 1024 * 1024
	DefaultMaxKeyLength    = 256
	DefaultMaxValueSize    = 1024 * 1024
	DefaultCleanupInterval = 60 * time.Second

	NamespaceQuotes         = "quotes"
	NamespaceRateLimits     = "rate_limits"
	NamespaceCircuitBreaker = "circuit_breaker"
	NamespaceSystem         = "system"
)

var namespaceQuotas = map[string]int64{
	NamespaceQuotes:         10000,
	NamespaceRateLimits:     100000,
	NamespaceCircuitBreaker: 1000,
	NamespaceSystem:         128,
}

type entry struct {
	namespace string
	key       string
	value     []byte
	expiresAt time.Time
	size      int
	lruNode   *list.Element
}

type MemoryStore struct {
	mu           sync.RWMutex
	data         map[string]*entry
	lru          *list.List
	maxBytes     int64
	currentBytes int64
	maxKeyLength int
	maxValueSize int
	evictions    int64
	hits         int64
	misses       int64
	stopCh       chan struct{}
	stopped      atomic.Bool
}

func NewMemoryStore() *MemoryStore {
	return NewMemoryStoreWithConfig(DefaultMaxBytes, DefaultMaxKeyLength, DefaultMaxValueSize)
}

func NewMemoryStoreWithConfig(maxBytes int64, maxKeyLength, maxValueSize int) *MemoryStore {
	s := &MemoryStore{
		data:         make(map[string]*entry),
		lru:          list.New(),
		maxBytes:     maxBytes,
		maxKeyLength: maxKeyLength,
		maxValueSize: maxValueSize,
		stopCh:       make(chan struct{}),
	}
	go s.cleanupLoop()
	return s
}

func (s *MemoryStore) Get(namespace, key string) ([]byte, bool) {
	if namespace == "" || key == "" {
		atomic.AddInt64(&s.misses, 1)
		return nil, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	fullKey := s.makeKey(namespace, key)
	e, ok := s.data[fullKey]
	if !ok {
		atomic.AddInt64(&s.misses, 1)
		return nil, false
	}

	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		s.deleteEntryLocked(fullKey, e)
		atomic.AddInt64(&s.misses, 1)
		return nil, false
	}

	s.lru.MoveToFront(e.lruNode)
	atomic.AddInt64(&s.hits, 1)

	result := make([]byte, len(e.value))
	copy(result, e.value)
	return result, true
}

func (s *MemoryStore) Set(namespace, key string, value []byte, ttl time.Duration) error {
	if err := s.validate(namespace, key, value); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.checkNamespaceQuota(namespace); err != nil {
		return err
	}

	fullKey := s.makeKey(namespace, key)
	entrySize := len(fullKey) + len(value) + 64

	if existing, ok := s.data[fullKey]; ok {
		atomic.AddInt64(&s.currentBytes, -int64(existing.size))
		s.lru.Remove(existing.lruNode)
	}

	for s.currentBytes+int64(entrySize) > s.maxBytes {
		if !s.evictOldest() {
			return ErrMemoryLimit
		}
	}

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	e := &entry{
		namespace: namespace,
		key:       key,
		value:     valueCopy,
		expiresAt: expiresAt,
		size:      entrySize,
	}
	e.lruNode = s.lru.PushFront(fullKey)
	s.data[fullKey] = e
	atomic.AddInt64(&s.currentBytes, int64(entrySize))

	return nil
}

func (s *MemoryStore) Delete(namespace, key string) error {
	if namespace == "" || key == "" {
		return ErrKeyEmpty
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	fullKey := s.makeKey(namespace, key)
	e, ok := s.data[fullKey]
	if !ok {
		return ErrKeyNotFound
	}

	s.deleteEntryLocked(fullKey, e)
	return nil
}

func (s *MemoryStore) IncrementRateLimit(partnerID string, ttl time.Duration) (int64, error) {
	if partnerID == "" {
		return 0, ErrKeyEmpty
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	fullKey := s.makeKey(NamespaceRateLimits, partnerID)
	e, ok := s.data[fullKey]

	if !ok || (!e.expiresAt.IsZero() && time.Now().After(e.expiresAt)) {
		if ok {
			s.deleteEntryLocked(fullKey, e)
		}

		if err := s.setLocked(NamespaceRateLimits, partnerID, []byte("1"), ttl); err != nil {
			return 0, err
		}
		return 1, nil
	}

	count, err := strconv.ParseInt(string(e.value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid counter value: %w", err)
	}
	count++

	newValue := []byte(strconv.FormatInt(count, 10))
	if err := s.setLocked(NamespaceRateLimits, partnerID, newValue, ttl); err != nil {
		return 0, err
	}

	return count, nil
}

func (s *MemoryStore) GetQuote(hash [32]byte) ([]byte, bool) {
	key := hex.EncodeToString(hash[:])
	return s.Get(NamespaceQuotes, key)
}

func (s *MemoryStore) SetQuote(hash [32]byte, route []byte, ttl time.Duration) error {
	key := hex.EncodeToString(hash[:])
	return s.Set(NamespaceQuotes, key, route, ttl)
}

func (s *MemoryStore) SetLedgerIndex(idx uint32) error {
	value := []byte(strconv.FormatUint(uint64(idx), 10))
	return s.Set(NamespaceSystem, "ledger_index", value, 0)
}

func (s *MemoryStore) GetLedgerIndex() (uint32, bool) {
	value, ok := s.Get(NamespaceSystem, "ledger_index")
	if !ok {
		return 0, false
	}

	parsed, err := strconv.ParseUint(string(value), 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(parsed), true
}

func (s *MemoryStore) Keys(namespace string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var keys []string

	for _, e := range s.data {
		if e.namespace == namespace {
			if e.expiresAt.IsZero() || now.Before(e.expiresAt) {
				keys = append(keys, e.key)
			}
		}
	}

	return keys
}

func (s *MemoryStore) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	namespaceCounts := make(map[string]int64)
	totalKeys := int64(0)

	now := time.Now()
	for _, e := range s.data {
		if e.expiresAt.IsZero() || now.Before(e.expiresAt) {
			namespaceCounts[e.namespace]++
			totalKeys++
		}
	}

	return Stats{
		TotalKeys:       totalKeys,
		CurrentBytes:    atomic.LoadInt64(&s.currentBytes),
		MaxBytes:        s.maxBytes,
		Evictions:       atomic.LoadInt64(&s.evictions),
		Hits:            atomic.LoadInt64(&s.hits),
		Misses:          atomic.LoadInt64(&s.misses),
		NamespaceCounts: namespaceCounts,
	}
}

func (s *MemoryStore) Close() error {
	if s.stopped.Swap(true) {
		return nil
	}
	close(s.stopCh)
	return nil
}

func (s *MemoryStore) validate(namespace, key string, value []byte) error {
	if namespace == "" {
		return ErrNamespaceEmpty
	}
	if key == "" {
		return ErrKeyEmpty
	}
	if len(key) > s.maxKeyLength {
		return ErrKeyTooLong
	}
	if len(value) > s.maxValueSize {
		return ErrValueTooLarge
	}
	return nil
}

func (s *MemoryStore) checkNamespaceQuota(namespace string) error {
	quota, ok := namespaceQuotas[namespace]
	if !ok {
		return nil
	}

	count := int64(0)
	now := time.Now()
	for _, e := range s.data {
		if e.namespace == namespace {
			if e.expiresAt.IsZero() || now.Before(e.expiresAt) {
				count++
			}
		}
	}

	if count >= quota {
		return ErrNamespaceQuota
	}
	return nil
}

func (s *MemoryStore) setLocked(namespace, key string, value []byte, ttl time.Duration) error {
	fullKey := s.makeKey(namespace, key)
	entrySize := len(fullKey) + len(value) + 64

	if existing, ok := s.data[fullKey]; ok {
		atomic.AddInt64(&s.currentBytes, -int64(existing.size))
		s.lru.Remove(existing.lruNode)
	}

	for s.currentBytes+int64(entrySize) > s.maxBytes {
		if !s.evictOldest() {
			return ErrMemoryLimit
		}
	}

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	e := &entry{
		namespace: namespace,
		key:       key,
		value:     valueCopy,
		expiresAt: expiresAt,
		size:      entrySize,
	}
	e.lruNode = s.lru.PushFront(fullKey)
	s.data[fullKey] = e
	atomic.AddInt64(&s.currentBytes, int64(entrySize))

	return nil
}

func (s *MemoryStore) evictOldest() bool {
	elem := s.lru.Back()
	if elem == nil {
		return false
	}

	fullKey := elem.Value.(string)
	if e, ok := s.data[fullKey]; ok {
		s.deleteEntryLocked(fullKey, e)
		atomic.AddInt64(&s.evictions, 1)
		return true
	}
	return false
}

func (s *MemoryStore) deleteEntryLocked(fullKey string, e *entry) {
	if e.lruNode != nil {
		s.lru.Remove(e.lruNode)
	}
	delete(s.data, fullKey)
	atomic.AddInt64(&s.currentBytes, -int64(e.size))
}

func (s *MemoryStore) makeKey(namespace, key string) string {
	return namespace + ":" + key
}

func (s *MemoryStore) cleanupLoop() {
	ticker := time.NewTicker(DefaultCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}

func (s *MemoryStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	toDelete := make([]string, 0)

	for fullKey, e := range s.data {
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			toDelete = append(toDelete, fullKey)
		}
	}

	for _, fullKey := range toDelete {
		if e, ok := s.data[fullKey]; ok {
			s.deleteEntryLocked(fullKey, e)
		}
	}
}
