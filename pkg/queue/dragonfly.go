// Package queue provides a DragonflyDB/Redis queue implementation.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// DragonflyQueue implements Queue using DragonflyDB (Redis-compatible).
type DragonflyQueue struct {
	client    *redis.Client
	config    Config
	queueKey  string
	dlqKey    string // Dead Letter Queue
}

// NewDragonflyQueue creates a new DragonflyDB queue.
func NewDragonflyQueue(cfg Config) (*DragonflyQueue, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.Database,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("queue: failed to connect to DragonflyDB: %w", err)
	}

	return &DragonflyQueue{
		client:   client,
		config:   cfg,
		queueKey: cfg.QueueName,
		dlqKey:   cfg.QueueName + ":dlq",
	}, nil
}

// Enqueue adds a job to the queue using LPUSH.
func (dq *DragonflyQueue) Enqueue(ctx context.Context, job *Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("queue: failed to marshal job: %w", err)
	}

	if job.Priority > 0 {
		// Use sorted set for priority queue
		return dq.enqueuePriority(ctx, job, data)
	}

	// Standard FIFO queue
	if err := dq.client.LPush(ctx, dq.queueKey, data).Err(); err != nil {
		return fmt.Errorf("queue: enqueue failed: %w", err)
	}

	return nil
}

// enqueuePriority adds a job to the priority queue.
func (dq *DragonflyQueue) enqueuePriority(ctx context.Context, job *Job, data []byte) error {
	// Use score = priority (higher priority = higher score = dequeued first)
	member := redis.Z{
		Score:  float64(job.Priority),
		Member: data,
	}

	if err := dq.client.ZAdd(ctx, dq.queueKey+":priority", member).Err(); err != nil {
		return fmt.Errorf("queue: priority enqueue failed: %w", err)
	}

	return nil
}

// Dequeue retrieves and removes the next job using BRPOP.
func (dq *DragonflyQueue) Dequeue(ctx context.Context, timeout time.Duration) (*Job, error) {
	// First check priority queue
	job, err := dq.dequeuePriority(ctx)
	if err == nil && job != nil {
		return job, nil
	}

	// Fall back to standard queue with blocking pop
	result, err := dq.client.BRPop(ctx, timeout, dq.queueKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // No job available
		}
		return nil, fmt.Errorf("queue: dequeue failed: %w", err)
	}

	if len(result) < 2 {
		return nil, nil
	}

	var job2 Job
	if err := json.Unmarshal([]byte(result[1]), &job2); err != nil {
		return nil, fmt.Errorf("queue: failed to unmarshal job: %w", err)
	}

	return &job2, nil
}

// dequeuePriority retrieves from the priority queue.
func (dq *DragonflyQueue) dequeuePriority(ctx context.Context) (*Job, error) {
	// Get highest priority item (highest score)
	result, err := dq.client.ZPopMax(ctx, dq.queueKey+":priority", 1).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	if len(result) == 0 {
		return nil, nil
	}

	var job Job
	data, ok := result[0].Member.(string)
	if !ok {
		return nil, fmt.Errorf("queue: unexpected member type")
	}

	if err := json.Unmarshal([]byte(data), &job); err != nil {
		return nil, err
	}

	return &job, nil
}

// Peek returns the next job without removing it.
func (dq *DragonflyQueue) Peek(ctx context.Context) (*Job, error) {
	// Check priority queue first
	result, err := dq.client.ZRevRange(ctx, dq.queueKey+":priority", 0, 0).Result()
	if err == nil && len(result) > 0 {
		var job Job
		if err := json.Unmarshal([]byte(result[0]), &job); err == nil {
			return &job, nil
		}
	}

	// Check standard queue
	data, err := dq.client.LIndex(ctx, dq.queueKey, -1).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("queue: peek failed: %w", err)
	}

	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("queue: failed to unmarshal job: %w", err)
	}

	return &job, nil
}

// Len returns the total number of jobs in both queues.
func (dq *DragonflyQueue) Len(ctx context.Context) (int64, error) {
	// Count standard queue
	listLen, err := dq.client.LLen(ctx, dq.queueKey).Result()
	if err != nil {
		return 0, err
	}

	// Count priority queue
	zsetLen, err := dq.client.ZCard(ctx, dq.queueKey+":priority").Result()
	if err != nil {
		return listLen, nil // Return list length if zset fails
	}

	return listLen + zsetLen, nil
}

// Close closes the DragonflyDB connection.
func (dq *DragonflyQueue) Close() error {
	return dq.client.Close()
}

// MoveToDLQ moves a failed job to the dead letter queue.
func (dq *DragonflyQueue) MoveToDLQ(ctx context.Context, job *Job, reason string) error {
	job.Metadata["dlq_reason"] = reason
	job.Metadata["dlq_time"] = time.Now().Format(time.RFC3339)

	data, err := json.Marshal(job)
	if err != nil {
		return err
	}

	return dq.client.LPush(ctx, dq.dlqKey, data).Err()
}

// GetDLQJobs retrieves jobs from the dead letter queue.
func (dq *DragonflyQueue) GetDLQJobs(ctx context.Context, count int64) ([]*Job, error) {
	results, err := dq.client.LRange(ctx, dq.dlqKey, 0, count-1).Result()
	if err != nil {
		return nil, err
	}

	jobs := make([]*Job, 0, len(results))
	for _, data := range results {
		var job Job
		if err := json.Unmarshal([]byte(data), &job); err == nil {
			jobs = append(jobs, &job)
		}
	}

	return jobs, nil
}

// Client returns the underlying Redis client.
func (dq *DragonflyQueue) Client() *redis.Client {
	return dq.client
}
