// Package browserbase provides Browserbase browser automation for agents.
// Browserbase allows agents to interact with web pages programmatically.
package browserbase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const baseURL = "https://www.browserbase.com/v1"

// Client provides access to Browserbase browser automation.
type Client struct {
	apiKey     string
	projectID  string
	httpClient *http.Client
}

// New creates a new Browserbase client.
func New(apiKey, projectID string) *Client {
	return &Client{
		apiKey:    apiKey,
		projectID: projectID,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Session represents a browser session.
type Session struct {
	client    *Client
	ID        string `json:"id"`
	Status    string `json:"status"`
	ProjectID string `json:"projectId"`
	CreatedAt string `json:"createdAt"`
	ConnectURL string `json:"connectUrl,omitempty"`
}

// CreateSessionOptions configures session creation.
type CreateSessionOptions struct {
	// Fingerprint browser fingerprint options
	Fingerprint *Fingerprint
	// Proxy configuration
	Proxy *Proxy
	// Timeout in seconds
	Timeout int
	// KeepAlive keep session alive after disconnect
	KeepAlive bool
}

// Fingerprint controls browser fingerprinting.
type Fingerprint struct {
	Browsers   []string `json:"browsers,omitempty"`   // chrome, firefox, safari
	Devices    []string `json:"devices,omitempty"`    // desktop, mobile
	Locales    []string `json:"locales,omitempty"`    // en-US, etc.
	OperatingSystems []string `json:"operatingSystems,omitempty"`
}

// Proxy configures proxy settings.
type Proxy struct {
	Type     string `json:"type"`     // http, socks5
	Server   string `json:"server"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// CreateSession creates a new browser session.
func (c *Client) CreateSession(ctx context.Context, opts *CreateSessionOptions) (*Session, error) {
	body := map[string]any{
		"projectId": c.projectID,
	}
	
	if opts != nil {
		if opts.Fingerprint != nil {
			body["fingerprint"] = opts.Fingerprint
		}
		if opts.Proxy != nil {
			body["proxies"] = []any{opts.Proxy}
		}
		if opts.Timeout > 0 {
			body["timeout"] = opts.Timeout
		}
		if opts.KeepAlive {
			body["keepAlive"] = true
		}
	}

	resp, err := c.post(ctx, "/sessions", body)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(resp, &session); err != nil {
		return nil, err
	}
	session.client = c
	return &session, nil
}

// GetSession retrieves an existing session.
func (c *Client) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	resp, err := c.get(ctx, "/sessions/"+sessionID)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(resp, &session); err != nil {
		return nil, err
	}
	session.client = c
	return &session, nil
}

// ListSessions lists all sessions.
func (c *Client) ListSessions(ctx context.Context) ([]Session, error) {
	resp, err := c.get(ctx, "/sessions")
	if err != nil {
		return nil, err
	}

	var sessions []Session
	if err := json.Unmarshal(resp, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// PageContent contains page content and metadata.
type PageContent struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	HTML    string `json:"html,omitempty"`
	Text    string `json:"text,omitempty"`
	Screenshot []byte `json:"screenshot,omitempty"`
}

// Action represents a browser action.
type Action struct {
	Type     string         `json:"type"`     // navigate, click, type, scroll, screenshot, extract
	Selector string         `json:"selector,omitempty"`
	Value    string         `json:"value,omitempty"`
	Options  map[string]any `json:"options,omitempty"`
}

// ActionResult contains the result of a browser action.
type ActionResult struct {
	Success    bool   `json:"success"`
	Data       any    `json:"data,omitempty"`
	Screenshot []byte `json:"screenshot,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Navigate navigates to a URL.
func (s *Session) Navigate(ctx context.Context, url string) (*ActionResult, error) {
	return s.Execute(ctx, Action{Type: "navigate", Value: url})
}

// Click clicks an element.
func (s *Session) Click(ctx context.Context, selector string) (*ActionResult, error) {
	return s.Execute(ctx, Action{Type: "click", Selector: selector})
}

// Type types text into an element.
func (s *Session) Type(ctx context.Context, selector, text string) (*ActionResult, error) {
	return s.Execute(ctx, Action{Type: "type", Selector: selector, Value: text})
}

// Screenshot takes a screenshot.
func (s *Session) Screenshot(ctx context.Context) ([]byte, error) {
	result, err := s.Execute(ctx, Action{Type: "screenshot"})
	if err != nil {
		return nil, err
	}
	return result.Screenshot, nil
}

// ExtractText extracts text content from the page.
func (s *Session) ExtractText(ctx context.Context, selector string) (string, error) {
	result, err := s.Execute(ctx, Action{
		Type:     "extract",
		Selector: selector,
		Options:  map[string]any{"type": "text"},
	})
	if err != nil {
		return "", err
	}
	if text, ok := result.Data.(string); ok {
		return text, nil
	}
	return "", nil
}

// ExtractHTML extracts HTML content from the page.
func (s *Session) ExtractHTML(ctx context.Context, selector string) (string, error) {
	result, err := s.Execute(ctx, Action{
		Type:     "extract",
		Selector: selector,
		Options:  map[string]any{"type": "html"},
	})
	if err != nil {
		return "", err
	}
	if html, ok := result.Data.(string); ok {
		return html, nil
	}
	return "", nil
}

// Execute executes a browser action.
func (s *Session) Execute(ctx context.Context, action Action) (*ActionResult, error) {
	resp, err := s.client.post(ctx, "/sessions/"+s.ID+"/actions", action)
	if err != nil {
		return nil, err
	}

	var result ActionResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Close closes the browser session.
func (s *Session) Close(ctx context.Context) error {
	_, err := s.client.delete(ctx, "/sessions/"+s.ID)
	return err
}

// GetDebugURL returns the debug URL for the session.
func (s *Session) GetDebugURL(ctx context.Context) (string, error) {
	resp, err := s.client.get(ctx, "/sessions/"+s.ID+"/debug")
	if err != nil {
		return "", err
	}
	var result struct {
		DebuggerURL string `json:"debuggerUrl"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}
	return result.DebuggerURL, nil
}

// ============ HTTP Helpers ============

func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return c.doRequest(req)
}

func (c *Client) post(ctx context.Context, path string, body any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.doRequest(req)
}

func (c *Client) delete(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "DELETE", baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return c.doRequest(req)
}

func (c *Client) doRequest(req *http.Request) ([]byte, error) {
	req.Header.Set("X-BB-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Browserbase API error (%d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// ============ GoFlow Tool Adapter ============

// Tool creates a GoFlow tool for browser automation.
type Tool struct {
	client *Client
}

// NewTool creates a Browserbase tool.
func NewTool(apiKey, projectID string) *Tool {
	return &Tool{
		client: New(apiKey, projectID),
	}
}

// Name returns the tool name.
func (t *Tool) Name() string { return "browserbase" }

// Description returns the tool description.
func (t *Tool) Description() string {
	return "Control a browser to navigate web pages, click elements, fill forms, and extract content."
}

// BrowserInput is the input for browser actions.
type BrowserInput struct {
	Action   string `json:"action"`   // navigate, click, type, screenshot, extract
	URL      string `json:"url,omitempty"`
	Selector string `json:"selector,omitempty"`
	Text     string `json:"text,omitempty"`
}

// Execute performs a browser action.
func (t *Tool) Execute(ctx context.Context, input string) (string, error) {
	var in BrowserInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	// Create session
	session, err := t.client.CreateSession(ctx, nil)
	if err != nil {
		return "", err
	}
	defer session.Close(ctx)

	var result *ActionResult

	switch in.Action {
	case "navigate":
		result, err = session.Navigate(ctx, in.URL)
	case "click":
		result, err = session.Click(ctx, in.Selector)
	case "type":
		result, err = session.Type(ctx, in.Selector, in.Text)
	case "screenshot":
		screenshot, e := session.Screenshot(ctx)
		if e != nil {
			return "", e
		}
		return fmt.Sprintf("Screenshot taken (%d bytes)", len(screenshot)), nil
	case "extract":
		text, e := session.ExtractText(ctx, in.Selector)
		if e != nil {
			return "", e
		}
		return text, nil
	default:
		return "", fmt.Errorf("unknown action: %s", in.Action)
	}

	if err != nil {
		return "", err
	}

	if result.Error != "" {
		return fmt.Sprintf("Error: %s", result.Error), nil
	}

	return fmt.Sprintf("Action '%s' completed successfully", in.Action), nil
}
