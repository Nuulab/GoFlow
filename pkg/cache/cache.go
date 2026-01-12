// Package cache provides caching capabilities for GoFlow using DragonflyDB/Redis.
// DragonflyDB is a modern, Redis-compatible in-memory datastore that provides
// high performance caching with lower memory usage.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Cache defines the interface for caching operations.
// Implementations can use DragonflyDB, Redis, or in-memory storage.
type Cache interface {
	// Get retrieves a value from the cache.
	// Returns ErrCacheMiss if the key doesn't exist.
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value in the cache with an optional TTL.
	// TTL of 0 means no expiration.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete removes a key from the cache.
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in the cache.
	Exists(ctx context.Context, key string) (bool, error)

	// Clear removes all keys from the cache (use with caution).
	Clear(ctx context.Context) error

	// Close closes the cache connection.
	Close() error
}

// TypedCache provides type-safe caching with JSON serialization.
type TypedCache[T any] struct {
	cache Cache
}

// NewTypedCache creates a typed cache wrapper.
func NewTypedCache[T any](cache Cache) *TypedCache[T] {
	return &TypedCache[T]{cache: cache}
}

// Get retrieves and deserializes a value.
func (tc *TypedCache[T]) Get(ctx context.Context, key string) (T, error) {
	var result T
	data, err := tc.cache.Get(ctx, key)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(data, &result)
	return result, err
}

// Set serializes and stores a value.
func (tc *TypedCache[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache: failed to marshal value: %w", err)
	}
	return tc.cache.Set(ctx, key, data, ttl)
}

// ErrCacheMiss is returned when a key is not found in the cache.
var ErrCacheMiss = fmt.Errorf("cache: key not found")

// Config holds configuration for cache connections.
type Config struct {
	// Address is the DragonflyDB/Redis server address (host:port).
	Address string
	// Password for authentication (optional).
	Password string
	// Database number to use (default: 0).
	Database int
	// PoolSize is the maximum number of connections.
	PoolSize int
	// Prefix is prepended to all keys.
	Prefix string
	// DefaultTTL is the default expiration time for keys.
	DefaultTTL time.Duration
}

// DefaultConfig returns sensible defaults for local development.
func DefaultConfig() Config {
	return Config{
		Address:    "localhost:6379",
		Database:   0,
		PoolSize:   10,
		DefaultTTL: 5 * time.Minute,
	}
}

// CacheStats holds cache statistics.
type CacheStats struct {
	Hits       int64
	Misses     int64
	KeyCount   int64
	MemoryUsed int64
}

// StatsProvider is an optional interface for caches that provide statistics.
type StatsProvider interface {
	Stats(ctx context.Context) (CacheStats, error)
}
