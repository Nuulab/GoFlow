---
title: Agents
---


Agents are the core building block of GoFlow. An agent takes a task, reasons about it, uses tools, and returns a result.

## Basic Agent

```go
import (
    "github.com/nuulab/goflow/pkg/agent"
    "github.com/nuulab/goflow/pkg/tools"
)

// Create with defaults
myAgent := agent.New(llm, tools.BuiltinTools())

// Run a task
result, err := myAgent.Run(ctx, "What is the weather in Tokyo?")
```

The `agent.New()` function takes an LLM (the AI model) and a tool registry (what the agent can do). The agent then uses a ReAct-style loop: **Reason → Act → Observe → Repeat** until it has the answer.

## Configuration

```go
myAgent := agent.New(llm, registry,
    agent.WithMaxIterations(20),       // Max reasoning steps
    agent.WithTimeout(5*time.Minute),  // Overall timeout
    agent.WithVerbose(true),           // Log each step
    agent.WithSystemPrompt("You are a helpful assistant"),
)
```

Each option customizes agent behavior:

| Option | What it does |
|--------|-------------|
| `WithMaxIterations` | Prevents infinite loops by capping reasoning steps |
| `WithTimeout` | Kills the agent if it takes too long |
| `WithVerbose` | Prints each thought/action for debugging |
| `WithSystemPrompt` | Sets the agent's personality and instructions |

## Agent Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithMaxIterations` | Maximum reasoning steps | 10 |
| `WithTimeout` | Overall execution timeout | 5m |
| `WithVerbose` | Enable step logging | false |
| `WithSystemPrompt` | Custom system prompt | (built-in) |
| `WithMemory` | Enable memory/context | nil |

## Lifecycle Hooks

Monitor agent execution with hooks:

```go
hooks := agent.NewHooksBuilder().
    OnStart(func(task string) {
        log.Println("Starting:", task)
    }).
    OnToolCall(func(tool, input string) {
        log.Println("Calling tool:", tool)
    }).
    OnComplete(func(result *agent.RunResult) {
        log.Println("Completed in", len(result.Steps), "steps")
    }).
    Build()

myAgent := agent.New(llm, registry, agent.WithHooks(hooks))
```

Hooks let you observe and log everything the agent does without modifying its behavior. This is invaluable for debugging and monitoring in production.

## Result Structure

```go
type RunResult struct {
    Output    string      // Final answer
    Steps     []Step      // All reasoning steps
    ToolCalls []ToolCall  // Tools used
    Duration  time.Duration
    Error     error
}

type Step struct {
    Thought  string
    Action   string
    Input    string
    Output   string
}
```

The `RunResult` gives you complete visibility into the agent's work:
- **Output** - The final answer to present to the user
- **Steps** - Every thought and action the agent took
- **ToolCalls** - Which tools were invoked and with what inputs
- **Duration** - Total execution time for performance monitoring

## Streaming

Stream agent output in real-time:

```go
stream, _ := myAgent.Stream(ctx, "Write a story about a robot")

for chunk := range stream {
    fmt.Print(chunk)
}
```

Streaming is essential for responsive UIs. Instead of waiting for the complete response, you display tokens as they arrive, making the agent feel faster and more interactive.

## Agent with Memory

Persist context across runs:

```go
memory := agent.NewConversationMemory(10) // Keep last 10 exchanges

myAgent := agent.New(llm, registry,
    agent.WithMemory(memory),
)

// First run
myAgent.Run(ctx, "My name is Alice")

// Second run - agent remembers
myAgent.Run(ctx, "What's my name?") // "Your name is Alice"
```

Without memory, each `Run()` is independent. With memory, the agent builds up context over multiple interactions, enabling conversational experiences.

## Custom Agent Types

Create specialized agents:

```go
// Research agent with web tools
researchAgent := agent.New(llm, tools.WebToolkit(),
    agent.WithSystemPrompt("You are a research assistant..."),
)

// Code agent with shell tools
codeAgent := agent.New(llm, tools.ShellToolkit(),
    agent.WithSystemPrompt("You are a coding assistant..."),
)
```

Different agents can have different tools and prompts. A research agent might have web search capabilities, while a code agent might have file system access. Keep agents focused on specific domains for better results.
