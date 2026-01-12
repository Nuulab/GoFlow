// Package tools provides built-in tools for agent execution.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// BuiltinTools returns a registry with commonly used built-in tools.
func BuiltinTools() *Registry {
	registry := NewRegistry()

	registry.Register(FinalAnswerTool())
	registry.Register(ThinkTool())

	return registry
}

// FinalAnswerTool signals that the agent has completed its task.
func FinalAnswerTool() *Tool {
	return &Tool{
		Name:        "final_answer",
		Description: "Use this tool when you have the final answer to the user's question. The input should be your complete final response.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Property{
				"answer": {
					Type:        "string",
					Description: "The final answer to provide to the user",
				},
			},
			Required: []string{"answer"},
		},
		Execute: func(ctx context.Context, jsonInput string) (string, error) {
			var input struct {
				Answer string `json:"answer"`
			}
			if err := json.Unmarshal([]byte(jsonInput), &input); err != nil {
				// Try to use raw input as answer
				return jsonInput, nil
			}
			return input.Answer, nil
		},
	}
}

// ThinkTool provides a scratchpad for agent reasoning.
func ThinkTool() *Tool {
	return &Tool{
		Name:        "think",
		Description: "Use this tool to think through a problem step by step. Write out your reasoning process. This helps with complex problems.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Property{
				"thought": {
					Type:        "string",
					Description: "Your step-by-step reasoning",
				},
			},
			Required: []string{"thought"},
		},
		Execute: func(ctx context.Context, jsonInput string) (string, error) {
			var input struct {
				Thought string `json:"thought"`
			}
			if err := json.Unmarshal([]byte(jsonInput), &input); err != nil {
				return "Thought recorded.", nil
			}
			return fmt.Sprintf("Thought recorded: %s", input.Thought), nil
		},
	}
}

// AskHumanTool requests input from a human operator.
func AskHumanTool(inputFn func(question string) (string, error)) *Tool {
	return &Tool{
		Name:        "ask_human",
		Description: "Use this tool when you need to ask the human user for clarification or additional information.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Property{
				"question": {
					Type:        "string",
					Description: "The question to ask the human",
				},
			},
			Required: []string{"question"},
		},
		Execute: func(ctx context.Context, jsonInput string) (string, error) {
			var input struct {
				Question string `json:"question"`
			}
			if err := json.Unmarshal([]byte(jsonInput), &input); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}

			if inputFn == nil {
				return "", fmt.Errorf("no human input function configured")
			}

			return inputFn(input.Question)
		},
	}
}

// CalculatorTool performs basic arithmetic operations.
func CalculatorTool() *Tool {
	return &Tool{
		Name:        "calculator",
		Description: "Performs basic arithmetic calculations. Supports add, subtract, multiply, divide operations.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Property{
				"operation": {
					Type:        "string",
					Description: "The operation to perform",
					Enum:        []string{"add", "subtract", "multiply", "divide"},
				},
				"a": {
					Type:        "number",
					Description: "First number",
				},
				"b": {
					Type:        "number",
					Description: "Second number",
				},
			},
			Required: []string{"operation", "a", "b"},
		},
		Execute: func(ctx context.Context, jsonInput string) (string, error) {
			var input struct {
				Operation string  `json:"operation"`
				A         float64 `json:"a"`
				B         float64 `json:"b"`
			}
			if err := json.Unmarshal([]byte(jsonInput), &input); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}

			var result float64
			switch input.Operation {
			case "add":
				result = input.A + input.B
			case "subtract":
				result = input.A - input.B
			case "multiply":
				result = input.A * input.B
			case "divide":
				if input.B == 0 {
					return "", fmt.Errorf("division by zero")
				}
				result = input.A / input.B
			default:
				return "", fmt.Errorf("unknown operation: %s", input.Operation)
			}

			return fmt.Sprintf("%.6g", result), nil
		},
	}
}

// SearchTool is a template for implementing search functionality.
func SearchTool(searchFn func(ctx context.Context, query string) (string, error)) *Tool {
	return &Tool{
		Name:        "search",
		Description: "Search for information. Use this when you need to find facts or look up information.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Property{
				"query": {
					Type:        "string",
					Description: "The search query",
				},
			},
			Required: []string{"query"},
		},
		Execute: func(ctx context.Context, jsonInput string) (string, error) {
			var input struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal([]byte(jsonInput), &input); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}

			if searchFn == nil {
				return "", fmt.Errorf("no search function configured")
			}

			return searchFn(ctx, input.Query)
		},
	}
}

// WebFetchTool fetches content from a URL.
func WebFetchTool(fetchFn func(ctx context.Context, url string) (string, error)) *Tool {
	return &Tool{
		Name:        "web_fetch",
		Description: "Fetch content from a web URL. Returns the text content of the page.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Property{
				"url": {
					Type:        "string",
					Description: "The URL to fetch",
				},
			},
			Required: []string{"url"},
		},
		Execute: func(ctx context.Context, jsonInput string) (string, error) {
			var input struct {
				URL string `json:"url"`
			}
			if err := json.Unmarshal([]byte(jsonInput), &input); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}

			if fetchFn == nil {
				return "", fmt.Errorf("no fetch function configured")
			}

			return fetchFn(ctx, input.URL)
		},
	}
}

// CodeExecutorTool executes code snippets (use with caution!).
func CodeExecutorTool(execFn func(ctx context.Context, language, code string) (string, error)) *Tool {
	return &Tool{
		Name:        "execute_code",
		Description: "Execute code in a sandboxed environment. Use for calculations or data processing.",
		Parameters: Schema{
			Type: "object",
			Properties: map[string]Property{
				"language": {
					Type:        "string",
					Description: "Programming language",
					Enum:        []string{"python", "javascript"},
				},
				"code": {
					Type:        "string",
					Description: "The code to execute",
				},
			},
			Required: []string{"language", "code"},
		},
		Execute: func(ctx context.Context, jsonInput string) (string, error) {
			var input struct {
				Language string `json:"language"`
				Code     string `json:"code"`
			}
			if err := json.Unmarshal([]byte(jsonInput), &input); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}

			if execFn == nil {
				return "", fmt.Errorf("no code executor configured")
			}

			return execFn(ctx, input.Language, input.Code)
		},
	}
}
