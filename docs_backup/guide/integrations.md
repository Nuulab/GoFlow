# Integrations

GoFlow integrates with external services for enhanced capabilities.

## MCP Server Integration

Connect to [Model Context Protocol](https://modelcontextprotocol.io/) servers to use external tools:

```go
import "github.com/goflow/goflow/pkg/integrations/mcp"

// Connect to MCP server
client, _ := mcp.New(mcp.Config{
    Name: "neon",
    Transport: mcp.TransportConfig{
        Type: "ws",  // or "sse", "http"
        URL:  "ws://localhost:8080",
    },
})

// Discover available tools
client.Connect(ctx)
tools := client.Tools()

// Call a tool
result, _ := client.Call(ctx, "create_database", map[string]any{
    "name": "mydb",
})

// Convert to GoFlow tools for agents
for _, tool := range tools {
    registry.Register(client.ToGoFlowTool(tool))
}
```

MCP servers expose tools via a standardized protocol. GoFlow supports all three transports: WebSocket, Server-Sent Events (SSE), and HTTP.

## E2B Code Sandbox

Execute code in isolated cloud sandboxes using [E2B](https://e2b.dev/):

```go
import "github.com/goflow/goflow/pkg/integrations/e2b"

// Create client
client := e2b.New(os.Getenv("E2B_API_KEY"))

// Create sandbox
sandbox, _ := client.CreateSandbox(ctx, e2b.CreateSandboxOptions{
    Template: "python",
    Timeout:  60 * time.Second,
})
defer sandbox.Kill(ctx)

// Run Python code
result, _ := sandbox.RunPython(ctx, `
import pandas as pd
df = pd.DataFrame({'a': [1, 2, 3]})
print(df.describe())
`)
fmt.Println(result.Stdout)

// Run JavaScript
result, _ = sandbox.RunJavaScript(ctx, `console.log("Hello from JS!")`)

// File operations
sandbox.WriteFile(ctx, "/app/data.json", []byte(`{"key": "value"}`))
content, _ := sandbox.ReadFile(ctx, "/app/data.json")
```

E2B provides secure, ephemeral sandboxes for code execution without risking your production environment.

### E2B Tool for Agents

```go
tool := e2b.NewTool(os.Getenv("E2B_API_KEY"), "python")
registry.Register(tool)

// Agent can now execute code
agent.Run(ctx, "Calculate the first 10 Fibonacci numbers using Python")
```

## Browserbase Browser Automation

Automate browsers with [Browserbase](https://www.browserbase.com/):

```go
import "github.com/goflow/goflow/pkg/integrations/browserbase"

// Create client
client := browserbase.New(
    os.Getenv("BROWSERBASE_API_KEY"),
    os.Getenv("BROWSERBASE_PROJECT_ID"),
)

// Create session
session, _ := client.CreateSession(ctx, &browserbase.CreateSessionOptions{
    Fingerprint: &browserbase.Fingerprint{
        Browsers: []string{"chrome"},
    },
})
defer session.Close(ctx)

// Navigate and interact
session.Navigate(ctx, "https://example.com")
session.Click(ctx, "#login-button")
session.Type(ctx, "#username", "alice")
session.Type(ctx, "#password", "secret")
session.Click(ctx, "#submit")

// Extract content
text, _ := session.ExtractText(ctx, ".main-content")
html, _ := session.ExtractHTML(ctx, ".results")

// Take screenshot
screenshot, _ := session.Screenshot(ctx)
```

Browserbase provides managed browser infrastructure with anti-detection, proxies, and fingerprinting.

### Browserbase Tool for Agents

```go
tool := browserbase.NewTool(
    os.Getenv("BROWSERBASE_API_KEY"),
    os.Getenv("BROWSERBASE_PROJECT_ID"),
)
registry.Register(tool)

// Agent can now browse the web
agent.Run(ctx, "Go to news.ycombinator.com and summarize the top 5 stories")
```

## Environment Variables

| Service | Variable | Description |
|---------|----------|-------------|
| E2B | `E2B_API_KEY` | E2B API key |
| Browserbase | `BROWSERBASE_API_KEY` | Browserbase API key |
| Browserbase | `BROWSERBASE_PROJECT_ID` | Browserbase project ID |
| MCP | (varies) | Server-specific configuration |
