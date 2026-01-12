// Package gemini provides a Google Gemini LLM provider.
package gemini

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

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// Client implements core.LLM for Google Gemini.
type Client struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// Option configures the Gemini client.
type Option func(*Client)

// New creates a new Gemini client.
// If apiKey is empty, it reads from GOOGLE_API_KEY or GEMINI_API_KEY environment variable.
func New(apiKey string, opts ...Option) *Client {
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("GEMINI_API_KEY")
		}
	}

	c := &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		model:   "gemini-3-flash-preview",
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

type generateRequest struct {
	Contents         []content           `json:"contents"`
	SystemInstruction *content           `json:"systemInstruction,omitempty"`
	GenerationConfig *generationConfig   `json:"generationConfig,omitempty"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text"`
}

type generationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type generateResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

type streamResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason,omitempty"`
	} `json:"candidates"`
}

type errorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
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
	var systemInstruction *content
	var contents []content

	for _, msg := range messages {
		if msg.Role == core.RoleSystem {
			systemInstruction = &content{
				Parts: []part{{Text: msg.Content}},
			}
		} else {
			role := "user"
			if msg.Role == core.RoleAssistant {
				role = "model"
			}
			contents = append(contents, content{
				Role:  role,
				Parts: []part{{Text: msg.Content}},
			})
		}
	}

	req := generateRequest{
		Contents:          contents,
		SystemInstruction: systemInstruction,
	}

	// Add generation config if any options set
	if options.Temperature > 0 || options.MaxTokens > 0 || options.TopP > 0 || len(options.StopSequences) > 0 {
		req.GenerationConfig = &generationConfig{}
		if options.Temperature > 0 {
			req.GenerationConfig.Temperature = &options.Temperature
		}
		if options.MaxTokens > 0 {
			req.GenerationConfig.MaxOutputTokens = &options.MaxTokens
		}
		if options.TopP > 0 {
			req.GenerationConfig.TopP = &options.TopP
		}
		if len(options.StopSequences) > 0 {
			req.GenerationConfig.StopSequences = options.StopSequences
		}
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, c.model, c.apiKey)

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	httpReq.Header.Set("Content-Type", "application/json")

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
		return "", fmt.Errorf("Gemini API error (%d): %s", resp.StatusCode, errResp.Error.Message)
	}

	var genResp generateResponse
	if err := json.Unmarshal(respBody, &genResp); err != nil {
		return "", err
	}

	if len(genResp.Candidates) == 0 || len(genResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content returned")
	}

	return genResp.Candidates[0].Content.Parts[0].Text, nil
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
	var systemInstruction *content
	var contents []content

	for _, msg := range messages {
		if msg.Role == core.RoleSystem {
			systemInstruction = &content{
				Parts: []part{{Text: msg.Content}},
			}
		} else {
			role := "user"
			if msg.Role == core.RoleAssistant {
				role = "model"
			}
			contents = append(contents, content{
				Role:  role,
				Parts: []part{{Text: msg.Content}},
			})
		}
	}

	req := generateRequest{
		Contents:          contents,
		SystemInstruction: systemInstruction,
	}

	if options.Temperature > 0 || options.MaxTokens > 0 {
		req.GenerationConfig = &generationConfig{}
		if options.Temperature > 0 {
			req.GenerationConfig.Temperature = &options.Temperature
		}
		if options.MaxTokens > 0 {
			req.GenerationConfig.MaxOutputTokens = &options.MaxTokens
		}
	}

	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?key=%s", c.baseURL, c.model, c.apiKey)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		var errResp errorResponse
		json.Unmarshal(respBody, &errResp)
		return nil, fmt.Errorf("Gemini API error (%d): %s", resp.StatusCode, errResp.Error.Message)
	}

	ch := make(chan string)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		// Read entire response body
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return
		}

		// Parse JSON array of stream responses
		var streamResps []streamResponse
		if err := json.Unmarshal(respBody, &streamResps); err != nil {
			return
		}

		// Emit each chunk
		for _, streamResp := range streamResps {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if len(streamResp.Candidates) > 0 && len(streamResp.Candidates[0].Content.Parts) > 0 {
				text := streamResp.Candidates[0].Content.Parts[0].Text
				if text != "" {
					ch <- text
				}
			}
		}
	}()

	return ch, nil
}
