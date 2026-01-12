# LLM Providers

GoFlow supports multiple Large Language Model providers out of the box.

## Supported Providers

| Provider | Package | Default Model | Streaming |
|----------|---------|---------------|-----------|
| **OpenAI** | `pkg/llm/openai` | `gpt-4o` | ✅ |
| **Anthropic Claude** | `pkg/llm/anthropic` | `claude-3-5-sonnet-20241022` | ✅ |
| **Google Gemini** | `pkg/llm/gemini` | `gemini-1.5-flash` | ✅ |

## Environment Variables

Set your API keys before running:

```bash
# OpenAI
export OPENAI_API_KEY="sk-..."

# Anthropic
export ANTHROPIC_API_KEY="sk-ant-..."

# Google Gemini (either works)
export GOOGLE_API_KEY="AIza..."
export GEMINI_API_KEY="AIza..."
```

## Quick Start

```go
import (
    "github.com/goflow/goflow/pkg/llm/openai"
    "github.com/goflow/goflow/pkg/llm/anthropic"
    "github.com/goflow/goflow/pkg/llm/gemini"
)

// OpenAI - reads OPENAI_API_KEY from env
llm := openai.New("")

// Anthropic Claude - reads ANTHROPIC_API_KEY from env
llm := anthropic.New("")

// Google Gemini - reads GOOGLE_API_KEY from env
llm := gemini.New("")
```

---

## OpenAI

### Supported Models

| Model | Context | Best For |
|-------|---------|----------|
| `gpt-4o` | 128K | Most capable, multimodal |
| `gpt-4o-mini` | 128K | Fast, cost-effective |
| `gpt-4-turbo` | 128K | Previous flagship |
| `gpt-3.5-turbo` | 16K | Budget option |
| `o1-preview` | 128K | Advanced reasoning |
| `o1-mini` | 128K | Fast reasoning |

### Usage

```go
import "github.com/goflow/goflow/pkg/llm/openai"

// Default: gpt-4o
llm := openai.New("")

// Specify model
llm := openai.New("", openai.WithModel("gpt-4o-mini"))

// With options
llm := openai.New("sk-your-key",
    openai.WithModel("gpt-4o"),
    openai.WithTimeout(60*time.Second),
)

// Azure OpenAI
llm := openai.New("your-azure-key",
    openai.WithBaseURL("https://your-resource.openai.azure.com/openai/deployments/gpt-4o"),
)
```

### Generate

```go
// Simple completion
response, err := llm.Generate(ctx, "What is Go?")

// Chat completion
response, err := llm.GenerateChat(ctx, []core.Message{
    {Role: core.RoleSystem, Content: "You are a helpful assistant."},
    {Role: core.RoleUser, Content: "What is Go?"},
})

// With options
response, err := llm.Generate(ctx, "Tell me a joke",
    core.WithTemperature(0.9),
    core.WithMaxTokens(100),
)
```

### Streaming

```go
// Stream response
stream, err := llm.Stream(ctx, "Write a story about a robot")
for chunk := range stream {
    fmt.Print(chunk)
}

// Stream chat
stream, err := llm.StreamChat(ctx, messages)
for chunk := range stream {
    fmt.Print(chunk)
}
```

---

## Anthropic Claude

### Supported Models

| Model | Context | Best For |
|-------|---------|----------|
| `claude-3-5-sonnet-20241022` | 200K | Best balance |
| `claude-3-5-haiku-20241022` | 200K | Fast, affordable |
| `claude-3-opus-20240229` | 200K | Most capable |
| `claude-3-sonnet-20240229` | 200K | Previous generation |
| `claude-3-haiku-20240307` | 200K | Previous fast model |

### Usage

```go
import "github.com/goflow/goflow/pkg/llm/anthropic"

// Default: claude-3-5-sonnet
llm := anthropic.New("")

// Specify model
llm := anthropic.New("", anthropic.WithModel("claude-3-5-haiku-20241022"))

// With options
llm := anthropic.New("sk-ant-your-key",
    anthropic.WithModel("claude-3-opus-20240229"),
    anthropic.WithTimeout(120*time.Second),
)
```

### System Prompts

Anthropic handles system prompts separately. GoFlow automatically extracts them:

```go
response, err := llm.GenerateChat(ctx, []core.Message{
    {Role: core.RoleSystem, Content: "You are a pirate."},  // Automatically extracted
    {Role: core.RoleUser, Content: "Hello!"},
})
// Response: "Ahoy, matey!"
```

---

## Google Gemini

### Supported Models

| Model | Context | Best For |
|-------|---------|----------|
| `gemini-1.5-pro` | 2M | Most capable, huge context |
| `gemini-1.5-flash` | 1M | Fast, large context |
| `gemini-1.5-flash-8b` | 1M | Fastest, most affordable |
| `gemini-1.0-pro` | 32K | Previous generation |

### Usage

```go
import "github.com/goflow/goflow/pkg/llm/gemini"

// Default: gemini-1.5-flash
llm := gemini.New("")

// Specify model
llm := gemini.New("", gemini.WithModel("gemini-1.5-pro"))

// With options
llm := gemini.New("AIza-your-key",
    gemini.WithModel("gemini-1.5-pro"),
    gemini.WithTimeout(120*time.Second),
)
```

---

## Configuration Options

All providers support these options:

| Option | Description | Example |
|--------|-------------|---------|
| `core.WithTemperature(f)` | Creativity (0.0-2.0) | `core.WithTemperature(0.7)` |
| `core.WithMaxTokens(n)` | Max output tokens | `core.WithMaxTokens(1000)` |
| `core.WithTopP(f)` | Nucleus sampling | `core.WithTopP(0.9)` |
| `core.WithStopSequences(s...)` | Stop generation | `core.WithStopSequences("\n\n")` |

```go
response, err := llm.Generate(ctx, prompt,
    core.WithTemperature(0.7),
    core.WithMaxTokens(500),
    core.WithTopP(0.9),
)
```

---

## Using with Agents

```go
import (
    "github.com/goflow/goflow/pkg/agent"
    "github.com/goflow/goflow/pkg/llm/openai"
    "github.com/goflow/goflow/pkg/tools"
)

// Create LLM
llm := openai.New("")

// Create agent with LLM
myAgent := agent.New(llm, tools.BuiltinTools(),
    agent.WithMaxIterations(10),
    agent.WithSystemPrompt("You are a helpful coding assistant."),
)

// Run task
result, err := myAgent.Run(ctx, "Write a function to reverse a string in Go")
fmt.Println(result.Output)
```

---

## Custom Providers

Implement the `core.LLM` interface to add your own provider:

```go
type LLM interface {
    Generate(ctx context.Context, prompt string, opts ...Option) (string, error)
    GenerateChat(ctx context.Context, messages []Message, opts ...Option) (string, error)
    Stream(ctx context.Context, prompt string, opts ...Option) (<-chan string, error)
    StreamChat(ctx context.Context, messages []Message, opts ...Option) (<-chan string, error)
}
```

Example custom provider:

```go
type MyLLM struct {
    client *myclient.Client
}

func (m *MyLLM) Generate(ctx context.Context, prompt string, opts ...core.Option) (string, error) {
    return m.client.Complete(prompt)
}

// ... implement other methods
```
