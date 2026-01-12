# Getting Started

Get up and running with GoFlow in minutes.

## Prerequisites

- Go 1.21 or later
- (Optional) Redis or DragonflyDB for persistence

## Installation

```bash
go get github.com/goflow/goflow
```

This adds GoFlow as a dependency to your Go module. The package provides all core functionality including agents, workflows, queues, and tools.

## Your First Agent

Create a simple AI agent that can use tools:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/goflow/goflow/pkg/agent"
    "github.com/goflow/goflow/pkg/tools"
    "github.com/goflow/goflow/pkg/llm/openai"
)

func main() {
    // 1. Create an LLM client
    llm := openai.New(os.Getenv("OPENAI_API_KEY"))

    // 2. Create a tool registry with built-in tools
    registry := tools.BuiltinTools()
    registry.Register(tools.CalculatorTool())

    // 3. Create the agent
    myAgent := agent.New(llm, registry,
        agent.WithMaxIterations(10),
    )

    // 4. Run a task
    result, err := myAgent.Run(context.Background(), "What is 25 * 4?")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Result:", result.Output)
    fmt.Println("Steps:", len(result.Steps))
}
```

**What this code does:**

1. **LLM Client** - Creates a connection to OpenAI. This is the "brain" that powers the agent's reasoning.

2. **Tool Registry** - Sets up the tools the agent can use. `BuiltinTools()` includes common utilities, and we add a calculator for math operations.

3. **Agent Creation** - Combines the LLM and tools into an agent. `WithMaxIterations(10)` prevents infinite loops by limiting reasoning steps.

4. **Running a Task** - The agent receives the task, thinks about it, uses tools as needed, and returns the final answer.

## Run as a Service

Start GoFlow as a standalone service:

```bash
# Build
go build -o goflow ./cmd/server

# Run
./goflow -port 8080
```

This starts the API server with REST endpoints and WebSocket support for real-time events.

Then use the JavaScript SDK:

```typescript
import { GoFlowClient } from '@goflow/client'

const client = new GoFlowClient('http://localhost:8080')
const result = await client.agent('my-agent').run('Hello, GoFlow!')
```

The SDK connects to your running server and provides a clean TypeScript interface. The `agent().run()` method sends the task and waits for the result.

## Webhooks

Trigger jobs via webhooks from external services:

```go
webhook := webhook.NewWebhookHandler(queue, engine)

// Register a webhook that enqueues jobs
webhook.RegisterJobWebhook("/github", "process_github_event")

// Register a webhook that starts workflows
webhook.RegisterWorkflowWebhook("/order", "order-process")

http.Handle("/webhooks/", webhook.Handler())
```

External services (GitHub, Stripe, etc.) can now POST to your webhook endpoints:
- `POST /webhooks/github` → Enqueues a `process_github_event` job
- `POST /webhooks/order` → Starts the `order-process` workflow

## Next Steps

- [Installation](/guide/installation) - Detailed setup guide
- [Agents](/guide/agents) - Deep dive into agent capabilities
- [Workflows](/guide/workflows) - Build complex workflows
- [Webhooks](/guide/webhooks) - Trigger jobs from external services
- [Examples](/examples/basic-agent) - See complete examples
