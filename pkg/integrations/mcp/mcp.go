// Package mcp provides Model Context Protocol server integration.
// MCP allows agents to use tools from external MCP servers via WebSocket, SSE, or HTTP.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client connects to an MCP server and exposes its tools.
type Client struct {
	name      string
	transport Transport
	tools     []Tool
	mu        sync.RWMutex
}

// Transport defines how to connect to an MCP server.
type Transport interface {
	Connect(ctx context.Context) error
	Call(ctx context.Context, method string, params any) (json.RawMessage, error)
	Close() error
}

// Tool represents an MCP tool.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"inputSchema"`
}

// Config configures an MCP client.
type Config struct {
	Name      string
	Transport TransportConfig
}

// TransportConfig specifies transport settings.
type TransportConfig struct {
	Type    string // "ws", "sse", "http"
	URL     string
	Headers map[string]string
	Timeout time.Duration
}

// New creates a new MCP client.
func New(cfg Config) (*Client, error) {
	var transport Transport
	switch cfg.Transport.Type {
	case "ws":
		transport = &WebSocketTransport{
			url:     cfg.Transport.URL,
			headers: cfg.Transport.Headers,
			timeout: cfg.Transport.Timeout,
		}
	case "sse":
		transport = &SSETransport{
			url:     cfg.Transport.URL,
			headers: cfg.Transport.Headers,
			timeout: cfg.Transport.Timeout,
		}
	case "http":
		transport = &HTTPTransport{
			url:     cfg.Transport.URL,
			headers: cfg.Transport.Headers,
			timeout: cfg.Transport.Timeout,
		}
	default:
		return nil, fmt.Errorf("unknown transport type: %s", cfg.Transport.Type)
	}

	return &Client{
		name:      cfg.Name,
		transport: transport,
		tools:     make([]Tool, 0),
	}, nil
}

// Connect establishes connection and discovers available tools.
func (c *Client) Connect(ctx context.Context) error {
	if err := c.transport.Connect(ctx); err != nil {
		return err
	}

	// Discover tools
	result, err := c.transport.Call(ctx, "tools/list", nil)
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	var response struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return err
	}

	c.mu.Lock()
	c.tools = response.Tools
	c.mu.Unlock()

	return nil
}

// Tools returns available tools.
func (c *Client) Tools() []Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Tool, len(c.tools))
	copy(result, c.tools)
	return result
}

// Call invokes an MCP tool.
func (c *Client) Call(ctx context.Context, toolName string, args any) (string, error) {
	result, err := c.transport.Call(ctx, "tools/call", map[string]any{
		"name":      toolName,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}

	var response struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return string(result), nil
	}

	if len(response.Content) > 0 {
		return response.Content[0].Text, nil
	}
	return string(result), nil
}

// Close disconnects from the MCP server.
func (c *Client) Close() error {
	return c.transport.Close()
}

// Name returns the client name.
func (c *Client) Name() string {
	return c.name
}

// ============ WebSocket Transport ============

// WebSocketTransport connects via WebSocket.
type WebSocketTransport struct {
	url     string
	headers map[string]string
	timeout time.Duration
	conn    *websocket.Conn
	mu      sync.Mutex
	msgID   int
}

func (t *WebSocketTransport) Connect(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: t.timeout,
	}

	header := http.Header{}
	for k, v := range t.headers {
		header.Set(k, v)
	}

	conn, _, err := dialer.DialContext(ctx, t.url, header)
	if err != nil {
		return err
	}
	t.conn = conn

	// Send initialize
	_, err = t.Call(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "goflow",
			"version": "1.0.0",
		},
	})
	return err
}

func (t *WebSocketTransport) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	t.mu.Lock()
	t.msgID++
	id := t.msgID
	t.mu.Unlock()

	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		msg["params"] = params
	}

	if err := t.conn.WriteJSON(msg); err != nil {
		return nil, err
	}

	var response struct {
		ID     int             `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := t.conn.ReadJSON(&response); err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s", response.Error.Code, response.Error.Message)
	}

	return response.Result, nil
}

func (t *WebSocketTransport) Close() error {
	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}

// ============ SSE Transport ============

// SSETransport connects via Server-Sent Events.
type SSETransport struct {
	url         string
	headers     map[string]string
	timeout     time.Duration
	client      *http.Client
	sessionURL  string
}

func (t *SSETransport) Connect(ctx context.Context) error {
	t.client = &http.Client{Timeout: t.timeout}

	// Connect to SSE endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", t.url, nil)
	if err != nil {
		return err
	}
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read session endpoint from SSE
	// (simplified - real implementation would parse SSE events)
	t.sessionURL = t.url + "/session"
	return nil
}

func (t *SSETransport) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.sessionURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response struct {
		Result json.RawMessage `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&response)
	return response.Result, nil
}

func (t *SSETransport) Close() error {
	return nil
}

// ============ HTTP Transport ============

// HTTPTransport connects via HTTP.
type HTTPTransport struct {
	url     string
	headers map[string]string
	timeout time.Duration
	client  *http.Client
}

func (t *HTTPTransport) Connect(ctx context.Context) error {
	t.client = &http.Client{Timeout: t.timeout}
	return nil
}

func (t *HTTPTransport) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("MCP error: %s", response.Error.Message)
	}

	return response.Result, nil
}

func (t *HTTPTransport) Close() error {
	return nil
}

// ============ Tool Adapter ============

// ToGoFlowTool converts an MCP tool to a GoFlow tool.
func (c *Client) ToGoFlowTool(mcpTool Tool) *GoFlowTool {
	return &GoFlowTool{
		name:        c.name + "." + mcpTool.Name,
		description: mcpTool.Description,
		client:      c,
		mcpName:     mcpTool.Name,
	}
}

// GoFlowTool wraps an MCP tool for use with GoFlow agents.
type GoFlowTool struct {
	name        string
	description string
	client      *Client
	mcpName     string
}

func (t *GoFlowTool) Name() string        { return t.name }
func (t *GoFlowTool) Description() string { return t.description }

func (t *GoFlowTool) Execute(ctx context.Context, input string) (string, error) {
	var args any
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		args = map[string]any{"input": input}
	}
	return t.client.Call(ctx, t.mcpName, args)
}
