// Package anthropic provides an Anthropic Claude LLM provider.
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/nuulab/goflow/pkg/core"
)

const defaultBaseURL = "https://api.anthropic.com/v1"
const apiVersion = "2023-06-01"

// Client implements core.LLM for Anthropic Claude.
type Client struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// Option configures the Anthropic client.
type Option func(*Client)

// New creates a new Anthropic client.
// If apiKey is empty, it reads from ANTHROPIC_API_KEY environment variable.
func New(apiKey string, opts ...Option) *Client {
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	c := &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		model:   "claude-3-5-sonnet-20241022",
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// WithModel sets the model to use.
func WithModel(model string) Option {
	return func(c *Client) {
		c.model = model
	}
}

// WithBaseURL sets a custom base URL.
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// ============ Request/Response Types ============

type messagesRequest struct {
	Model       string           `json:"model"`
	Messages    []messageContent `json:"messages"`
	System      string           `json:"system,omitempty"`
	MaxTokens   int              `json:"max_tokens"`
	Temperature *float64         `json:"temperature,omitempty"`
	TopP        *float64         `json:"top_p,omitempty"`
	StopSequences []string       `json:"stop_sequences,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
}

type messageContent struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type messagesResponse struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Role         string `json:"role"`
	Content      []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence,omitempty"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type streamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index,omitempty"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta,omitempty"`
	ContentBlock *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content_block,omitempty"`
}

type errorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// ============ LLM Interface Implementation ============

// Generate produces a completion for the given prompt.
func (c *Client) Generate(ctx context.Context, prompt string, opts ...core.Option) (string, error) {
	return c.GenerateChat(ctx, []core.Message{
		{Role: core.RoleUser, Content: prompt},
	}, opts...)
}

// GenerateChat produces a completion for a conversation.
func (c *Client) GenerateChat(ctx context.Context, messages []core.Message, opts ...core.Option) (string, error) {
	options := &core.CallOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Extract system message if present
	var systemPrompt string
	var chatMessages []messageContent

	for _, msg := range messages {
		if msg.Role == core.RoleSystem {
			systemPrompt = msg.Content
		} else {
			role := string(msg.Role)
			if role == "tool" {
				role = "user" // Anthropic doesn't have tool role
			}
			chatMessages = append(chatMessages, messageContent{
				Role:    role,
				Content: msg.Content,
			})
		}
	}

	maxTokens := options.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	req := messagesRequest{
		Model:     c.model,
		Messages:  chatMessages,
		System:    systemPrompt,
		MaxTokens: maxTokens,
	}

	if options.Temperature > 0 {
		req.Temperature = &options.Temperature
	}
	if options.TopP > 0 {
		req.TopP = &options.TopP
	}
	if len(options.StopSequences) > 0 {
		req.StopSequences = options.StopSequences
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", apiVersion)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= 400 {
		var errResp errorResponse
		json.Unmarshal(respBody, &errResp)
		return "", fmt.Errorf("Anthropic API error (%d): %s", resp.StatusCode, errResp.Error.Message)
	}

	var msgResp messagesResponse
	if err := json.Unmarshal(respBody, &msgResp); err != nil {
		return "", err
	}

	if len(msgResp.Content) == 0 {
		return "", fmt.Errorf("no content returned")
	}

	return msgResp.Content[0].Text, nil
}

// Stream produces a streaming completion for the given prompt.
func (c *Client) Stream(ctx context.Context, prompt string, opts ...core.Option) (<-chan string, error) {
	return c.StreamChat(ctx, []core.Message{
		{Role: core.RoleUser, Content: prompt},
	}, opts...)
}

// StreamChat produces a streaming completion for a conversation.
func (c *Client) StreamChat(ctx context.Context, messages []core.Message, opts ...core.Option) (<-chan string, error) {
	options := &core.CallOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Extract system message if present
	var systemPrompt string
	var chatMessages []messageContent

	for _, msg := range messages {
		if msg.Role == core.RoleSystem {
			systemPrompt = msg.Content
		} else {
			role := string(msg.Role)
			if role == "tool" {
				role = "user"
			}
			chatMessages = append(chatMessages, messageContent{
				Role:    role,
				Content: msg.Content,
			})
		}
	}

	maxTokens := options.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	req := messagesRequest{
		Model:     c.model,
		Messages:  chatMessages,
		System:    systemPrompt,
		MaxTokens: maxTokens,
		Stream:    true,
	}

	if options.Temperature > 0 {
		req.Temperature = &options.Temperature
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", apiVersion)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		var errResp errorResponse
		json.Unmarshal(respBody, &errResp)
		return nil, fmt.Errorf("Anthropic API error (%d): %s", resp.StatusCode, errResp.Error.Message)
	}

	ch := make(chan string)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Read SSE line
			var line []byte
			for {
				b := make([]byte, 1)
				_, err := resp.Body.Read(b)
				if err != nil {
					return
				}
				if b[0] == '\n' {
					break
				}
				line = append(line, b[0])
			}

			lineStr := string(line)
			if lineStr == "" {
				continue
			}

			// Parse SSE data
			if len(lineStr) > 6 && lineStr[:6] == "data: " {
				var event streamEvent
				if err := json.Unmarshal([]byte(lineStr[6:]), &event); err != nil {
					continue
				}

				switch event.Type {
				case "content_block_delta":
					if event.Delta != nil && event.Delta.Text != "" {
						ch <- event.Delta.Text
					}
				case "message_stop":
					return
				}
			}
		}
	}()

	return ch, nil
}
