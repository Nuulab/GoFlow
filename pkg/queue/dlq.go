// Package queue provides dead letter queue with alerting.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// DLQ is an enhanced dead letter queue with alerting.
type DLQ struct {
	client    *redis.Client
	key       string
	alerters  []Alerter
	maxSize   int64
}

// DLQEntry is an entry in the dead letter queue.
type DLQEntry struct {
	Job       *Job      `json:"job"`
	Error     string    `json:"error"`
	FailedAt  time.Time `json:"failed_at"`
	Attempts  int       `json:"attempts"`
	WorkerID  string    `json:"worker_id,omitempty"`
	StackTrace string   `json:"stack_trace,omitempty"`
}

// Alerter sends alerts when jobs fail permanently.
type Alerter interface {
	Alert(ctx context.Context, entry DLQEntry) error
}

// NewDLQ creates a dead letter queue.
func NewDLQ(client *redis.Client, name string) *DLQ {
	return &DLQ{
		client:   client,
		key:      "goflow:dlq:" + name,
		alerters: make([]Alerter, 0),
		maxSize:  10000,
	}
}

// AddAlerter adds an alerter.
func (d *DLQ) AddAlerter(alerter Alerter) {
	d.alerters = append(d.alerters, alerter)
}

// Add adds a job to the DLQ.
func (d *DLQ) Add(ctx context.Context, job *Job, err error, workerID string) error {
	entry := DLQEntry{
		Job:      job,
		Error:    err.Error(),
		FailedAt: time.Now(),
		Attempts: job.Attempts,
		WorkerID: workerID,
	}

	data, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		return marshalErr
	}

	// Add to DLQ
	if pushErr := d.client.LPush(ctx, d.key, data).Err(); pushErr != nil {
		return pushErr
	}

	// Trim to max size
	d.client.LTrim(ctx, d.key, 0, d.maxSize-1)

	// Send alerts
	for _, alerter := range d.alerters {
		go alerter.Alert(ctx, entry)
	}

	return nil
}

// Get retrieves entries from the DLQ.
func (d *DLQ) Get(ctx context.Context, start, stop int64) ([]DLQEntry, error) {
	results, err := d.client.LRange(ctx, d.key, start, stop).Result()
	if err != nil {
		return nil, err
	}

	entries := make([]DLQEntry, 0, len(results))
	for _, data := range results {
		var entry DLQEntry
		if err := json.Unmarshal([]byte(data), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// Len returns the number of jobs in the DLQ.
func (d *DLQ) Len(ctx context.Context) (int64, error) {
	return d.client.LLen(ctx, d.key).Result()
}

// Retry moves a job from DLQ back to the main queue.
func (d *DLQ) Retry(ctx context.Context, queue Queue, index int64) error {
	// Get the entry
	data, err := d.client.LIndex(ctx, d.key, index).Result()
	if err != nil {
		return err
	}

	var entry DLQEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		return err
	}

	// Reset attempts
	entry.Job.Attempts = 0

	// Re-queue
	if err := queue.Enqueue(ctx, entry.Job); err != nil {
		return err
	}

	// Remove from DLQ (by value since index might change)
	d.client.LRem(ctx, d.key, 1, data)

	return nil
}

// RetryAll retries all jobs in the DLQ.
func (d *DLQ) RetryAll(ctx context.Context, queue Queue) (int, error) {
	count := 0
	for {
		data, err := d.client.RPop(ctx, d.key).Result()
		if err == redis.Nil {
			break
		}
		if err != nil {
			return count, err
		}

		var entry DLQEntry
		if err := json.Unmarshal([]byte(data), &entry); err != nil {
			continue
		}

		entry.Job.Attempts = 0
		if err := queue.Enqueue(ctx, entry.Job); err != nil {
			// Put it back
			d.client.LPush(ctx, d.key, data)
			return count, err
		}
		count++
	}

	return count, nil
}

// Purge removes all jobs from the DLQ.
func (d *DLQ) Purge(ctx context.Context) error {
	return d.client.Del(ctx, d.key).Err()
}

// ============ Alerters ============

// WebhookAlerter sends alerts via webhook.
type WebhookAlerter struct {
	URL     string
	Headers map[string]string
	client  *http.Client
}

// NewWebhookAlerter creates a webhook alerter.
func NewWebhookAlerter(url string) *WebhookAlerter {
	return &WebhookAlerter{
		URL:     url,
		Headers: make(map[string]string),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Alert sends an alert via webhook.
func (w *WebhookAlerter) Alert(ctx context.Context, entry DLQEntry) error {
	data, err := json.Marshal(map[string]any{
		"type":      "job_failed_permanently",
		"job_id":    entry.Job.ID,
		"job_type":  entry.Job.Type,
		"error":     entry.Error,
		"attempts":  entry.Attempts,
		"failed_at": entry.FailedAt,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", w.URL, strings.NewReader(string(data)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.Headers {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// SlackAlerter sends alerts to Slack.
type SlackAlerter struct {
	WebhookURL string
	Channel    string
	client     *http.Client
}

// NewSlackAlerter creates a Slack alerter.
func NewSlackAlerter(webhookURL, channel string) *SlackAlerter {
	return &SlackAlerter{
		WebhookURL: webhookURL,
		Channel:    channel,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Alert sends an alert to Slack.
func (s *SlackAlerter) Alert(ctx context.Context, entry DLQEntry) error {
	text := fmt.Sprintf(":x: *Job Failed Permanently*\n"+
		"• Job ID: `%s`\n"+
		"• Type: `%s`\n"+
		"• Error: %s\n"+
		"• Attempts: %d\n"+
		"• Failed At: %s",
		entry.Job.ID,
		entry.Job.Type,
		entry.Error,
		entry.Attempts,
		entry.FailedAt.Format(time.RFC3339),
	)

	payload := map[string]any{
		"text": text,
	}
	if s.Channel != "" {
		payload["channel"] = s.Channel
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.WebhookURL, strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// LogAlerter logs alerts.
type LogAlerter struct {
	Logger func(format string, args ...any)
}

// NewLogAlerter creates a log alerter.
func NewLogAlerter(logger func(format string, args ...any)) *LogAlerter {
	return &LogAlerter{Logger: logger}
}

// Alert logs the failure.
func (l *LogAlerter) Alert(ctx context.Context, entry DLQEntry) error {
	l.Logger("DLQ ALERT: Job %s (type: %s) failed permanently after %d attempts. Error: %s",
		entry.Job.ID, entry.Job.Type, entry.Attempts, entry.Error)
	return nil
}

// CallbackAlerter calls a function.
type CallbackAlerter struct {
	Callback func(entry DLQEntry)
}

// Alert calls the callback.
func (c *CallbackAlerter) Alert(ctx context.Context, entry DLQEntry) error {
	c.Callback(entry)
	return nil
}
