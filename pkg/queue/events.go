// Package queue provides event sourcing for job history.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// EventType identifies the type of job event.
type EventType string

const (
	EventJobCreated   EventType = "job.created"
	EventJobQueued    EventType = "job.queued"
	EventJobStarted   EventType = "job.started"
	EventJobCompleted EventType = "job.completed"
	EventJobFailed    EventType = "job.failed"
	EventJobRetried   EventType = "job.retried"
	EventJobDLQ       EventType = "job.dlq"
)

// Event represents a job lifecycle event.
type Event struct {
	ID        string            `json:"id"`
	Type      EventType         `json:"type"`
	JobID     string            `json:"job_id"`
	JobType   string            `json:"job_type"`
	Timestamp time.Time         `json:"timestamp"`
	Data      map[string]any    `json:"data,omitempty"`
	WorkerID  string            `json:"worker_id,omitempty"`
	Error     string            `json:"error,omitempty"`
	Duration  time.Duration     `json:"duration,omitempty"`
}

// EventStore provides append-only event storage.
type EventStore struct {
	client    *redis.Client
	keyPrefix string
	maxEvents int64
}

// NewEventStore creates an event store.
func NewEventStore(client *redis.Client) *EventStore {
	return &EventStore{
		client:    client,
		keyPrefix: "goflow:events",
		maxEvents: 100000, // Keep last 100k events per stream
	}
}

// Append adds an event to the store.
func (es *EventStore) Append(ctx context.Context, event Event) error {
	if event.ID == "" {
		event.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Add to global event stream
	globalKey := es.keyPrefix + ":all"
	if err := es.client.XAdd(ctx, &redis.XAddArgs{
		Stream: globalKey,
		MaxLen: es.maxEvents,
		Values: map[string]any{"data": data},
	}).Err(); err != nil {
		return err
	}

	// Add to job-specific stream
	jobKey := fmt.Sprintf("%s:job:%s", es.keyPrefix, event.JobID)
	return es.client.XAdd(ctx, &redis.XAddArgs{
		Stream: jobKey,
		MaxLen: 1000, // Keep last 1000 events per job
		Values: map[string]any{"data": data},
	}).Err()
}

// GetJobEvents returns events for a specific job.
func (es *EventStore) GetJobEvents(ctx context.Context, jobID string) ([]Event, error) {
	key := fmt.Sprintf("%s:job:%s", es.keyPrefix, jobID)
	
	messages, err := es.client.XRange(ctx, key, "-", "+").Result()
	if err != nil {
		return nil, err
	}

	events := make([]Event, 0, len(messages))
	for _, msg := range messages {
		data, ok := msg.Values["data"].(string)
		if !ok {
			continue
		}

		var event Event
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		events = append(events, event)
	}

	return events, nil
}

// GetRecentEvents returns the most recent events.
func (es *EventStore) GetRecentEvents(ctx context.Context, count int64) ([]Event, error) {
	key := es.keyPrefix + ":all"

	messages, err := es.client.XRevRange(ctx, key, "+", "-").Result()
	if err != nil {
		return nil, err
	}

	if int64(len(messages)) > count {
		messages = messages[:count]
	}

	events := make([]Event, 0, len(messages))
	for _, msg := range messages {
		data, ok := msg.Values["data"].(string)
		if !ok {
			continue
		}

		var event Event
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		events = append(events, event)
	}

	// Reverse to chronological order
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}

	return events, nil
}

// GetEventsByType returns events of a specific type.
func (es *EventStore) GetEventsByType(ctx context.Context, eventType EventType, since time.Time, count int64) ([]Event, error) {
	key := es.keyPrefix + ":all"

	start := "-"
	if !since.IsZero() {
		start = fmt.Sprintf("%d-0", since.UnixMilli())
	}

	messages, err := es.client.XRange(ctx, key, start, "+").Result()
	if err != nil {
		return nil, err
	}

	events := make([]Event, 0)
	for _, msg := range messages {
		if int64(len(events)) >= count {
			break
		}

		data, ok := msg.Values["data"].(string)
		if !ok {
			continue
		}

		var event Event
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		if event.Type == eventType {
			events = append(events, event)
		}
	}

	return events, nil
}

// Subscribe subscribes to real-time events.
func (es *EventStore) Subscribe(ctx context.Context, handler func(Event)) error {
	key := es.keyPrefix + ":all"
	lastID := "$" // Only new events

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		streams, err := es.client.XRead(ctx, &redis.XReadArgs{
			Streams: []string{key, lastID},
			Block:   5 * time.Second,
			Count:   100,
		}).Result()

		if err != nil {
			if err == redis.Nil {
				continue
			}
			return err
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				lastID = msg.ID

				data, ok := msg.Values["data"].(string)
				if !ok {
					continue
				}

				var event Event
				if err := json.Unmarshal([]byte(data), &event); err != nil {
					continue
				}

				handler(event)
			}
		}
	}
}

// EventRecorder wraps a queue to record events.
type EventRecorder struct {
	queue    Queue
	store    *EventStore
	workerID string
}

// NewEventRecorder creates an event-recording queue wrapper.
func NewEventRecorder(queue Queue, store *EventStore, workerID string) *EventRecorder {
	return &EventRecorder{
		queue:    queue,
		store:    store,
		workerID: workerID,
	}
}

// Enqueue adds a job and records the event.
func (er *EventRecorder) Enqueue(ctx context.Context, job *Job) error {
	// Record creation
	er.store.Append(ctx, Event{
		Type:      EventJobQueued,
		JobID:     job.ID,
		JobType:   job.Type,
		Timestamp: time.Now(),
		WorkerID:  er.workerID,
	})

	return er.queue.Enqueue(ctx, job)
}

// Dequeue retrieves a job and records the start.
func (er *EventRecorder) Dequeue(ctx context.Context, timeout time.Duration) (*Job, error) {
	job, err := er.queue.Dequeue(ctx, timeout)
	if err != nil || job == nil {
		return job, err
	}

	er.store.Append(ctx, Event{
		Type:      EventJobStarted,
		JobID:     job.ID,
		JobType:   job.Type,
		Timestamp: time.Now(),
		WorkerID:  er.workerID,
	})

	return job, nil
}

// RecordComplete records job completion.
func (er *EventRecorder) RecordComplete(ctx context.Context, job *Job, duration time.Duration) {
	er.store.Append(ctx, Event{
		Type:      EventJobCompleted,
		JobID:     job.ID,
		JobType:   job.Type,
		Timestamp: time.Now(),
		WorkerID:  er.workerID,
		Duration:  duration,
	})
}

// RecordFailed records job failure.
func (er *EventRecorder) RecordFailed(ctx context.Context, job *Job, err error, duration time.Duration) {
	er.store.Append(ctx, Event{
		Type:      EventJobFailed,
		JobID:     job.ID,
		JobType:   job.Type,
		Timestamp: time.Now(),
		WorkerID:  er.workerID,
		Error:     err.Error(),
		Duration:  duration,
	})
}

// Peek, Len, Close delegate to underlying queue
func (er *EventRecorder) Peek(ctx context.Context) (*Job, error) { return er.queue.Peek(ctx) }
func (er *EventRecorder) Len(ctx context.Context) (int64, error) { return er.queue.Len(ctx) }
func (er *EventRecorder) Close() error { return er.queue.Close() }
