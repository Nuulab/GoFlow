# Tools API

The `tools` package provides tool registration and built-in tools.

## Tool

```go
type Tool struct {
    Name        string
    Description string
    Execute     func(ctx context.Context, input string) (string, error)
    Schema      any
}

func NewTool(name, description string, fn func(context.Context, string) (string, error)) *Tool
func NewToolWithSchema[T any](name, description string, schema T, fn func(context.Context, T) (string, error)) *Tool
```

## Registry

```go
type Registry struct{}

func NewRegistry() *Registry
func BuiltinTools() *Registry

func (r *Registry) Register(tool *Tool)
func (r *Registry) RegisterToolkit(toolkit *Toolkit)
func (r *Registry) Unregister(name string)
func (r *Registry) Get(name string) (*Tool, bool)
func (r *Registry) List() []*Tool
func (r *Registry) Execute(ctx context.Context, name, input string) (string, error)
```

## Built-in Tools

```go
func CalculatorTool() *Tool
func WebSearchTool() *Tool
func HTTPRequestTool() *Tool
func ReadFileTool() *Tool
func WriteFileTool() *Tool
func ShellExecTool() *Tool
func JSONParseTool() *Tool
```

## Toolkits

```go
type Toolkit struct {
    Name  string
    Tools []*Tool
}

func WebToolkit() *Toolkit    // search, http, scrape
func DataToolkit() *Toolkit   // json, csv, sql
func MathToolkit() *Toolkit   // calculator, stats
func ShellToolkit() *Toolkit  // exec, read, write
```
