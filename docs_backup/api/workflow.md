# Workflow API

The `workflow` package provides workflow orchestration.

## Workflow

```go
type Workflow struct {
    ID    string
    Name  string
    Steps []Step
}

func New(name string) *Builder
```

## Builder

```go
type Builder struct{}

func (b *Builder) Step(name string, fn StepFunc) *Builder
func (b *Builder) Then() *Builder
func (b *Builder) If(name string, cond Condition) *IfBuilder
func (b *Builder) Loop(name string) *LoopBuilder
func (b *Builder) Parallel(name string, steps ...Step) *ParallelBuilder
func (b *Builder) AwaitSignal(name string) *SignalBuilder
func (b *Builder) AwaitApproval(name string, approvers []string) *ApprovalBuilder
func (b *Builder) SubWorkflow(name, workflowName string, input any) *Builder
func (b *Builder) Build() *Workflow
```

## Step

```go
type Step interface {
    Execute(ctx context.Context, state *State) error
}

type StepFunc func(ctx context.Context, state *State) error

type ActionStep struct {
    Name       string
    Action     StepFunc
    Compensate StepFunc
    Retry      *RetryPolicy
}
```

## State

```go
type State struct {
    ID        string
    Workflow  string
    Status    Status
    Data      map[string]any
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Status string
const (
    StatusPending   Status = "pending"
    StatusRunning   Status = "running"
    StatusCompleted Status = "completed"
    StatusFailed    Status = "failed"
    StatusPaused    Status = "paused"
)
```

## Engine

```go
type Engine struct{}

func NewEngine(persistence *Persistence) *Engine
func (e *Engine) Register(wf *Workflow)
func (e *Engine) Start(ctx context.Context, name string, data map[string]any) (string, error)
func (e *Engine) Status(stateID string) (*State, error)
func (e *Engine) Pause(stateID string) error
func (e *Engine) Resume(stateID string) error
func (e *Engine) Signal(stateID, signal string, data any) error
func (e *Engine) Approve(stateID, approvalName string) error
func (e *Engine) Reject(stateID, approvalName string) error
```

## Conditionals

```go
type Condition func(state *State) bool

func (b *Builder) If(name string, cond Condition) *IfBuilder
func (ib *IfBuilder) Then(steps ...Step) *IfBuilder
func (ib *IfBuilder) ElseIf(cond Condition) *IfBuilder
func (ib *IfBuilder) Else(steps ...Step) *IfBuilder
func (ib *IfBuilder) End() *Builder
```

## Loops

```go
func (b *Builder) Loop(name string) *LoopBuilder
func (lb *LoopBuilder) ForEach(key string) *LoopBuilder
func (lb *LoopBuilder) While(cond Condition) *LoopBuilder
func (lb *LoopBuilder) MaxIterations(n int) *LoopBuilder
func (lb *LoopBuilder) BreakWhen(cond Condition) *LoopBuilder
func (lb *LoopBuilder) Do(steps ...Step) *LoopBuilder
func (lb *LoopBuilder) End() *Builder
```

## Parallel

```go
type WaitStrategy int
const (
    WaitAll WaitStrategy = iota
    WaitAny
    WaitCount
)

func (b *Builder) Parallel(name string, steps ...Step) *ParallelBuilder
func (pb *ParallelBuilder) WaitFor(strategy WaitStrategy) *ParallelBuilder
func (pb *ParallelBuilder) WaitCount(n int) *ParallelBuilder
```

## RetryPolicy

```go
type RetryPolicy struct {
    MaxAttempts int
    InitialWait time.Duration
    MaxWait     time.Duration
    Multiplier  float64
    RetryOn     func(error) bool
}

func NewRetryPolicy() *RetryPolicy
func (rp *RetryPolicy) Attempts(n int) *RetryPolicy
func (rp *RetryPolicy) Backoff(initial, max time.Duration) *RetryPolicy
```

## Cron

```go
type Cron struct{}

func NewCron(engine *Engine) *Cron
func (c *Cron) Add(id, workflow, expression string, input any) error
func (c *Cron) Remove(id string)
func (c *Cron) Enable(id string)
func (c *Cron) Disable(id string)
func (c *Cron) List() []Schedule
func (c *Cron) Start(ctx context.Context)
func (c *Cron) Stop()
```

Supported expressions:
- Standard cron: `*/5 * * * *`
- `@yearly`, `@monthly`, `@weekly`, `@daily`, `@hourly`
- `@every 5m`, `@every 1h30m`
