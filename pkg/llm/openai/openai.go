// Package openai provides an OpenAI LLM provider.
package openai

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

const defaultBaseURL = "https://api.openai.com/v1"

// Client implements core.LLM for OpenAI.
type Client struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// Option configures the OpenAI client.
type Option func(*Client)

// New creates a new OpenAI client.
// If apiKey is empty, it reads from OPENAI_API_KEY environment variable.
func New(apiKey string, opts ...Option) *Client {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	c := &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		model:   "gpt-4o",
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

// WithBaseURL sets a custom base URL (for Azure OpenAI or proxies).
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

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
	Stop        []string      `json:"stop,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

type errorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
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

	// Convert messages
	chatMessages := make([]chatMessage, len(messages))
	for i, msg := range messages {
		chatMessages[i] = chatMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	req := chatRequest{
		Model:    c.model,
		Messages: chatMessages,
	}

	if options.Temperature > 0 {
		req.Temperature = &options.Temperature
	}
	if options.MaxTokens > 0 {
		req.MaxTokens = &options.MaxTokens
	}
	if options.TopP > 0 {
		req.TopP = &options.TopP
	}
	if len(options.StopSequences) > 0 {
		req.Stop = options.StopSequences
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

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
		return "", fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, errResp.Error.Message)
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned")
	}

	return chatResp.Choices[0].Message.Content, nil
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

	// Convert messages
	chatMessages := make([]chatMessage, len(messages))
	for i, msg := range messages {
		chatMessages[i] = chatMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	req := chatRequest{
		Model:    c.model,
		Messages: chatMessages,
		Stream:   true,
	}

	if options.Temperature > 0 {
		req.Temperature = &options.Temperature
	}
	if options.MaxTokens > 0 {
		req.MaxTokens = &options.MaxTokens
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		var errResp errorResponse
		json.Unmarshal(respBody, &errResp)
		return nil, fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, errResp.Error.Message)
	}

	ch := make(chan string)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)
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
			if lineStr == "" || lineStr == "data: [DONE]" {
				continue
			}

			// Parse SSE data
			if len(lineStr) > 6 && lineStr[:6] == "data: " {
				var chunk streamChunk
				if err := json.Unmarshal([]byte(lineStr[6:]), &chunk); err != nil {
					continue
				}

				if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
					ch <- chunk.Choices[0].Delta.Content
				}

				if len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != nil {
					return
				}
			}

			// Reset decoder for next chunk
			decoder = json.NewDecoder(resp.Body)
			_ = decoder
		}
	}()

	return ch, nil
}
