// Package queue provides advanced queue features.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// AdvancedQueue extends DragonflyQueue with advanced features.
type AdvancedQueue struct {
	*DragonflyQueue
	rateLimiter *RateLimiter
	scheduler   *Scheduler
	deps        *DependencyManager
}

// NewAdvancedQueue creates a queue with advanced features.
func NewAdvancedQueue(cfg Config) (*AdvancedQueue, error) {
	base, err := NewDragonflyQueue(cfg)
	if err != nil {
		return nil, err
	}
	
	return &AdvancedQueue{
		DragonflyQueue: base,
		rateLimiter:    NewRateLimiter(base.client),
		scheduler:      NewScheduler(base),
		deps:           NewDependencyManager(base.client),
	}, nil
}

// EnqueueDelayed adds a job to be processed after a delay.
func (aq *AdvancedQueue) EnqueueDelayed(ctx context.Context, job *Job, delay time.Duration) error {
	return aq.scheduler.ScheduleAfter(ctx, job, delay)
}

// EnqueueAt adds a job to be processed at a specific time.
func (aq *AdvancedQueue) EnqueueAt(ctx context.Context, job *Job, at time.Time) error {
	return aq.scheduler.ScheduleAt(ctx, job, at)
}

// EnqueueWithDependency adds a job that depends on another job.
func (aq *AdvancedQueue) EnqueueWithDependency(ctx context.Context, job *Job, dependsOn string) error {
	return aq.deps.AddWithDependency(ctx, job, dependsOn)
}

// RateLimiter controls job processing rate.
type RateLimiter struct {
	client *redis.Client
	mu     sync.RWMutex
	limits map[string]*Limit
}

// Limit defines a rate limit.
type Limit struct {
	MaxRequests int
	Window      time.Duration
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(client *redis.Client) *RateLimiter {
	return &RateLimiter{
		client: client,
		limits: make(map[string]*Limit),
	}
}

// SetLimit sets a rate limit for a job type.
func (rl *RateLimiter) SetLimit(jobType string, maxRequests int, window time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.limits[jobType] = &Limit{
		MaxRequests: maxRequests,
		Window:      window,
	}
}

// Allow checks if a job can be processed within rate limits.
func (rl *RateLimiter) Allow(ctx context.Context, jobType string) (bool, error) {
	rl.mu.RLock()
	limit, ok := rl.limits[jobType]
	rl.mu.RUnlock()
	
	if !ok {
		return true, nil // No limit configured
	}
	
	key := fmt.Sprintf("ratelimit:%s", jobType)
	
	// Use Redis INCR with expiry for sliding window
	count, err := rl.client.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	
	if count == 1 {
		// First request in window, set expiry
		rl.client.Expire(ctx, key, limit.Window)
	}
	
	return count <= int64(limit.MaxRequests), nil
}

// WaitForSlot blocks until a slot is available.
func (rl *RateLimiter) WaitForSlot(ctx context.Context, jobType string) error {
	for {
		allowed, err := rl.Allow(ctx, jobType)
		if err != nil {
			return err
		}
		if allowed {
			return nil
		}
		
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}
}

// Scheduler handles delayed and scheduled jobs.
type Scheduler struct {
	queue     *DragonflyQueue
	schedKey  string
	pollInterval time.Duration
	stop      chan struct{}
}

// NewScheduler creates a job scheduler.
func NewScheduler(queue *DragonflyQueue) *Scheduler {
	return &Scheduler{
		queue:        queue,
		schedKey:     queue.queueKey + ":scheduled",
		pollInterval: 1 * time.Second,
		stop:         make(chan struct{}),
	}
}

// ScheduleAfter schedules a job after a delay.
func (s *Scheduler) ScheduleAfter(ctx context.Context, job *Job, delay time.Duration) error {
	return s.ScheduleAt(ctx, job, time.Now().Add(delay))
}

// ScheduleAt schedules a job at a specific time.
func (s *Scheduler) ScheduleAt(ctx context.Context, job *Job, at time.Time) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	
	// Use sorted set with score = unix timestamp
	return s.queue.client.ZAdd(ctx, s.schedKey, redis.Z{
		Score:  float64(at.Unix()),
		Member: data,
	}).Err()
}

// Start begins the scheduler loop.
func (s *Scheduler) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.pollInterval)
		defer ticker.Stop()
		
		for {
			select {
			case <-s.stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.processScheduled(ctx)
			}
		}
	}()
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() {
	close(s.stop)
}

func (s *Scheduler) processScheduled(ctx context.Context) {
	now := float64(time.Now().Unix())
	
	// Get jobs that are due
	results, err := s.queue.client.ZRangeByScoreWithScores(ctx, s.schedKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%f", now),
	}).Result()
	
	if err != nil {
		return
	}
	
	for _, z := range results {
		data, ok := z.Member.(string)
		if !ok {
			continue
		}
		
		var job Job
		if err := json.Unmarshal([]byte(data), &job); err != nil {
			continue
		}
		
		// Move to main queue
		if err := s.queue.Enqueue(ctx, &job); err == nil {
			// Remove from scheduled set
			s.queue.client.ZRem(ctx, s.schedKey, z.Member)
		}
	}
}

// DependencyManager handles job dependencies.
type DependencyManager struct {
	client  *redis.Client
	depsKey string
	pending map[string]*Job
	mu      sync.RWMutex
}

// NewDependencyManager creates a dependency manager.
func NewDependencyManager(client *redis.Client) *DependencyManager {
	return &DependencyManager{
		client:  client,
		depsKey: "goflow:deps",
		pending: make(map[string]*Job),
	}
}

// AddWithDependency adds a job that depends on another.
func (dm *DependencyManager) AddWithDependency(ctx context.Context, job *Job, dependsOn string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	// Store dependency relationship
	depKey := fmt.Sprintf("%s:%s", dm.depsKey, dependsOn)
	
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	
	// Add to waiting list for this dependency
	return dm.client.LPush(ctx, depKey, data).Err()
}

// Complete marks a job as complete and triggers dependents.
func (dm *DependencyManager) Complete(ctx context.Context, jobID string, queue Queue) error {
	depKey := fmt.Sprintf("%s:%s", dm.depsKey, jobID)
	
	// Get all dependent jobs
	for {
		data, err := dm.client.RPop(ctx, depKey).Bytes()
		if err != nil {
			break // No more dependents
		}
		
		var job Job
		if err := json.Unmarshal(data, &job); err != nil {
			continue
		}
		
		// Enqueue the dependent job
		if err := queue.Enqueue(ctx, &job); err != nil {
			// Put it back if enqueue fails
			dm.client.LPush(ctx, depKey, data)
			return err
		}
	}
	
	return nil
}

// GetPendingDependents returns jobs waiting on a dependency.
func (dm *DependencyManager) GetPendingDependents(ctx context.Context, jobID string) ([]*Job, error) {
	depKey := fmt.Sprintf("%s:%s", dm.depsKey, jobID)
	
	results, err := dm.client.LRange(ctx, depKey, 0, -1).Result()
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
