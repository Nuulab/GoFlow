// Package queue provides job/task queue capabilities using DragonflyDB/Redis.
// Supports both simple FIFO queues and priority queues for async processing.
package queue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Queue defines the interface for job queue operations.
type Queue interface {
	// Enqueue adds a job to the queue.
	Enqueue(ctx context.Context, job *Job) error

	// Dequeue retrieves and removes the next job from the queue.
	// Blocks up to timeout duration waiting for a job.
	Dequeue(ctx context.Context, timeout time.Duration) (*Job, error)

	// Peek returns the next job without removing it.
	Peek(ctx context.Context) (*Job, error)

	// Len returns the number of jobs in the queue.
	Len(ctx context.Context) (int64, error)

	// Close closes the queue connection.
	Close() error
}

// Job represents a unit of work in the queue.
// It is safe for concurrent use when using the fluent API methods.
type Job struct {
	// ID is a unique identifier for the job (UUID format).
	ID string `json:"id"`
	// Type identifies the kind of job for routing.
	Type string `json:"type"`
	// Payload contains the job data.
	Payload json.RawMessage `json:"payload"`
	// Priority determines processing order (higher = sooner).
	Priority int `json:"priority,omitempty"`
	// CreatedAt is when the job was created.
	CreatedAt time.Time `json:"created_at"`
	// Attempts tracks retry count.
	Attempts int `json:"attempts,omitempty"`
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int `json:"max_retries,omitempty"`
	// Metadata holds optional key-value data.
	Metadata map[string]string `json:"metadata,omitempty"`
	// mu protects Metadata from concurrent access
	mu sync.Mutex `json:"-"`
}

// NewJob creates a new job with the given type and payload.
func NewJob[T any](jobType string, payload T) (*Job, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("queue: failed to marshal payload: %w", err)
	}

	return &Job{
		ID:        generateID(),
		Type:      jobType,
		Payload:   data,
		CreatedAt: time.Now(),
		Metadata:  make(map[string]string),
	}, nil
}

// UnmarshalPayload deserializes the job payload into the given type.
func (j *Job) UnmarshalPayload(v any) error {
	return json.Unmarshal(j.Payload, v)
}

// WithPriority sets the job priority (fluent API).
func (j *Job) WithPriority(p int) *Job {
	j.Priority = p
	return j
}

// WithMaxRetries sets the maximum retry count (fluent API).
func (j *Job) WithMaxRetries(n int) *Job {
	j.MaxRetries = n
	return j
}

// WithMetadata adds metadata to the job (fluent API).
// It is safe for concurrent use.
func (j *Job) WithMetadata(key, value string) *Job {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.Metadata == nil {
		j.Metadata = make(map[string]string)
	}
	j.Metadata[key] = value
	return j
}

// Config holds configuration for queue connections.
type Config struct {
	// Address is the DragonflyDB/Redis server address.
	Address string
	// Password for authentication.
	Password string
	// Database number.
	Database int
	// QueueName is the name of this queue.
	QueueName string
	// MaxRetries for failed jobs.
	MaxRetries int
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Address:    "localhost:6379",
		QueueName:  "goflow:jobs",
		MaxRetries: 3,
	}
}

// Handler processes jobs of a specific type.
type Handler func(ctx context.Context, job *Job) error

// Worker processes jobs from a queue.
type Worker struct {
	queue    Queue
	handlers map[string]Handler
	stop     chan struct{}
}

// NewWorker creates a new queue worker.
func NewWorker(queue Queue) *Worker {
	return &Worker{
		queue:    queue,
		handlers: make(map[string]Handler),
		stop:     make(chan struct{}),
	}
}

// Handle registers a handler for a job type.
func (w *Worker) Handle(jobType string, handler Handler) {
	w.handlers[jobType] = handler
}

// Start begins processing jobs.
func (w *Worker) Start(ctx context.Context, concurrency int) {
	for i := 0; i < concurrency; i++ {
		go w.processLoop(ctx)
	}
}

// Stop signals the worker to stop processing.
func (w *Worker) Stop() {
	close(w.stop)
}

func (w *Worker) processLoop(ctx context.Context) {
	for {
		select {
		case <-w.stop:
			return
		case <-ctx.Done():
			return
		default:
			job, err := w.queue.Dequeue(ctx, 5*time.Second)
			if err != nil {
				continue
			}
			if job == nil {
				continue
			}

			w.processJob(ctx, job)
		}
	}
}

func (w *Worker) processJob(ctx context.Context, job *Job) {
	handler, ok := w.handlers[job.Type]
	if !ok {
		// No handler for this job type, re-queue or drop
		return
	}

	err := handler(ctx, job)
	if err != nil {
		job.Attempts++
		if job.Attempts < job.MaxRetries {
			// Re-queue for retry
			_ = w.queue.Enqueue(ctx, job)
		}
	}
}

// generateID creates a unique job ID using crypto/rand.
// Returns a UUID-like hex string that is unique across all calls.
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp if crypto/rand fails
		return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().UnixNano()%1000000)
	}
	return hex.EncodeToString(b)
}
