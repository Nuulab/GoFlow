---
title: Basic Agent
---


A simple agent that can use tools.

## Code

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/nuulab/goflow/pkg/agent"
	"github.com/nuulab/goflow/pkg/llm/openai"
	"github.com/nuulab/goflow/pkg/tools"
)

func main() {
	// Create LLM client
	llm := openai.New(os.Getenv("OPENAI_API_KEY"))

	// Create tool registry
	registry := tools.NewRegistry()
	registry.Register(tools.CalculatorTool())
	registry.Register(tools.WebSearchTool())

	// Create agent with hooks for logging
	hooks := agent.NewHooksBuilder().
		OnStart(func(task string) {
			fmt.Println("🚀 Starting:", task)
		}).
		OnToolCall(func(tool, input string) {
			fmt.Printf("🔧 Using %s: %s\n", tool, input)
		}).
		OnComplete(func(result *agent.RunResult) {
			fmt.Printf("✅ Done in %d steps\n", len(result.Steps))
		}).
		Build()

	myAgent := agent.New(llm, registry,
		agent.WithMaxIterations(10),
		agent.WithHooks(hooks),
	)

	// Run a task
	result, err := myAgent.Run(context.Background(), 
		"What is 25 * 4? Then search for Go programming tutorials.")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n📝 Result:")
	fmt.Println(result.Output)
}
```

## Output

```
🚀 Starting: What is 25 * 4? Then search for Go programming tutorials.
🔧 Using calculator: 25 * 4
🔧 Using web_search: Go programming tutorials
✅ Done in 3 steps

📝 Result:
25 * 4 = 100

Here are some Go programming tutorials:
1. A Tour of Go - https://go.dev/tour/
2. Go by Example - https://gobyexample.com/
3. Learn Go - https://learn.go.dev/
```

## Key Points

- Create an LLM client (OpenAI, Anthropic, etc.)
- Register tools the agent can use
- Add hooks to monitor execution
- Run tasks with `agent.Run()`
