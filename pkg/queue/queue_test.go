// Package queue_test provides comprehensive tests for the queue package.
package queue_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nuulab/goflow/pkg/queue"
)

func TestNewJob(t *testing.T) {
	payload := map[string]string{"task": "process data"}
	job, err := queue.NewJob("data_processing", payload)
	if err != nil {
		t.Fatalf("NewJob failed: %v", err)
	}

	if job.ID == "" {
		t.Error("Expected non-empty job ID")
	}

	if job.Type != "data_processing" {
		t.Errorf("Expected type 'data_processing', got '%s'", job.Type)
	}

	if job.CreatedAt.IsZero() {
		t.Error("Expected non-zero CreatedAt")
	}
}

func TestJob_UnmarshalPayload(t *testing.T) {
	type Payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	original := Payload{Name: "test", Count: 42}
	job, _ := queue.NewJob("test", original)

	var result Payload
	err := job.UnmarshalPayload(&result)
	if err != nil {
		t.Fatalf("UnmarshalPayload failed: %v", err)
	}

	if result.Name != "test" {
		t.Errorf("Expected name 'test', got '%s'", result.Name)
	}
	if result.Count != 42 {
		t.Errorf("Expected count 42, got %d", result.Count)
	}
}

func TestJob_FluentAPI(t *testing.T) {
	job, _ := queue.NewJob("email", map[string]string{"to": "user@example.com"})

	// Test fluent API chaining
	job.WithPriority(10).
		WithMaxRetries(5).
		WithMetadata("source", "api").
		WithMetadata("tenant", "acme")

	if job.Priority != 10 {
		t.Errorf("Expected priority 10, got %d", job.Priority)
	}

	if job.MaxRetries != 5 {
		t.Errorf("Expected max retries 5, got %d", job.MaxRetries)
	}

	if job.Metadata["source"] != "api" {
		t.Error("Expected metadata 'source' = 'api'")
	}

	if job.Metadata["tenant"] != "acme" {
		t.Error("Expected metadata 'tenant' = 'acme'")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := queue.DefaultConfig()

	if config.Address != "localhost:6379" {
		t.Errorf("Expected default address 'localhost:6379', got '%s'", config.Address)
	}

	if config.QueueName != "goflow:jobs" {
		t.Errorf("Expected queue name 'goflow:jobs', got '%s'", config.QueueName)
	}

	if config.MaxRetries != 3 {
		t.Errorf("Expected max retries 3, got %d", config.MaxRetries)
	}
}

// MockQueue implements queue.Queue for testing
type MockQueue struct {
	jobs []*queue.Job
}

func NewMockQueue() *MockQueue {
	return &MockQueue{jobs: make([]*queue.Job, 0)}
}

func (m *MockQueue) Enqueue(ctx context.Context, job *queue.Job) error {
	m.jobs = append(m.jobs, job)
	return nil
}

func (m *MockQueue) Dequeue(ctx context.Context, timeout time.Duration) (*queue.Job, error) {
	if len(m.jobs) == 0 {
		return nil, nil
	}
	job := m.jobs[0]
	m.jobs = m.jobs[1:]
	return job, nil
}

func (m *MockQueue) Peek(ctx context.Context) (*queue.Job, error) {
	if len(m.jobs) == 0 {
		return nil, nil
	}
	return m.jobs[0], nil
}

func (m *MockQueue) Len(ctx context.Context) (int64, error) {
	return int64(len(m.jobs)), nil
}

func (m *MockQueue) Close() error {
	return nil
}

func TestWorker_Handle(t *testing.T) {
	mockQueue := NewMockQueue()
	worker := queue.NewWorker(mockQueue)

	worker.Handle("test_job", func(ctx context.Context, job *queue.Job) error {
		return nil
	})

	// Enqueue a job
	job, _ := queue.NewJob("test_job", map[string]string{"data": "test"})
	mockQueue.Enqueue(context.Background(), job)

	// Process the job manually (worker.Start uses goroutines)
	ctx := context.Background()
	dequeuedJob, _ := mockQueue.Dequeue(ctx, time.Second)
	if dequeuedJob == nil {
		t.Fatal("Expected to dequeue a job")
	}

	if dequeuedJob.Type != "test_job" {
		t.Errorf("Expected job type 'test_job', got '%s'", dequeuedJob.Type)
	}
}

func TestJob_Serialization(t *testing.T) {
	job, _ := queue.NewJob("serialization_test", map[string]int{"value": 123})
	job.WithPriority(5).WithMaxRetries(3)

	// Serialize
	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Deserialize
	var restored queue.Job
	err = json.Unmarshal(data, &restored)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if restored.ID != job.ID {
		t.Error("ID mismatch")
	}
	if restored.Type != job.Type {
		t.Error("Type mismatch")
	}
	if restored.Priority != job.Priority {
		t.Error("Priority mismatch")
	}
}
