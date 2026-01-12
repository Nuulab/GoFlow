// Package httpclient provides a resilient HTTP client with retries and backoff.
package httpclient

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

// Config holds configuration for the resilient HTTP client.
type Config struct {
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int
	// BaseDelay is the initial delay between retries.
	BaseDelay time.Duration
	// MaxDelay is the maximum delay between retries.
	MaxDelay time.Duration
	// Timeout is the request timeout.
	Timeout time.Duration
	// RetryableStatusCodes defines which HTTP status codes should trigger a retry.
	RetryableStatusCodes []int
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		MaxRetries:           3,
		BaseDelay:            500 * time.Millisecond,
		MaxDelay:             30 * time.Second,
		Timeout:              60 * time.Second,
		RetryableStatusCodes: []int{429, 500, 502, 503, 504},
	}
}

// Client is a resilient HTTP client with automatic retries.
type Client struct {
	config     Config
	httpClient *http.Client
}

// New creates a new resilient HTTP client.
func New(cfg Config) *Client {
	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// NewWithHTTPClient creates a resilient client wrapping an existing http.Client.
func NewWithHTTPClient(cfg Config, client *http.Client) *Client {
	return &Client{
		config:     cfg,
		httpClient: client,
	}
}

// Do executes an HTTP request with retries.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	var lastErr error
	var lastResp *http.Response

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		// Check context before making request
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Clone the request for retry (body may have been consumed)
		reqCopy := req.Clone(ctx)

		resp, err := c.httpClient.Do(reqCopy)
		if err != nil {
			lastErr = err
			c.waitForRetry(ctx, attempt)
			continue
		}

		// Check if status code is retryable
		if c.isRetryable(resp.StatusCode) {
			// Close the response body to allow connection reuse
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			
			lastResp = resp
			lastErr = fmt.Errorf("received retryable status code: %d", resp.StatusCode)
			c.waitForRetry(ctx, attempt)
			continue
		}

		return resp, nil
	}

	if lastErr != nil {
		return lastResp, fmt.Errorf("request failed after %d attempts: %w", c.config.MaxRetries+1, lastErr)
	}

	return lastResp, nil
}

// isRetryable checks if a status code should trigger a retry.
func (c *Client) isRetryable(statusCode int) bool {
	for _, code := range c.config.RetryableStatusCodes {
		if code == statusCode {
			return true
		}
	}
	return false
}

// waitForRetry waits for the appropriate backoff duration.
func (c *Client) waitForRetry(ctx context.Context, attempt int) {
	delay := c.calculateBackoff(attempt)
	
	select {
	case <-ctx.Done():
		return
	case <-time.After(delay):
		return
	}
}

// calculateBackoff calculates the exponential backoff duration.
func (c *Client) calculateBackoff(attempt int) time.Duration {
	delay := time.Duration(float64(c.config.BaseDelay) * math.Pow(2, float64(attempt)))
	if delay > c.config.MaxDelay {
		delay = c.config.MaxDelay
	}
	return delay
}

// Get performs a GET request with retries.
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	return c.Do(ctx, req)
}

// Post performs a POST request with retries.
func (c *Client) Post(ctx context.Context, url string, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	return c.Do(ctx, req)
}
