// Package cache provides a DragonflyDB/Redis cache implementation.
package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// DragonflyCache implements Cache using DragonflyDB (Redis-compatible).
type DragonflyCache struct {
	client *redis.Client
	config Config
}

// NewDragonflyCache creates a new DragonflyDB cache connection.
func NewDragonflyCache(cfg Config) (*DragonflyCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.Database,
		PoolSize: cfg.PoolSize,
	})

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("cache: failed to connect to DragonflyDB at %s: %w", cfg.Address, err)
	}

	return &DragonflyCache{
		client: client,
		config: cfg,
	}, nil
}

// prefixKey adds the configured prefix to a key.
func (dc *DragonflyCache) prefixKey(key string) string {
	if dc.config.Prefix == "" {
		return key
	}
	return dc.config.Prefix + ":" + key
}

// Get retrieves a value from DragonflyDB.
func (dc *DragonflyCache) Get(ctx context.Context, key string) ([]byte, error) {
	result, err := dc.client.Get(ctx, dc.prefixKey(key)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("cache: get failed: %w", err)
	}
	return result, nil
}

// Set stores a value in DragonflyDB.
func (dc *DragonflyCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl == 0 {
		ttl = dc.config.DefaultTTL
	}

	err := dc.client.Set(ctx, dc.prefixKey(key), value, ttl).Err()
	if err != nil {
		return fmt.Errorf("cache: set failed: %w", err)
	}
	return nil
}

// Delete removes a key from DragonflyDB.
func (dc *DragonflyCache) Delete(ctx context.Context, key string) error {
	err := dc.client.Del(ctx, dc.prefixKey(key)).Err()
	if err != nil {
		return fmt.Errorf("cache: delete failed: %w", err)
	}
	return nil
}

// Exists checks if a key exists.
func (dc *DragonflyCache) Exists(ctx context.Context, key string) (bool, error) {
	result, err := dc.client.Exists(ctx, dc.prefixKey(key)).Result()
	if err != nil {
		return false, fmt.Errorf("cache: exists check failed: %w", err)
	}
	return result > 0, nil
}

// Clear removes all keys with the configured prefix.
func (dc *DragonflyCache) Clear(ctx context.Context) error {
	if dc.config.Prefix == "" {
		// Without a prefix, we'd clear everything - require explicit confirmation
		return fmt.Errorf("cache: clear without prefix is not allowed, use FLUSHDB directly if needed")
	}

	// Use SCAN to find and delete keys with prefix
	pattern := dc.config.Prefix + ":*"
	iter := dc.client.Scan(ctx, 0, pattern, 100).Iterator()

	for iter.Next(ctx) {
		if err := dc.client.Del(ctx, iter.Val()).Err(); err != nil {
			return fmt.Errorf("cache: failed to delete key %s: %w", iter.Val(), err)
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("cache: scan failed: %w", err)
	}

	return nil
}

// Close closes the DragonflyDB connection.
func (dc *DragonflyCache) Close() error {
	return dc.client.Close()
}

// Stats returns cache statistics.
func (dc *DragonflyCache) Stats(ctx context.Context) (CacheStats, error) {
	info, err := dc.client.Info(ctx, "stats", "memory", "keyspace").Result()
	if err != nil {
		return CacheStats{}, fmt.Errorf("cache: failed to get stats: %w", err)
	}

	// Parse basic stats (simplified)
	stats := CacheStats{}
	// In production, parse the INFO response properly
	_ = info

	dbSize, err := dc.client.DBSize(ctx).Result()
	if err == nil {
		stats.KeyCount = dbSize
	}

	return stats, nil
}

// SetNX sets a key only if it doesn't exist (useful for locking).
func (dc *DragonflyCache) SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	result, err := dc.client.SetNX(ctx, dc.prefixKey(key), value, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("cache: setnx failed: %w", err)
	}
	return result, nil
}

// Incr atomically increments a counter.
func (dc *DragonflyCache) Incr(ctx context.Context, key string) (int64, error) {
	result, err := dc.client.Incr(ctx, dc.prefixKey(key)).Result()
	if err != nil {
		return 0, fmt.Errorf("cache: incr failed: %w", err)
	}
	return result, nil
}

// Expire sets a TTL on an existing key.
func (dc *DragonflyCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	err := dc.client.Expire(ctx, dc.prefixKey(key), ttl).Err()
	if err != nil {
		return fmt.Errorf("cache: expire failed: %w", err)
	}
	return nil
}

// Client returns the underlying Redis client for advanced operations.
func (dc *DragonflyCache) Client() *redis.Client {
	return dc.client
}
