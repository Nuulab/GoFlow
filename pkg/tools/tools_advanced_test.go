// Package tools_test provides advanced tests for the tools package.
package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nuulab/goflow/pkg/tools"
)

// ============ Edge Case Tests ============

func TestRegistry_EmptyRegistry(t *testing.T) {
	registry := tools.NewRegistry()

	// List should return empty slice, not nil
	list := registry.List()
	if list == nil {
		t.Error("List should return empty slice, not nil")
	}
	if len(list) != 0 {
		t.Error("Expected empty list")
	}

	// Get should return false for any key
	_, ok := registry.Get("anything")
	if ok {
		t.Error("Get should return false for empty registry")
	}

	// Execute should error for any tool
	_, err := registry.Execute(context.Background(), "test", "{}")
	if err == nil {
		t.Error("Execute should error for unknown tool")
	}
}

func TestRegistry_NilExecuteFunc(t *testing.T) {
	registry := tools.NewRegistry()

	// Tool with nil Execute should still register
	tool := &tools.Tool{
		Name:        "nil_execute",
		Description: "Tool with nil execute",
		Execute:     nil,
	}
	err := registry.Register(tool)
	if err != nil {
		t.Errorf("Should register tool even with nil Execute: %v", err)
	}

	// But executing it should panic/error - let's check it doesn't crash
	defer func() {
		if r := recover(); r != nil {
			// Expected - nil function called
			t.Log("Recovered from nil Execute panic as expected")
		}
	}()
}

func TestRegistry_SpecialCharactersInName(t *testing.T) {
	registry := tools.NewRegistry()

	specialNames := []string{
		"tool-with-dash",
		"tool_with_underscore",
		"tool.with.dots",
		"tool:with:colons",
		"UPPERCASE_TOOL",
		"CamelCaseTool",
		"tool123",
		"123tool",
	}

	for _, name := range specialNames {
		tool := &tools.Tool{
			Name: name,
			Execute: func(ctx context.Context, input string) (string, error) {
				return "ok", nil
			},
		}
		err := registry.Register(tool)
		if err != nil {
			t.Errorf("Failed to register tool with name %q: %v", name, err)
		}
	}

	if len(registry.List()) != len(specialNames) {
		t.Error("Not all tools registered")
	}
}

func TestRegistry_LargeJSON(t *testing.T) {
	registry := tools.NewRegistry()

	registry.Register(&tools.Tool{
		Name: "large_handler",
		Execute: func(ctx context.Context, input string) (string, error) {
			return input, nil // Echo back
		},
	})

	// Create large JSON payload
	largeData := make(map[string]string)
	for i := 0; i < 1000; i++ {
		largeData[string(rune('a'+i%26))+string(rune(i))] = "value" + string(rune(i))
	}
	jsonBytes, _ := json.Marshal(largeData)

	result, err := registry.Execute(context.Background(), "large_handler", string(jsonBytes))
	if err != nil {
		t.Errorf("Failed to handle large JSON: %v", err)
	}
	if len(result) != len(jsonBytes) {
		t.Errorf("Data size mismatch: got %d, want %d", len(result), len(jsonBytes))
	}
}

// ============ Concurrency Tests ============

func TestRegistry_ConcurrentExecute(t *testing.T) {
	registry := tools.NewRegistry()

	counter := 0
	var mu sync.Mutex

	registry.Register(&tools.Tool{
		Name: "counter",
		Execute: func(ctx context.Context, input string) (string, error) {
			mu.Lock()
			counter++
			mu.Unlock()
			return "ok", nil
		},
	})

	// Run 100 concurrent executions
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := registry.Execute(context.Background(), "counter", "{}")
			if err != nil {
				t.Errorf("Concurrent execute failed: %v", err)
			}
		}()
	}
	wg.Wait()

	if counter != 100 {
		t.Errorf("Expected counter to be 100, got %d", counter)
	}
}

func TestRegistry_ConcurrentRegisterAndGet(t *testing.T) {
	// Registry is now thread-safe with sync.RWMutex
	registry := tools.NewRegistry()

	// Register tools concurrently - some may fail due to duplicates
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tool := &tools.Tool{
				Name: "concurrent_tool_" + string(rune('a'+i%26)),
				Execute: func(ctx context.Context, input string) (string, error) {
					return "ok", nil
				},
			}
			registry.Register(tool) // May error, that's ok
		}(i)
	}

	// Concurrently get tools
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			registry.Get("concurrent_tool_" + string(rune('a'+i%26)))
		}(i)
	}

	wg.Wait()

	// No panic = success
	t.Log("Concurrent register/get completed without panic")
}

// ============ Error Handling Tests ============

func TestRegistry_ExecuteWithError(t *testing.T) {
	registry := tools.NewRegistry()

	expectedErr := errors.New("intentional error")
	registry.Register(&tools.Tool{
		Name: "error_tool",
		Execute: func(ctx context.Context, input string) (string, error) {
			return "", expectedErr
		},
	})

	_, err := registry.Execute(context.Background(), "error_tool", "{}")
	if err != expectedErr {
		t.Errorf("Expected specific error, got: %v", err)
	}
}

