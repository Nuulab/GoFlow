---
title: Tools
---


Tools extend agent capabilities by allowing them to interact with external systems.

## Built-in Tools

GoFlow includes several built-in tools:

```go
registry := tools.BuiltinTools()
```

| Tool | Description |
|------|-------------|
| `calculator` | Math calculations |
| `web_search` | Search the web |
| `http_request` | Make HTTP requests |
| `read_file` | Read local files |
| `write_file` | Write local files |

## Creating Custom Tools

### Simple Tool

```go
greetTool := tools.NewTool(
    "greet",
    "Greet a person by name",
    func(ctx context.Context, input string) (string, error) {
        return fmt.Sprintf("Hello, %s!", input), nil
    },
)

registry.Register(greetTool)
```

### Tool with Schema

```go
type WeatherInput struct {
    City    string `json:"city" description:"City name"`
    Country string `json:"country" description:"Country code (optional)"`
}

weatherTool := tools.NewToolWithSchema(
    "get_weather",
    "Get current weather for a city",
    WeatherInput{},
    func(ctx context.Context, input WeatherInput) (string, error) {
        // Call weather API
        return getWeather(input.City, input.Country), nil
    },
)
```

### Async Tool

```go
longRunningTool := tools.NewAsyncTool(
    "process_data",
    "Process a large dataset",
    func(ctx context.Context, input string) <-chan tools.Result {
        ch := make(chan tools.Result)
        go func() {
            defer close(ch)
            // Long running operation
            result := processData(input)
            ch <- tools.Result{Output: result}
        }()
        return ch
    },
)
```

## Tool Registry

```go
// Create empty registry
registry := tools.NewRegistry()

// Register individual tool
registry.Register(myTool)

// Register toolkit (group of tools)
registry.RegisterToolkit(tools.WebToolkit())

// List all tools
for _, tool := range registry.List() {
    fmt.Println(tool.Name, "-", tool.Description)
}
```

## Toolkits

Toolkits are pre-built collections of related tools:

```go
// Web toolkit
tools.WebToolkit()  // search, http, scrape

// Data toolkit
tools.DataToolkit() // json, csv, sql

// Math toolkit
tools.MathToolkit() // calculator, statistics

// Shell toolkit (use with caution)
tools.ShellToolkit() // exec, read_file, write_file
```

## Tool Validation

Tools automatically validate inputs based on the schema:

```go
type EmailInput struct {
    To      string `json:"to" validate:"required,email"`
    Subject string `json:"subject" validate:"required,min=1"`
    Body    string `json:"body"`
}
```

## Tool Permissions

Control which tools can be used:

```go
// Only allow specific tools
registry := tools.NewRegistry()
registry.Register(tools.CalculatorTool())
registry.Register(tools.WebSearchTool())
// Agent can only use calculator and web_search

// Deny specific tools
registry := tools.BuiltinTools()
registry.Unregister("shell_exec") // Remove dangerous tool
```
