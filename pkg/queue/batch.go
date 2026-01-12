// Package queue provides batch processing and progress tracking.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// BatchProcessor processes jobs in batches for efficiency.
type BatchProcessor struct {
	queue       Queue
	client      *redis.Client
	handlers    map[string]BatchHandler
	batchSizes  map[string]int
	batchTimeouts map[string]time.Duration
	mu          sync.RWMutex
}

// BatchHandler processes a batch of jobs.
type BatchHandler func(ctx context.Context, jobs []*Job) []error

// NewBatchProcessor creates a new batch processor.
func NewBatchProcessor(queue Queue, client *redis.Client) *BatchProcessor {
	return &BatchProcessor{
		queue:         queue,
		client:        client,
		handlers:      make(map[string]BatchHandler),
		batchSizes:    make(map[string]int),
		batchTimeouts: make(map[string]time.Duration),
	}
}

// HandleBatch registers a batch handler for a job type.
func (bp *BatchProcessor) HandleBatch(jobType string, batchSize int, handler BatchHandler) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.handlers[jobType] = handler
	bp.batchSizes[jobType] = batchSize
	bp.batchTimeouts[jobType] = 5 * time.Second // Default timeout
}

// WithTimeout sets the batch collection timeout.
func (bp *BatchProcessor) WithTimeout(jobType string, timeout time.Duration) *BatchProcessor {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.batchTimeouts[jobType] = timeout
	return bp
}

// Start begins batch processing.
func (bp *BatchProcessor) Start(ctx context.Context) {
	bp.mu.RLock()
	handlers := make(map[string]BatchHandler)
	for k, v := range bp.handlers {
		handlers[k] = v
	}
	bp.mu.RUnlock()
	
	for jobType := range handlers {
		go bp.processBatches(ctx, jobType)
	}
}

func (bp *BatchProcessor) processBatches(ctx context.Context, jobType string) {
	bp.mu.RLock()
	batchSize := bp.batchSizes[jobType]
	timeout := bp.batchTimeouts[jobType]
	handler := bp.handlers[jobType]
	bp.mu.RUnlock()
	
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		
		// Collect a batch
		batch := make([]*Job, 0, batchSize)
		deadline := time.Now().Add(timeout)
		
		for len(batch) < batchSize && time.Now().Before(deadline) {
			job, err := bp.queue.Dequeue(ctx, 1*time.Second)
			if err != nil || job == nil {
				continue
			}
			if job.Type == jobType {
				batch = append(batch, job)
			}
		}
		
		if len(batch) == 0 {
			continue
		}
		
		// Process the batch
		errors := handler(ctx, batch)
		
		// Handle failures
		for i, err := range errors {
			if err != nil && i < len(batch) {
				job := batch[i]
				job.Attempts++
				if job.Attempts < job.MaxRetries {
					bp.queue.Enqueue(ctx, job)
				}
			}
		}
	}
}

// ProgressTracker tracks job progress.
type ProgressTracker struct {
	client    *redis.Client
	keyPrefix string
}

// Progress represents job progress.
type Progress struct {
	JobID      string    `json:"job_id"`
	Percent    int       `json:"percent"`
	Message    string    `json:"message"`
	StartedAt  time.Time `json:"started_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	EstimatedEnd time.Time `json:"estimated_end,omitempty"`
}

// NewProgressTracker creates a progress tracker.
func NewProgressTracker(client *redis.Client) *ProgressTracker {
	return &ProgressTracker{
		client:    client,
		keyPrefix: "goflow:progress",
	}
}

// Start marks a job as started.
func (pt *ProgressTracker) Start(ctx context.Context, jobID string) error {
	progress := Progress{
		JobID:     jobID,
		Percent:   0,
		Message:   "Started",
		StartedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return pt.save(ctx, progress)
}

// Update updates job progress.
func (pt *ProgressTracker) Update(ctx context.Context, jobID string, percent int, message string) error {
	key := fmt.Sprintf("%s:%s", pt.keyPrefix, jobID)
	
	// Get existing progress
	data, err := pt.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	
	var progress Progress
	if err := json.Unmarshal(data, &progress); err != nil {
		return err
	}
	
	progress.Percent = percent
	progress.Message = message
	progress.UpdatedAt = time.Now()
	
	// Estimate completion time
	if percent > 0 {
		elapsed := time.Since(progress.StartedAt)
		estimated := time.Duration(float64(elapsed) / float64(percent) * 100)
		progress.EstimatedEnd = progress.StartedAt.Add(estimated)
	}
	
	return pt.save(ctx, progress)
}

// Get retrieves current progress.
func (pt *ProgressTracker) Get(ctx context.Context, jobID string) (*Progress, error) {
	key := fmt.Sprintf("%s:%s", pt.keyPrefix, jobID)
	
	data, err := pt.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	
	var progress Progress
	if err := json.Unmarshal(data, &progress); err != nil {
		return nil, err
	}
	
	return &progress, nil
}

// Complete marks a job as complete.
func (pt *ProgressTracker) Complete(ctx context.Context, jobID string) error {
	return pt.Update(ctx, jobID, 100, "Complete")
}

// Fail marks a job as failed.
func (pt *ProgressTracker) Fail(ctx context.Context, jobID string, errorMsg string) error {
	key := fmt.Sprintf("%s:%s", pt.keyPrefix, jobID)
	
	data, _ := pt.client.Get(ctx, key).Bytes()
	
	var progress Progress
	json.Unmarshal(data, &progress)
	
	progress.Message = "Failed: " + errorMsg
	progress.UpdatedAt = time.Now()
	
	return pt.save(ctx, progress)
}

func (pt *ProgressTracker) save(ctx context.Context, progress Progress) error {
	key := fmt.Sprintf("%s:%s", pt.keyPrefix, progress.JobID)
	
	data, err := json.Marshal(progress)
	if err != nil {
		return err
	}
	
	// Keep progress for 24 hours
	return pt.client.Set(ctx, key, data, 24*time.Hour).Err()
}

// ProgressJob wraps a job with progress tracking.
type ProgressJob struct {
	*Job
	tracker *ProgressTracker
}

// NewProgressJob creates a job with progress tracking.
func NewProgressJob(job *Job, tracker *ProgressTracker) *ProgressJob {
	return &ProgressJob{
		Job:     job,
		tracker: tracker,
	}
}

// UpdateProgress updates this job's progress.
func (pj *ProgressJob) UpdateProgress(ctx context.Context, percent int, message string) error {
	return pj.tracker.Update(ctx, pj.ID, percent, message)
}
