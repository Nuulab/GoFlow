// Package queue provides queue sharding for horizontal scaling.
package queue

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// ShardedQueue distributes jobs across multiple queue shards.
type ShardedQueue struct {
	shards    []Queue
	numShards int
	strategy  ShardStrategy
	mu        sync.RWMutex
}

// ShardStrategy determines how to select a shard.
type ShardStrategy int

const (
	// HashShard uses consistent hashing on job ID.
	HashShard ShardStrategy = iota
	// RoundRobinShard distributes evenly.
	RoundRobinShard
	// LeastLoadedShard picks the shard with fewest jobs.
	LeastLoadedShard
)

// ShardedConfig configures the sharded queue.
type ShardedConfig struct {
	Shards   []Config      // Config for each shard
	Strategy ShardStrategy
}

// NewShardedQueue creates a sharded queue.
func NewShardedQueue(cfg ShardedConfig) (*ShardedQueue, error) {
	if len(cfg.Shards) == 0 {
		return nil, fmt.Errorf("at least one shard required")
	}

	shards := make([]Queue, len(cfg.Shards))
	for i, shardCfg := range cfg.Shards {
		q, err := NewDragonflyQueue(shardCfg)
		if err != nil {
			// Close already created shards
			for j := 0; j < i; j++ {
				shards[j].Close()
			}
			return nil, fmt.Errorf("failed to create shard %d: %w", i, err)
		}
		shards[i] = q
	}

	return &ShardedQueue{
		shards:    shards,
		numShards: len(shards),
		strategy:  cfg.Strategy,
	}, nil
}

// Enqueue adds a job to a shard.
func (sq *ShardedQueue) Enqueue(ctx context.Context, job *Job) error {
	shard := sq.selectShard(ctx, job)
	return sq.shards[shard].Enqueue(ctx, job)
}

// Dequeue retrieves a job from any shard.
func (sq *ShardedQueue) Dequeue(ctx context.Context, timeout time.Duration) (*Job, error) {
	// Try each shard with a fraction of the timeout
	perShardTimeout := timeout / time.Duration(sq.numShards)
	if perShardTimeout < 100*time.Millisecond {
		perShardTimeout = 100 * time.Millisecond
	}

	for _, shard := range sq.shards {
		job, err := shard.Dequeue(ctx, perShardTimeout)
		if err == nil && job != nil {
			return job, nil
		}
	}

	return nil, nil
}

// DequeueFromShard retrieves a job from a specific shard.
func (sq *ShardedQueue) DequeueFromShard(ctx context.Context, shardID int, timeout time.Duration) (*Job, error) {
	if shardID < 0 || shardID >= sq.numShards {
		return nil, fmt.Errorf("invalid shard ID: %d", shardID)
	}
	return sq.shards[shardID].Dequeue(ctx, timeout)
}

// Peek returns the next job without removing it.
func (sq *ShardedQueue) Peek(ctx context.Context) (*Job, error) {
	for _, shard := range sq.shards {
		job, err := shard.Peek(ctx)
		if err == nil && job != nil {
			return job, nil
		}
	}
	return nil, nil
}

// Len returns total jobs across all shards.
func (sq *ShardedQueue) Len(ctx context.Context) (int64, error) {
	var total int64
	for _, shard := range sq.shards {
		count, err := shard.Len(ctx)
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

// LenPerShard returns job count per shard.
func (sq *ShardedQueue) LenPerShard(ctx context.Context) ([]int64, error) {
	counts := make([]int64, sq.numShards)
	for i, shard := range sq.shards {
		count, err := shard.Len(ctx)
		if err != nil {
			return nil, err
		}
		counts[i] = count
	}
	return counts, nil
}

// Close closes all shards.
func (sq *ShardedQueue) Close() error {
	var lastErr error
	for _, shard := range sq.shards {
		if err := shard.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// NumShards returns the number of shards.
func (sq *ShardedQueue) NumShards() int {
	return sq.numShards
}

func (sq *ShardedQueue) selectShard(ctx context.Context, job *Job) int {
	switch sq.strategy {
	case HashShard:
		return sq.hashShard(job.ID)
	case RoundRobinShard:
		return sq.roundRobinShard()
	case LeastLoadedShard:
		return sq.leastLoadedShard(ctx)
	default:
		return sq.hashShard(job.ID)
	}
}

var roundRobinCounter uint64

func (sq *ShardedQueue) hashShard(id string) int {
	h := sha256.Sum256([]byte(id))
	num := binary.BigEndian.Uint64(h[:8])
	return int(num % uint64(sq.numShards))
}

func (sq *ShardedQueue) roundRobinShard() int {
	sq.mu.Lock()
	defer sq.mu.Unlock()
	roundRobinCounter++
	return int(roundRobinCounter % uint64(sq.numShards))
}

func (sq *ShardedQueue) leastLoadedShard(ctx context.Context) int {
	counts, err := sq.LenPerShard(ctx)
	if err != nil {
		return 0
	}

	minShard := 0
	minCount := counts[0]
	for i, count := range counts {
		if count < minCount {
			minCount = count
			minShard = i
		}
	}
	return minShard
}

// PartitionedWorker processes jobs from a specific shard.
type PartitionedWorker struct {
	queue     *ShardedQueue
	shardID   int
	handlers  map[string]Handler
	stop      chan struct{}
}

// NewPartitionedWorker creates a worker for a specific shard.
func NewPartitionedWorker(queue *ShardedQueue, shardID int) *PartitionedWorker {
	return &PartitionedWorker{
		queue:    queue,
		shardID:  shardID,
		handlers: make(map[string]Handler),
		stop:     make(chan struct{}),
	}
}

// Handle registers a handler.
func (pw *PartitionedWorker) Handle(jobType string, handler Handler) {
	pw.handlers[jobType] = handler
}

// Start begins processing from the assigned shard.
func (pw *PartitionedWorker) Start(ctx context.Context, concurrency int) {
	for i := 0; i < concurrency; i++ {
		go pw.processLoop(ctx)
	}
}

// Stop stops the worker.
func (pw *PartitionedWorker) Stop() {
	close(pw.stop)
}

func (pw *PartitionedWorker) processLoop(ctx context.Context) {
	for {
		select {
		case <-pw.stop:
			return
		case <-ctx.Done():
			return
		default:
			job, err := pw.queue.DequeueFromShard(ctx, pw.shardID, 5*time.Second)
			if err != nil || job == nil {
				continue
			}

			handler, ok := pw.handlers[job.Type]
			if ok {
				handler(ctx, job)
			}
		}
	}
}