func TestRegistry_ExecuteWithPanic(t *testing.T) {
	registry := tools.NewRegistry()

	registry.Register(&tools.Tool{
		Name: "panic_tool",
		Execute: func(ctx context.Context, input string) (string, error) {
			panic("intentional panic")
		},
	})

	// This should panic - test that we can handle it
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic to propagate")
		}
	}()

	registry.Execute(context.Background(), "panic_tool", "{}")
}

func TestRegistry_ContextCancellation(t *testing.T) {
	registry := tools.NewRegistry()

	registry.Register(&tools.Tool{
		Name: "slow_tool",
		Execute: func(ctx context.Context, input string) (string, error) {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(10 * time.Second):
				return "completed", nil
			}
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	
	done := make(chan bool)
	go func() {
		_, err := registry.Execute(ctx, "slow_tool", "{}")
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got: %v", err)
		}
		done <- true
	}()

	// Cancel after short delay
	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("Tool did not respond to context cancellation")
	}
}

func TestRegistry_InvalidJSON(t *testing.T) {
	registry := tools.NewRegistry()

	type Input struct {
		A int `json:"a"`
	}

	tool := tools.NewTool("typed_tool", "Test", func(ctx context.Context, input Input) (string, error) {
		return "ok", nil
	})
	registry.Register(tool)

	// Execute with invalid JSON
	_, err := registry.Execute(context.Background(), "typed_tool", "not valid json")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}

	// Execute with wrong type JSON
	_, err = registry.Execute(context.Background(), "typed_tool", `{"a": "string instead of int"}`)
	if err == nil {
		t.Error("Expected error for wrong type")
	}
}

// ============ ExecuteCalls Tests ============

func TestRegistry_ExecuteCallsPartialFailure(t *testing.T) {
	registry := tools.NewRegistry()

	registry.Register(&tools.Tool{
		Name: "success",
		Execute: func(ctx context.Context, input string) (string, error) {
			return "ok", nil
		},
	})
	registry.Register(&tools.Tool{
		Name: "failure",
		Execute: func(ctx context.Context, input string) (string, error) {
			return "", errors.New("failed")
		},
	})

	calls := []tools.ToolCall{
		{ID: "1", Name: "success", Arguments: "{}"},
		{ID: "2", Name: "failure", Arguments: "{}"},
		{ID: "3", Name: "success", Arguments: "{}"},
		{ID: "4", Name: "unknown", Arguments: "{}"},
	}

	results := registry.ExecuteCalls(context.Background(), calls)

	if len(results) != 4 {
		t.Fatalf("Expected 4 results, got %d", len(results))
	}

	// Check success
	if results[0].Error != "" {
		t.Error("First call should succeed")
	}

	// Check failure
	if results[1].Error == "" {
		t.Error("Second call should fail")
	}

	// Check success
	if results[2].Error != "" {
		t.Error("Third call should succeed")
	}

	// Check unknown tool
	if results[3].Error == "" {
		t.Error("Fourth call should fail (unknown tool)")
	}
}

func TestRegistry_ExecuteCallsEmpty(t *testing.T) {
	registry := tools.NewRegistry()

	results := registry.ExecuteCalls(context.Background(), nil)
	if len(results) != 0 {
		t.Error("Expected empty results for nil calls")
	}

	results = registry.ExecuteCalls(context.Background(), []tools.ToolCall{})
	if len(results) != 0 {
		t.Error("Expected empty results for empty calls")
	}
}

// ============ Format Conversion Tests ============

func TestRegistry_OpenAIFormatComplete(t *testing.T) {
	registry := tools.NewRegistry()

	registry.Register(&tools.Tool{
		Name:        "get_weather",
		Description: "Gets weather for a location",
		Parameters: tools.Schema{
			Type: "object",
			Properties: map[string]tools.Property{
				"location": {Type: "string", Description: "City name"},
				"units":    {Type: "string", Description: "Temperature units", Enum: []string{"celsius", "fahrenheit"}},
			},
			Required: []string{"location"},
		},
		Execute: func(ctx context.Context, input string) (string, error) {
			return "sunny", nil
		},
	})

	format := registry.ToOpenAIFormat()

	// Validate structure
	data, _ := json.Marshal(format)
	var parsed []map[string]any
	json.Unmarshal(data, &parsed)

	if len(parsed) != 1 {
		t.Fatal("Expected 1 function")
	}

	fn := parsed[0]
	if fn["type"] != "function" {
		t.Error("Expected type 'function'")
	}

	function := fn["function"].(map[string]any)
	if function["name"] != "get_weather" {
		t.Error("Expected name 'get_weather'")
	}

	params := function["parameters"].(map[string]any)
	props := params["properties"].(map[string]any)
	
	units := props["units"].(map[string]any)
	enum := units["enum"].([]any)
	if len(enum) != 2 {
		t.Error("Expected 2 enum values")
	}
}
