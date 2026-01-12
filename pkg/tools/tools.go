// Package tools provides the function calling and tool execution framework.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// Tool represents an executable function that can be called by an LLM.
type Tool struct {
	// Name is the unique identifier for this tool.
	Name string `json:"name"`
	// Description explains what the tool does (used by LLM for selection).
	Description string `json:"description"`
	// Parameters defines the JSON schema for input parameters.
	Parameters Schema `json:"parameters"`
	// Execute is the function that runs when the tool is invoked.
	Execute func(ctx context.Context, jsonInput string) (string, error) `json:"-"`
}

// Schema represents a JSON schema for tool parameters.
type Schema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

// Property represents a property in a JSON schema.
type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// Registry holds a collection of tools and provides lookup.
// It is safe for concurrent use.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*Tool
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]*Tool),
	}
}

// Register adds a tool to the registry.
// It is safe for concurrent use.
func (r *Registry) Register(tool *Tool) error {
	if tool.Name == "" {
		return fmt.Errorf("tools: tool name cannot be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[tool.Name]; exists {
		return fmt.Errorf("tools: tool %q already registered", tool.Name)
	}
	r.tools[tool.Name] = tool
	return nil
}

// Get retrieves a tool by name.
// It is safe for concurrent use.
func (r *Registry) Get(name string) (*Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools.
// It is safe for concurrent use.
func (r *Registry) List() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

// Execute runs a tool by name with the given JSON input.
func (r *Registry) Execute(ctx context.Context, name string, jsonInput string) (string, error) {
	tool, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("tools: unknown tool %q", name)
	}
	return tool.Execute(ctx, jsonInput)
}

// ToOpenAIFormat converts tools to OpenAI's function calling format.
// It is safe for concurrent use.
func (r *Registry) ToOpenAIFormat() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]map[string]any, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.Parameters,
			},
		})
	}
	return result
}

// ToolCall represents a request from the LLM to execute a tool.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolResult represents the result of executing a tool.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	Error      string `json:"error,omitempty"`
}

// ExecuteCalls runs multiple tool calls and returns results.
func (r *Registry) ExecuteCalls(ctx context.Context, calls []ToolCall) []ToolResult {
	results := make([]ToolResult, len(calls))

	for i, call := range calls {
		output, err := r.Execute(ctx, call.Name, call.Arguments)
		results[i] = ToolResult{
			ToolCallID: call.ID,
			Content:    output,
		}
		if err != nil {
			results[i].Error = err.Error()
		}
	}

	return results
}

// NewTool is a helper to create a typed tool with automatic JSON parsing.
func NewTool[I, O any](name, description string, fn func(ctx context.Context, input I) (O, error)) *Tool {
	return &Tool{
		Name:        name,
		Description: description,
		Parameters:  inferSchema[I](),
		Execute: func(ctx context.Context, jsonInput string) (string, error) {
			var input I
			if err := json.Unmarshal([]byte(jsonInput), &input); err != nil {
				return "", fmt.Errorf("failed to parse input: %w", err)
			}

			output, err := fn(ctx, input)
			if err != nil {
				return "", err
			}

			result, err := json.Marshal(output)
			if err != nil {
				return "", fmt.Errorf("failed to marshal output: %w", err)
			}

			return string(result), nil
		},
	}
}

// inferSchema creates a JSON schema from a type using reflection.
func inferSchema[T any]() Schema {
	var zero T
	return reflectSchema(reflect.TypeOf(zero))
}

// reflectSchema builds a Schema from a reflect.Type.
func reflectSchema(t reflect.Type) Schema {
	if t == nil {
		return Schema{Type: "object", Properties: make(map[string]Property)}
	}

	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	schema := Schema{
		Type:       "object",
		Properties: make(map[string]Property),
		Required:   make([]string, 0),
	}

	if t.Kind() != reflect.Struct {
		schema.Type = goKindToJSONType(t.Kind())
		return schema
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get JSON tag
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		name := field.Name
		omitempty := false
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				name = parts[0]
			}
			for _, p := range parts[1:] {
				if p == "omitempty" {
					omitempty = true
				}
			}
		}

		// Get description from doc tag
		description := field.Tag.Get("description")

		prop := Property{
			Type:        goKindToJSONType(field.Type.Kind()),
			Description: description,
		}

		// Handle enum tag
		if enumTag := field.Tag.Get("enum"); enumTag != "" {
			prop.Enum = strings.Split(enumTag, ",")
		}

		schema.Properties[name] = prop

		// Add to required if not omitempty
		if !omitempty {
			schema.Required = append(schema.Required, name)
		}
	}

	return schema
}

// goKindToJSONType converts Go types to JSON schema types.
func goKindToJSONType(k reflect.Kind) string {
	switch k {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map, reflect.Struct:
		return "object"
	default:
		return "string"
	}
}

// ToAnthropicFormat converts tools to Anthropic's tool format.
// It is safe for concurrent use.
func (r *Registry) ToAnthropicFormat() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]map[string]any, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"input_schema": map[string]any{
				"type":       tool.Parameters.Type,
				"properties": tool.Parameters.Properties,
				"required":   tool.Parameters.Required,
			},
		})
	}
	return result
}
