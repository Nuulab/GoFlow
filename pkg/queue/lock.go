// Package queue provides distributed locking for job deduplication.
package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrLockNotAcquired is returned when a lock cannot be obtained.
var ErrLockNotAcquired = errors.New("lock not acquired")

// DistributedLock provides distributed locking using Redis/DragonflyDB.
type DistributedLock struct {
	client    *redis.Client
	keyPrefix string
}

// Lock represents a held lock.
type Lock struct {
	dl       *DistributedLock
	key      string
	value    string
	ttl      time.Duration
	released bool
}

// NewDistributedLock creates a distributed lock manager.
func NewDistributedLock(client *redis.Client) *DistributedLock {
	return &DistributedLock{
		client:    client,
		keyPrefix: "goflow:lock:",
	}
}

// Acquire attempts to acquire a lock.
func (dl *DistributedLock) Acquire(ctx context.Context, key string, ttl time.Duration) (*Lock, error) {
	lockKey := dl.keyPrefix + key
	value := fmt.Sprintf("%d", time.Now().UnixNano())

	// Use SET NX (only set if not exists)
	ok, err := dl.client.SetNX(ctx, lockKey, value, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("lock acquire failed: %w", err)
	}

	if !ok {
		return nil, ErrLockNotAcquired
	}

	return &Lock{
		dl:    dl,
		key:   lockKey,
		value: value,
		ttl:   ttl,
	}, nil
}

// TryAcquire attempts to acquire a lock with retries.
func (dl *DistributedLock) TryAcquire(ctx context.Context, key string, ttl time.Duration, maxWait time.Duration) (*Lock, error) {
	deadline := time.Now().Add(maxWait)
	backoff := 10 * time.Millisecond

	for time.Now().Before(deadline) {
		lock, err := dl.Acquire(ctx, key, ttl)
		if err == nil {
			return lock, nil
		}

		if !errors.Is(err, ErrLockNotAcquired) {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
			if backoff > 1*time.Second {
				backoff = 1 * time.Second
			}
		}
	}

	return nil, ErrLockNotAcquired
}

// Release releases the lock.
func (l *Lock) Release(ctx context.Context) error {
	if l.released {
		return nil
	}

	// Use Lua script to ensure we only delete our own lock
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`)

	_, err := script.Run(ctx, l.dl.client, []string{l.key}, l.value).Result()
	if err != nil {
		return fmt.Errorf("lock release failed: %w", err)
	}

	l.released = true
	return nil
}

// Extend extends the lock TTL.
func (l *Lock) Extend(ctx context.Context, ttl time.Duration) error {
	// Use Lua script to extend only our own lock
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("pexpire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)

	result, err := script.Run(ctx, l.dl.client, []string{l.key}, l.value, ttl.Milliseconds()).Int()
	if err != nil {
		return fmt.Errorf("lock extend failed: %w", err)
	}

	if result == 0 {
		return errors.New("lock no longer held")
	}

	l.ttl = ttl
	return nil
}

// IsHeld checks if the lock is still held.
func (l *Lock) IsHeld(ctx context.Context) (bool, error) {
	val, err := l.dl.client.Get(ctx, l.key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return val == l.value, nil
}

// WithLock executes a function while holding a lock.
func (dl *DistributedLock) WithLock(ctx context.Context, key string, ttl time.Duration, fn func() error) error {
	lock, err := dl.Acquire(ctx, key, ttl)
	if err != nil {
		return err
	}
	defer lock.Release(ctx)

	return fn()
}

// JobLocker provides job-specific locking.
type JobLocker struct {
	dl *DistributedLock
}

// NewJobLocker creates a job locker.
func NewJobLocker(client *redis.Client) *JobLocker {
	return &JobLocker{
		dl: NewDistributedLock(client),
	}
}

// LockJob locks a job for processing.
func (jl *JobLocker) LockJob(ctx context.Context, jobID string, ttl time.Duration) (*Lock, error) {
	return jl.dl.Acquire(ctx, "job:"+jobID, ttl)
}

// IsJobLocked checks if a job is locked.
func (jl *JobLocker) IsJobLocked(ctx context.Context, jobID string) (bool, error) {
	key := jl.dl.keyPrefix + "job:" + jobID
	exists, err := jl.dl.client.Exists(ctx, key).Result()
	return exists > 0, err
}

// Semaphore provides distributed counting semaphore.
type Semaphore struct {
	client *redis.Client
	key    string
	limit  int
}

// NewSemaphore creates a distributed semaphore.
func NewSemaphore(client *redis.Client, key string, limit int) *Semaphore {
	return &Semaphore{
		client: client,
		key:    "goflow:sem:" + key,
		limit:  limit,
	}
}

// Acquire acquires a semaphore slot.
func (s *Semaphore) Acquire(ctx context.Context, ttl time.Duration) (string, error) {
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	score := float64(time.Now().Add(ttl).UnixNano())

	// Clean up expired entries
	s.client.ZRemRangeByScore(ctx, s.key, "-inf", fmt.Sprintf("%d", time.Now().UnixNano()))

	// Check if we can acquire
	count, err := s.client.ZCard(ctx, s.key).Result()
	if err != nil {
		return "", err
	}

	if count >= int64(s.limit) {
		return "", ErrLockNotAcquired
	}

	// Add our entry
	err = s.client.ZAdd(ctx, s.key, redis.Z{Score: score, Member: id}).Err()
	if err != nil {
		return "", err
	}

	return id, nil
}

// Release releases a semaphore slot.
func (s *Semaphore) Release(ctx context.Context, id string) error {
	return s.client.ZRem(ctx, s.key, id).Err()
}

// Available returns the number of available slots.
func (s *Semaphore) Available(ctx context.Context) (int, error) {
	// Clean up expired
	s.client.ZRemRangeByScore(ctx, s.key, "-inf", fmt.Sprintf("%d", time.Now().UnixNano()))

	count, err := s.client.ZCard(ctx, s.key).Result()
	if err != nil {
		return 0, err
	}

	available := s.limit - int(count)
	if available < 0 {
		available = 0
	}
	return available, nil
}
