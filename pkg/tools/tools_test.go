// Package tools_test provides comprehensive tests for the tools package.
package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nuulab/goflow/pkg/tools"
)

func TestRegistry_Register(t *testing.T) {
	registry := tools.NewRegistry()

	// Test successful registration
	tool := &tools.Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Execute: func(ctx context.Context, input string) (string, error) {
			return "success", nil
		},
	}

	err := registry.Register(tool)
	if err != nil {
		t.Errorf("Register failed: %v", err)
	}

	// Test duplicate registration
	err = registry.Register(tool)
	if err == nil {
		t.Error("Expected error for duplicate registration")
	}

	// Test empty name
	emptyTool := &tools.Tool{
		Name: "",
	}
	err = registry.Register(emptyTool)
	if err == nil {
		t.Error("Expected error for empty tool name")
	}
}

func TestRegistry_Get(t *testing.T) {
	registry := tools.NewRegistry()

	tool := &tools.Tool{
		Name:        "get_test",
		Description: "Test tool for Get",
		Execute: func(ctx context.Context, input string) (string, error) {
			return "found", nil
		},
	}
	registry.Register(tool)

	// Test successful get
	found, ok := registry.Get("get_test")
	if !ok {
		t.Error("Expected to find registered tool")
	}
	if found.Name != "get_test" {
		t.Errorf("Got wrong tool: %s", found.Name)
	}

	// Test non-existent tool
	_, ok = registry.Get("nonexistent")
	if ok {
		t.Error("Expected not to find unregistered tool")
	}
}

func TestRegistry_List(t *testing.T) {
	registry := tools.NewRegistry()

	// Empty registry
	list := registry.List()
	if len(list) != 0 {
		t.Error("Expected empty list")
	}

	// Add tools
	for i := 0; i < 3; i++ {
		registry.Register(&tools.Tool{
			Name: "tool_" + string(rune('a'+i)),
			Execute: func(ctx context.Context, input string) (string, error) {
				return "", nil
			},
		})
	}

	list = registry.List()
	if len(list) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(list))
	}
}

func TestRegistry_Execute(t *testing.T) {
	registry := tools.NewRegistry()

	registry.Register(&tools.Tool{
		Name:        "echo",
		Description: "Echoes the input",
		Execute: func(ctx context.Context, input string) (string, error) {
			var data struct {
				Message string `json:"message"`
			}
			json.Unmarshal([]byte(input), &data)
			return data.Message, nil
		},
	})

	ctx := context.Background()

	// Test successful execution
	result, err := registry.Execute(ctx, "echo", `{"message":"hello world"}`)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}
	if result != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", result)
	}

	// Test unknown tool
	_, err = registry.Execute(ctx, "unknown", "{}")
	if err == nil {
		t.Error("Expected error for unknown tool")
	}
}

func TestRegistry_ExecuteCalls(t *testing.T) {
	registry := tools.NewRegistry()

	registry.Register(&tools.Tool{
		Name: "add",
		Execute: func(ctx context.Context, input string) (string, error) {
			var data struct {
				A int `json:"a"`
				B int `json:"b"`
			}
			json.Unmarshal([]byte(input), &data)
			return string(rune(data.A + data.B + '0')), nil
		},
	})

	calls := []tools.ToolCall{
		{ID: "1", Name: "add", Arguments: `{"a":2,"b":3}`},
		{ID: "2", Name: "add", Arguments: `{"a":5,"b":4}`},
	}

	ctx := context.Background()
	results := registry.ExecuteCalls(ctx, calls)

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	if results[0].ToolCallID != "1" {
		t.Error("Wrong tool call ID")
	}
}

func TestRegistry_ToOpenAIFormat(t *testing.T) {
	registry := tools.NewRegistry()

	registry.Register(&tools.Tool{
		Name:        "weather",
		Description: "Get weather for a location",
		Parameters: tools.Schema{
			Type: "object",
			Properties: map[string]tools.Property{
				"location": {Type: "string", Description: "City name"},
			},
			Required: []string{"location"},
		},
		Execute: func(ctx context.Context, input string) (string, error) {
			return "sunny", nil
		},
	})

	format := registry.ToOpenAIFormat()
	if len(format) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(format))
	}

	if format[0]["type"] != "function" {
		t.Error("Expected type 'function'")
	}
}

func TestRegistry_ToAnthropicFormat(t *testing.T) {
	registry := tools.NewRegistry()

	registry.Register(&tools.Tool{
		Name:        "calculator",
		Description: "Performs calculations",
		Parameters: tools.Schema{
			Type: "object",
			Properties: map[string]tools.Property{
				"expression": {Type: "string"},
			},
		},
		Execute: func(ctx context.Context, input string) (string, error) {
			return "42", nil
		},
	})

	format := registry.ToAnthropicFormat()
	if len(format) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(format))
	}

	if format[0]["name"] != "calculator" {
		t.Error("Expected name 'calculator'")
	}

	if format[0]["input_schema"] == nil {
		t.Error("Expected input_schema")
	}
}

// Test typed tool creation
type AddInput struct {
	A int `json:"a"`
	B int `json:"b"`
}

type AddOutput struct {
	Sum int `json:"sum"`
}

func TestNewTool(t *testing.T) {
	tool := tools.NewTool("add", "Adds two numbers", func(ctx context.Context, input AddInput) (AddOutput, error) {
		return AddOutput{Sum: input.A + input.B}, nil
	})

	if tool.Name != "add" {
		t.Errorf("Expected name 'add', got '%s'", tool.Name)
	}

	// Execute the tool
	result, err := tool.Execute(context.Background(), `{"a":5,"b":3}`)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	var output AddOutput
	json.Unmarshal([]byte(result), &output)
	if output.Sum != 8 {
		t.Errorf("Expected sum 8, got %d", output.Sum)
	}
}
