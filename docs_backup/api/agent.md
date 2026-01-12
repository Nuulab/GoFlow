# Agent API

The `agent` package provides AI agent capabilities.

## Agent

```go
type Agent struct {
    // ...
}

func New(llm core.LLM, registry *tools.Registry, opts ...Option) *Agent
func (a *Agent) Run(ctx context.Context, task string) (*RunResult, error)
func (a *Agent) Stream(ctx context.Context, task string) (<-chan string, error)
```

## Options

```go
func WithMaxIterations(n int) Option
func WithTimeout(d time.Duration) Option
func WithVerbose(v bool) Option
func WithSystemPrompt(prompt string) Option
func WithMemory(m Memory) Option
func WithHooks(h *Hooks) Option
```

## RunResult

```go
type RunResult struct {
    Output    string
    Steps     []Step
    ToolCalls []ToolCall
    Duration  time.Duration
    Error     error
}

type Step struct {
    Thought string
    Action  string
    Input   string
    Output  string
}
```

## Hooks

```go
type Hooks struct {
    OnStart      func(task string)
    OnBeforeStep func(step int)
    OnAfterStep  func(step int, result string)
    OnToolCall   func(tool, input string)
    OnToolResult func(tool, output string)
    OnComplete   func(result *RunResult)
    OnError      func(err error)
}

func NewHooksBuilder() *HooksBuilder
```

## Supervisor

```go
type Supervisor struct{}

func NewSupervisor(llm core.LLM, opts ...SupervisorOption) *Supervisor
func (s *Supervisor) Run(ctx context.Context, task string) (*RunResult, error)
func (s *Supervisor) AddWorker(name string, agent *Agent)
```

## Router

```go
type Router interface {
    Route(ctx context.Context, task string) (*Agent, error)
}

func NewLLMRouter(llm core.LLM, agents []*Agent) Router
func NewKeywordRouter(mapping map[string]*Agent) Router
```

## Team

```go
type Team struct{}

func NewTeam(llm core.LLM, members ...*Agent) *Team
func (t *Team) Collaborate(ctx context.Context, task string) (*RunResult, error)
```

## Pipeline

```go
type Pipeline struct{}

func NewPipeline(agents ...*Agent) *Pipeline
func (p *Pipeline) Run(ctx context.Context, input string) (string, error)
```

## HierarchicalSupervisor

```go
type HierarchicalSupervisor struct{}

func NewHierarchicalSupervisor(llm core.LLM, registry *tools.Registry) *HierarchicalSupervisor
func (hs *HierarchicalSupervisor) CreateRoot(task string) *HierarchicalAgent
func (hs *HierarchicalSupervisor) SetMaxDepth(d int)
func (hs *HierarchicalSupervisor) SetSpawnLimit(n int)
```

## ChannelHub

```go
type ChannelHub struct{}

func NewChannelHub() *ChannelHub
func (h *ChannelHub) Create(name string, buffer int)
func (h *ChannelHub) Publish(name string, msg Message) error
func (h *ChannelHub) Subscribe(name string) <-chan Message
```

## Consensus

```go
type Consensus struct{}

func NewConsensus(agents []*Agent, strategy VotingStrategy) *Consensus
func (c *Consensus) Decide(ctx context.Context, question string) (string, error)

type VotingStrategy int
const (
    MajorityVote VotingStrategy = iota
    UnanimousVote
    PluralityVote
    WeightedVote
    LLMJudge
)
```
