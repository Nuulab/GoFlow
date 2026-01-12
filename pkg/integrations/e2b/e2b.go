// Package e2b provides E2B code sandbox integration for agents.
// E2B allows agents to execute code in isolated cloud sandboxes.
package e2b

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const baseURL = "https://api.e2b.dev"

// Client provides access to E2B sandboxes.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// New creates a new E2B client.
func New(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Sandbox represents an E2B sandbox instance.
type Sandbox struct {
	client    *Client
	ID        string `json:"sandboxId"`
	Template  string `json:"template"`
	ClientID  string `json:"clientId"`
}

// CreateSandboxOptions configures sandbox creation.
type CreateSandboxOptions struct {
	Template string            // Sandbox template (e.g., "python", "nodejs", "go")
	Timeout  time.Duration     // Sandbox timeout
	Metadata map[string]string // Custom metadata
}

// CreateSandbox creates a new sandbox.
func (c *Client) CreateSandbox(ctx context.Context, opts CreateSandboxOptions) (*Sandbox, error) {
	template := opts.Template
	if template == "" {
		template = "base"
	}

	body := map[string]any{
		"template": template,
	}
	if opts.Timeout > 0 {
		body["timeout"] = int(opts.Timeout.Seconds())
	}
	if opts.Metadata != nil {
		body["metadata"] = opts.Metadata
	}

	resp, err := c.post(ctx, "/sandboxes", body)
	if err != nil {
		return nil, err
	}

	var sandbox Sandbox
	if err := json.Unmarshal(resp, &sandbox); err != nil {
		return nil, err
	}
	sandbox.client = c
	return &sandbox, nil
}

// GetSandbox retrieves an existing sandbox.
func (c *Client) GetSandbox(ctx context.Context, sandboxID string) (*Sandbox, error) {
	resp, err := c.get(ctx, "/sandboxes/"+sandboxID)
	if err != nil {
		return nil, err
	}

	var sandbox Sandbox
	if err := json.Unmarshal(resp, &sandbox); err != nil {
		return nil, err
	}
	sandbox.client = c
	return &sandbox, nil
}

// ExecutionResult contains code execution output.
type ExecutionResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
	Error    string `json:"error,omitempty"`
}

// RunCode executes code in the sandbox.
func (s *Sandbox) RunCode(ctx context.Context, code string, language string) (*ExecutionResult, error) {
	body := map[string]any{
		"code": code,
	}
	if language != "" {
		body["language"] = language
	}

	resp, err := s.client.post(ctx, "/sandboxes/"+s.ID+"/code/run", body)
	if err != nil {
		return nil, err
	}

	var result ExecutionResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RunPython executes Python code.
func (s *Sandbox) RunPython(ctx context.Context, code string) (*ExecutionResult, error) {
	return s.RunCode(ctx, code, "python")
}

// RunJavaScript executes JavaScript code.
func (s *Sandbox) RunJavaScript(ctx context.Context, code string) (*ExecutionResult, error) {
	return s.RunCode(ctx, code, "javascript")
}

// RunBash executes a bash command.
func (s *Sandbox) RunBash(ctx context.Context, command string) (*ExecutionResult, error) {
	return s.RunCode(ctx, command, "bash")
}

// FileInfo contains file metadata.
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"isDir"`
}

// WriteFile writes a file to the sandbox.
func (s *Sandbox) WriteFile(ctx context.Context, path string, content []byte) error {
	_, err := s.client.post(ctx, "/sandboxes/"+s.ID+"/files", map[string]any{
		"path":    path,
		"content": string(content),
	})
	return err
}

// ReadFile reads a file from the sandbox.
func (s *Sandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	resp, err := s.client.get(ctx, "/sandboxes/"+s.ID+"/files?path="+path)
	if err != nil {
		return nil, err
	}

	var result struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	return []byte(result.Content), nil
}

// ListFiles lists files in a directory.
func (s *Sandbox) ListFiles(ctx context.Context, path string) ([]FileInfo, error) {
	resp, err := s.client.get(ctx, "/sandboxes/"+s.ID+"/files/list?path="+path)
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	if err := json.Unmarshal(resp, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// InstallPackage installs a package in the sandbox.
func (s *Sandbox) InstallPackage(ctx context.Context, packageManager, packageName string) (*ExecutionResult, error) {
	var cmd string
	switch packageManager {
	case "pip", "python":
		cmd = "pip install " + packageName
	case "npm", "node":
		cmd = "npm install " + packageName
	case "go":
		cmd = "go get " + packageName
	default:
		cmd = packageManager + " install " + packageName
	}
	return s.RunBash(ctx, cmd)
}

// Kill terminates the sandbox.
func (s *Sandbox) Kill(ctx context.Context) error {
	_, err := s.client.delete(ctx, "/sandboxes/"+s.ID)
	return err
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
	req.Header.Set("X-E2B-API-Key", c.apiKey)

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
		return nil, fmt.Errorf("E2B API error (%d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// ============ GoFlow Tool Adapter ============

// Tool creates a GoFlow tool for code execution.
type Tool struct {
	client   *Client
	template string
}

// NewTool creates an E2B tool.
func NewTool(apiKey, template string) *Tool {
	return &Tool{
		client:   New(apiKey),
		template: template,
	}
}

// Name returns the tool name.
func (t *Tool) Name() string { return "e2b_sandbox" }

// Description returns the tool description.
func (t *Tool) Description() string {
	return "Execute code in an isolated cloud sandbox. Supports Python, JavaScript, Bash, and more."
}

// ExecuteInput is the input for code execution.
type ExecuteInput struct {
	Code     string `json:"code"`
	Language string `json:"language"` // python, javascript, bash
}

// Execute runs code in a sandbox.
func (t *Tool) Execute(ctx context.Context, input string) (string, error) {
	var in ExecuteInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		// Treat as raw code
		in = ExecuteInput{Code: input, Language: "python"}
	}

	sandbox, err := t.client.CreateSandbox(ctx, CreateSandboxOptions{
		Template: t.template,
		Timeout:  60 * time.Second,
	})
	if err != nil {
		return "", err
	}
	defer sandbox.Kill(ctx)

	result, err := sandbox.RunCode(ctx, in.Code, in.Language)
	if err != nil {
		return "", err
	}

	if result.Error != "" {
		return fmt.Sprintf("Error: %s\nStderr: %s", result.Error, result.Stderr), nil
	}

	output := result.Stdout
	if result.Stderr != "" {
		output += "\nStderr: " + result.Stderr
	}
	return output, nil
}
