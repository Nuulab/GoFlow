// Package cache provides an in-memory cache for testing and development.
package cache

import (
	"context"
	"sync"
	"time"
)

// MemoryCache implements Cache using an in-memory map.
// Useful for testing and development without DragonflyDB.
type MemoryCache struct {
	mu      sync.RWMutex
	data    map[string]cacheEntry
	config  Config
	closed  bool
	cleanup *time.Ticker
	done    chan struct{}
}

type cacheEntry struct {
	value     []byte
	expiresAt time.Time
}

// NewMemoryCache creates a new in-memory cache.
func NewMemoryCache(cfg Config) *MemoryCache {
	mc := &MemoryCache{
		data:   make(map[string]cacheEntry),
		config: cfg,
		done:   make(chan struct{}),
	}

	// Start background cleanup goroutine
	mc.cleanup = time.NewTicker(1 * time.Minute)
	go mc.cleanupLoop()

	return mc
}

// cleanupLoop periodically removes expired entries.
func (mc *MemoryCache) cleanupLoop() {
	for {
		select {
		case <-mc.cleanup.C:
			mc.removeExpired()
		case <-mc.done:
			return
		}
	}
}

// removeExpired removes all expired entries.
func (mc *MemoryCache) removeExpired() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()
	for key, entry := range mc.data {
		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			delete(mc.data, key)
		}
	}
}

// prefixKey adds the configured prefix.
func (mc *MemoryCache) prefixKey(key string) string {
	if mc.config.Prefix == "" {
		return key
	}
	return mc.config.Prefix + ":" + key
}

// Get retrieves a value from memory.
func (mc *MemoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if mc.closed {
		return nil, ErrCacheMiss
	}

	entry, ok := mc.data[mc.prefixKey(key)]
	if !ok {
		return nil, ErrCacheMiss
	}

	// Check expiration
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		return nil, ErrCacheMiss
	}

	// Return a copy to prevent mutation
	result := make([]byte, len(entry.value))
	copy(result, entry.value)
	return result, nil
}

// Set stores a value in memory.
func (mc *MemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.closed {
		return ErrCacheMiss
	}

	if ttl == 0 {
		ttl = mc.config.DefaultTTL
	}

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	// Store a copy
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	mc.data[mc.prefixKey(key)] = cacheEntry{
		value:     valueCopy,
		expiresAt: expiresAt,
	}

	return nil
}

// Delete removes a key from memory.
func (mc *MemoryCache) Delete(ctx context.Context, key string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.data, mc.prefixKey(key))
	return nil
}

// Exists checks if a key exists and is not expired.
func (mc *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	entry, ok := mc.data[mc.prefixKey(key)]
	if !ok {
		return false, nil
	}

	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		return false, nil
	}

	return true, nil
}

// Clear removes all keys.
func (mc *MemoryCache) Clear(ctx context.Context) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.data = make(map[string]cacheEntry)
	return nil
}

// Close stops the cleanup goroutine.
func (mc *MemoryCache) Close() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if !mc.closed {
		mc.closed = true
		mc.cleanup.Stop()
		close(mc.done)
	}

	return nil
}

// Stats returns cache statistics.
func (mc *MemoryCache) Stats(ctx context.Context) (CacheStats, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return CacheStats{
		KeyCount: int64(len(mc.data)),
	}, nil
}
