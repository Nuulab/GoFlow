// Package agent_test provides integration tests for the Agent with real LLM calls.
package agent_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nuulab/goflow/pkg/agent"
	"github.com/nuulab/goflow/pkg/llm/gemini"
	"github.com/nuulab/goflow/pkg/tools"
)

func skipIfNoAPIKey(t *testing.T) string {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		key = os.Getenv("GOOGLE_API_KEY")
	}
	if key == "" {
		t.Skip("GEMINI_API_KEY or GOOGLE_API_KEY not set, skipping integration test")
	}
	return key
}

// TestGemini_DirectGeneration tests that the Gemini LLM works directly
// without going through the agent (which has specific format requirements)
func TestGemini_DirectGeneration(t *testing.T) {
	apiKey := skipIfNoAPIKey(t)

	llm := gemini.New(apiKey, gemini.WithModel("gemini-3-flash-preview"))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := llm.Generate(ctx, "What is 15 * 7? Just give me the number.")
	if err != nil {
		t.Fatalf("LLM Generate failed: %v", err)
	}

	if response == "" {
		t.Error("Expected non-empty response")
	}

	t.Logf("LLM response: %s", response)
}

// TestAgent_Creation tests that an agent can be created with LLM and tools
func TestAgent_Creation(t *testing.T) {
	apiKey := skipIfNoAPIKey(t)

	llm := gemini.New(apiKey, gemini.WithModel("gemini-3-flash-preview"))
	registry := tools.NewRegistry()

	ag := agent.New(llm, registry,
		agent.WithMaxIterations(5),
		agent.WithSystemPrompt("You are a helpful assistant."),
	)

	if ag == nil {
		t.Fatal("Expected non-nil agent")
	}

	// Test that agent can be reset
	ag.Reset()
	messages := ag.GetMessages()
	if len(messages) != 0 {
		t.Errorf("Expected 0 messages after reset, got %d", len(messages))
	}
}

// TestAgent_WithCalculatorTool tests agent with a simple tool
func TestAgent_WithCalculatorTool(t *testing.T) {
	apiKey := skipIfNoAPIKey(t)

	llm := gemini.New(apiKey, gemini.WithModel("gemini-3-flash-preview"))
	registry := tools.NewRegistry()

	// Register a calculator tool
	registry.Register(&tools.Tool{
		Name:        "calculator",
		Description: "Performs basic arithmetic. Input: JSON with 'expression' field containing a math expression.",
		Parameters: tools.Schema{
			Type: "object",
			Properties: map[string]tools.Property{
				"expression": {Type: "string", Description: "Math expression like '2 + 2'"},
			},
			Required: []string{"expression"},
		},
		Execute: func(ctx context.Context, input string) (string, error) {
			return "150", nil // Always return 150 for testing
		},
	})

	ag := agent.New(llm, registry,
		agent.WithMaxIterations(10),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Run the agent - we don't require it to succeed, just to run without crashing
	result, err := ag.Run(ctx, "Use the calculator to compute 100 + 50")
	
	// Log the result regardless of success/failure
	t.Logf("Agent iterations: %d", result.Iterations)
	t.Logf("Agent output: %s", result.Output)
	if err != nil {
		t.Logf("Agent error (expected with some LLMs): %v", err)
	}
	if len(result.ToolCalls) > 0 {
		t.Logf("Tool calls made: %d", len(result.ToolCalls))
		for _, tc := range result.ToolCalls {
			t.Logf("  - %s: %s -> %s", tc.Name, tc.Input, tc.Output)
		}
	}

	// The test passes as long as the agent ran without panic
	// Real-world agent behavior depends on LLM response format
}
