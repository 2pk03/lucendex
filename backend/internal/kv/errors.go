package kv

import "errors"

var (
	ErrKeyTooLong       = errors.New("key exceeds maximum length")
	ErrKeyEmpty         = errors.New("key cannot be empty")
	ErrNamespaceEmpty   = errors.New("namespace cannot be empty")
	ErrValueTooLarge    = errors.New("value exceeds maximum size")
	ErrMemoryLimit      = errors.New("memory limit exceeded")
	ErrKeyNotFound      = errors.New("key not found")
	ErrNamespaceQuota   = errors.New("namespace quota exceeded")
	ErrInvalidNamespace = errors.New("invalid namespace")
)
