// Package webhook provides webhook handling and queue integration.
package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nuulab/goflow/pkg/queue"
	"github.com/nuulab/goflow/pkg/workflow"
)

// WebhookHandler processes incoming webhooks and triggers jobs/workflows.
type WebhookHandler struct {
	queue    queue.Queue
	engine   *workflow.Engine
	hooks    map[string]*WebhookConfig
	secret   string
}

// WebhookConfig defines how a webhook triggers actions.
type WebhookConfig struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Path        string            `json:"path"`
	Secret      string            `json:"secret,omitempty"`
	Action      WebhookAction     `json:"action"`
	JobType     string            `json:"job_type,omitempty"`
	WorkflowID  string            `json:"workflow_id,omitempty"`
	Transform   func([]byte) any  `json:"-"`
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"created_at"`
}

// WebhookAction defines what the webhook triggers.
type WebhookAction string

const (
	ActionEnqueueJob    WebhookAction = "enqueue_job"
	ActionStartWorkflow WebhookAction = "start_workflow"
	ActionSignal        WebhookAction = "signal"
	ActionCustom        WebhookAction = "custom"
)

// WebhookPayload is the incoming webhook data.
type WebhookPayload struct {
	Event     string         `json:"event"`
	Data      map[string]any `json:"data"`
	Timestamp time.Time      `json:"timestamp"`
	Source    string         `json:"source"`
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(q queue.Queue, engine *workflow.Engine) *WebhookHandler {
	return &WebhookHandler{
		queue:  q,
		engine: engine,
		hooks:  make(map[string]*WebhookConfig),
	}
}

// Register adds a webhook configuration.
func (h *WebhookHandler) Register(cfg *WebhookConfig) {
	cfg.CreatedAt = time.Now()
	cfg.Enabled = true
	h.hooks[cfg.Path] = cfg
}

// RegisterJobWebhook creates a webhook that enqueues a job.
func (h *WebhookHandler) RegisterJobWebhook(path, jobType string) {
	h.Register(&WebhookConfig{
		ID:      fmt.Sprintf("wh-%d", time.Now().UnixNano()),
		Name:    jobType + " webhook",
		Path:    path,
		Action:  ActionEnqueueJob,
		JobType: jobType,
	})
}

// RegisterWorkflowWebhook creates a webhook that starts a workflow.
func (h *WebhookHandler) RegisterWorkflowWebhook(path, workflowID string) {
	h.Register(&WebhookConfig{
		ID:         fmt.Sprintf("wh-%d", time.Now().UnixNano()),
		Name:       workflowID + " webhook",
		Path:       path,
		Action:     ActionStartWorkflow,
		WorkflowID: workflowID,
	})
}

// SetGlobalSecret sets a default secret for HMAC validation.
func (h *WebhookHandler) SetGlobalSecret(secret string) {
	h.secret = secret
}

// Handler returns an HTTP handler for webhooks.
func (h *WebhookHandler) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Find matching webhook
		path := strings.TrimPrefix(r.URL.Path, "/webhooks")
		cfg, ok := h.hooks[path]
		if !ok {
			http.Error(w, "Webhook not found", http.StatusNotFound)
			return
		}

		if !cfg.Enabled {
			http.Error(w, "Webhook disabled", http.StatusServiceUnavailable)
			return
		}

		// Read body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}

		// Validate signature if secret is set
		secret := cfg.Secret
		if secret == "" {
			secret = h.secret
		}
		if secret != "" {
			sig := r.Header.Get("X-Webhook-Signature")
			if !h.validateSignature(body, sig, secret) {
				http.Error(w, "Invalid signature", http.StatusUnauthorized)
				return
			}
		}

		// Parse payload
		var payload WebhookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			// Try raw data
			payload = WebhookPayload{
				Data:      map[string]any{"raw": string(body)},
				Timestamp: time.Now(),
			}
		}

		// Execute action
		ctx := r.Context()
		result, err := h.executeAction(ctx, cfg, payload)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"result":  result,
		})
	})
}

func (h *WebhookHandler) executeAction(ctx context.Context, cfg *WebhookConfig, payload WebhookPayload) (any, error) {
	switch cfg.Action {
	case ActionEnqueueJob:
		// Transform payload if transformer is set
		var jobPayload any = payload.Data
		if cfg.Transform != nil {
			data, _ := json.Marshal(payload)
			jobPayload = cfg.Transform(data)
		}

		job, err := queue.NewJob(cfg.JobType, jobPayload)
		if err != nil {
			return nil, err
		}
		job.WithMetadata("webhook_id", cfg.ID)
		job.WithMetadata("webhook_event", payload.Event)

		if err := h.queue.Enqueue(ctx, job); err != nil {
			return nil, err
		}
		return map[string]string{"job_id": job.ID}, nil

	case ActionStartWorkflow:
		input := payload.Data
		if cfg.Transform != nil {
			data, _ := json.Marshal(payload)
			input = cfg.Transform(data).(map[string]any)
		}

		stateID, err := h.engine.Start(ctx, cfg.WorkflowID, input)
		if err != nil {
			return nil, err
		}
		return map[string]string{"workflow_state_id": stateID}, nil

	case ActionSignal:
		// Signal an existing workflow (requires engine.Signal method)
		// workflowID := payload.Data["workflow_id"].(string)
		// signal := payload.Data["signal"].(string)
		// return nil, h.engine.Signal(workflowID, signal, payload.Data)
		return nil, fmt.Errorf("signal action not yet implemented")

	default:
		return nil, fmt.Errorf("unknown action: %s", cfg.Action)
	}
}

func (h *WebhookHandler) validateSignature(body []byte, signature, secret string) bool {
	if signature == "" {
		return false
	}

	// Remove "sha256=" prefix if present
	signature = strings.TrimPrefix(signature, "sha256=")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expected))
}

// List returns all registered webhooks.
func (h *WebhookHandler) List() []*WebhookConfig {
	result := make([]*WebhookConfig, 0, len(h.hooks))
	for _, cfg := range h.hooks {
		result = append(result, cfg)
	}
	return result
}

// Enable enables a webhook by path.
func (h *WebhookHandler) Enable(path string) {
	if cfg, ok := h.hooks[path]; ok {
		cfg.Enabled = true
	}
}

// Disable disables a webhook by path.
func (h *WebhookHandler) Disable(path string) {
	if cfg, ok := h.hooks[path]; ok {
		cfg.Enabled = false
	}
}

// Remove removes a webhook by path.
func (h *WebhookHandler) Remove(path string) {
	delete(h.hooks, path)
}

// ============ Webhook Tool for Agents ============

// WebhookTool creates a tool for agents to register/trigger webhooks.
type WebhookTool struct {
	handler *WebhookHandler
}

// NewWebhookTool creates a webhook tool.
func NewWebhookTool(handler *WebhookHandler) *WebhookTool {
	return &WebhookTool{handler: handler}
}

// SendWebhookInput is the input for sending a webhook.
type SendWebhookInput struct {
	URL     string            `json:"url" description:"Target URL to send webhook to"`
	Event   string            `json:"event" description:"Event type/name"`
	Data    map[string]any    `json:"data" description:"Webhook payload data"`
	Headers map[string]string `json:"headers" description:"Optional HTTP headers"`
}

// SendWebhook sends an outgoing webhook.
func (wt *WebhookTool) SendWebhook(ctx context.Context, input SendWebhookInput) (string, error) {
	payload := WebhookPayload{
		Event:     input.Event,
		Data:      input.Data,
		Timestamp: time.Now(),
		Source:    "goflow",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", input.URL, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range input.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("webhook failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return fmt.Sprintf("Webhook sent successfully. Status: %d, Response: %s", resp.StatusCode, string(respBody)), nil
}
