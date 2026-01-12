// Package workflow provides an advanced workflow engine with dynamic features.
package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Workflow represents a workflow definition.
type Workflow struct {
	ID          string
	Name        string
	Version     string
	Steps       []Step
	OnError     ErrorHandler
	OnComplete  CompleteHandler
	persistence *Persistence
}

// Step is the interface for all workflow steps.
type Step interface {
	Execute(ctx context.Context, state *State) error
	Name() string
	Type() StepType
}

// StepType identifies the kind of step.
type StepType string

const (
	StepTypeAction      StepType = "action"
	StepTypeCondition   StepType = "condition"
	StepTypeLoop        StepType = "loop"
	StepTypeParallel    StepType = "parallel"
	StepTypeAwait       StepType = "await"
	StepTypeSleep       StepType = "sleep"
	StepTypeSubWorkflow StepType = "subworkflow"
	StepTypeCheckpoint  StepType = "checkpoint"
)

// State holds the workflow execution state.
type State struct {
	ID           string                 `json:"id"`
	WorkflowID   string                 `json:"workflow_id"`
	CurrentStep  int                    `json:"current_step"`
	Status       Status                 `json:"status"`
	Data         map[string]any         `json:"data"`
	StepResults  map[string]any         `json:"step_results"`
	Checkpoints  map[string]int         `json:"checkpoints"`
	Errors       []string               `json:"errors"`
	StartedAt    time.Time              `json:"started_at"`
	CompletedAt  time.Time              `json:"completed_at,omitempty"`
	Compensations []Compensation        `json:"compensations,omitempty"`
	mu           sync.RWMutex
}

// Status represents workflow execution status.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusPaused    Status = "paused"
	StatusAwaitingSignal Status = "awaiting_signal"
	StatusAwaitingApproval Status = "awaiting_approval"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCompensating Status = "compensating"
)

// ErrorHandler handles workflow errors.
type ErrorHandler func(ctx context.Context, state *State, err error) error

// CompleteHandler is called on workflow completion.
type CompleteHandler func(ctx context.Context, state *State)

// Compensation represents a compensation action for saga pattern.
type Compensation struct {
	StepName string
	Handler  func(ctx context.Context, state *State) error
}

// NewWorkflow creates a new workflow.
func New(name string) *Builder {
	return &Builder{
		workflow: &Workflow{
			ID:      fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
			Name:    name,
			Version: "1.0.0",
			Steps:   make([]Step, 0),
		},
	}
}

// Builder provides fluent API for building workflows.
type Builder struct {
	workflow *Workflow
}

// Version sets the workflow version.
func (b *Builder) Version(v string) *Builder {
	b.workflow.Version = v
	return b
}

// OnError sets the error handler.
func (b *Builder) OnError(handler ErrorHandler) *Builder {
	b.workflow.OnError = handler
	return b
}

// OnComplete sets the completion handler.
func (b *Builder) OnComplete(handler CompleteHandler) *Builder {
	b.workflow.OnComplete = handler
	return b
}

// WithPersistence enables durable execution.
func (b *Builder) WithPersistence(p *Persistence) *Builder {
	b.workflow.persistence = p
	return b
}

// Step adds an action step.
func (b *Builder) Step(name string, handler ActionHandler) *ActionBuilder {
	step := &ActionStep{
		name:    name,
		handler: handler,
	}
	b.workflow.Steps = append(b.workflow.Steps, step)
	return &ActionBuilder{builder: b, step: step}
}

// If adds a conditional step.
func (b *Builder) If(name string, condition Condition) *ConditionBuilder {
	step := &ConditionStep{
		name:      name,
		condition: condition,
	}
	b.workflow.Steps = append(b.workflow.Steps, step)
	return &ConditionBuilder{builder: b, step: step}
}

// Loop adds a loop step.
func (b *Builder) Loop(name string) *LoopBuilder {
	step := &LoopStep{name: name}
	b.workflow.Steps = append(b.workflow.Steps, step)
	return &LoopBuilder{builder: b, step: step}
}

// Parallel adds parallel execution.
func (b *Builder) Parallel(name string, steps ...Step) *ParallelBuilder {
	step := &ParallelStep{
		name:  name,
		steps: steps,
	}
	b.workflow.Steps = append(b.workflow.Steps, step)
	return &ParallelBuilder{builder: b, step: step}
}

// AwaitSignal adds a signal await step.
func (b *Builder) AwaitSignal(name, signalName string) *AwaitBuilder {
	step := &AwaitStep{
		name:       name,
		signalName: signalName,
		awaitType:  AwaitTypeSignal,
	}
	b.workflow.Steps = append(b.workflow.Steps, step)
	return &AwaitBuilder{builder: b, step: step}
}

// AwaitApproval adds a human approval step.
func (b *Builder) AwaitApproval(name string, approvers []string) *AwaitBuilder {
	step := &AwaitStep{
		name:      name,
		approvers: approvers,
		awaitType: AwaitTypeApproval,
	}
	b.workflow.Steps = append(b.workflow.Steps, step)
	return &AwaitBuilder{builder: b, step: step}
}

// Sleep adds a delay step.
func (b *Builder) Sleep(name string, duration time.Duration) *Builder {
	b.workflow.Steps = append(b.workflow.Steps, &SleepStep{
		name:     name,
		duration: duration,
	})
	return b
}

// SubWorkflow adds a sub-workflow step.
func (b *Builder) SubWorkflow(name string, workflow *Workflow) *SubWorkflowBuilder {
	step := &SubWorkflowStep{
		name:     name,
		workflow: workflow,
	}
	b.workflow.Steps = append(b.workflow.Steps, step)
	return &SubWorkflowBuilder{builder: b, step: step}
}

// Checkpoint adds a checkpoint for durability.
func (b *Builder) Checkpoint(name string) *Builder {
	b.workflow.Steps = append(b.workflow.Steps, &CheckpointStep{name: name})
	return b
}

// Build returns the workflow.
func (b *Builder) Build() *Workflow {
	return b.workflow
}

// ============ Action Step ============

// ActionHandler is a step handler function.
type ActionHandler func(ctx context.Context, state *State) (any, error)

// ActionStep is a simple action step.
type ActionStep struct {
	name         string
	handler      ActionHandler
	retryPolicy  *RetryPolicy
	compensation func(ctx context.Context, state *State) error
	timeout      time.Duration
}

func (s *ActionStep) Name() string    { return s.name }
func (s *ActionStep) Type() StepType  { return StepTypeAction }

func (s *ActionStep) Execute(ctx context.Context, state *State) error {
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}

	var result any
	var err error

	if s.retryPolicy != nil {
		result, err = s.retryPolicy.Execute(ctx, func() (any, error) {
			return s.handler(ctx, state)
		})
	} else {
		result, err = s.handler(ctx, state)
	}

	if err != nil {
		return err
	}

	state.mu.Lock()
	state.StepResults[s.name] = result
	state.mu.Unlock()

	// Register compensation if provided
	if s.compensation != nil {
		state.Compensations = append(state.Compensations, Compensation{
			StepName: s.name,
			Handler:  s.compensation,
		})
	}

	return nil
}

// ActionBuilder builds action steps.
type ActionBuilder struct {
	builder *Builder
	step    *ActionStep
}

// Retry sets retry policy.
func (ab *ActionBuilder) Retry(policy *RetryPolicy) *ActionBuilder {
	ab.step.retryPolicy = policy
	return ab
}

// Compensate sets compensation handler for saga pattern.
func (ab *ActionBuilder) Compensate(handler func(ctx context.Context, state *State) error) *ActionBuilder {
	ab.step.compensation = handler
	return ab
}

// Timeout sets step timeout.
func (ab *ActionBuilder) Timeout(d time.Duration) *ActionBuilder {
	ab.step.timeout = d
	return ab
}

// Then continues building.
func (ab *ActionBuilder) Then() *Builder {
	return ab.builder
}

// ============ Condition Step ============

// Condition is a predicate function.
type Condition func(state *State) bool

// ConditionStep implements conditional branching.
type ConditionStep struct {
	name       string
	condition  Condition
	thenSteps  []Step
	elseSteps  []Step
	elifConds  []Condition
	elifSteps  [][]Step
}

func (s *ConditionStep) Name() string    { return s.name }
func (s *ConditionStep) Type() StepType  { return StepTypeCondition }

func (s *ConditionStep) Execute(ctx context.Context, state *State) error {
	var stepsToRun []Step

	if s.condition(state) {
		stepsToRun = s.thenSteps
	} else {
		// Check elif conditions
		for i, cond := range s.elifConds {
			if cond(state) {
				stepsToRun = s.elifSteps[i]
				break
			}
		}
		// Use else if no elif matched
		if stepsToRun == nil {
			stepsToRun = s.elseSteps
		}
	}

	for _, step := range stepsToRun {
		if err := step.Execute(ctx, state); err != nil {
			return err
		}
	}

	return nil
}

// ConditionBuilder builds conditional steps.
type ConditionBuilder struct {
	builder *Builder
	step    *ConditionStep
}

// Then sets the true branch.
func (cb *ConditionBuilder) Then(steps ...Step) *ConditionBuilder {
	cb.step.thenSteps = steps
	return cb
}

// ElseIf adds an else-if branch.
func (cb *ConditionBuilder) ElseIf(condition Condition, steps ...Step) *ConditionBuilder {
	cb.step.elifConds = append(cb.step.elifConds, condition)
	cb.step.elifSteps = append(cb.step.elifSteps, steps)
	return cb
}

// Else sets the false branch.
func (cb *ConditionBuilder) Else(steps ...Step) *ConditionBuilder {
	cb.step.elseSteps = steps
	return cb
}

// End ends the conditional block.
func (cb *ConditionBuilder) End() *Builder {
	return cb.builder
}

// ============ Loop Step ============

// LoopStep implements loops.
type LoopStep struct {
	name          string
	steps         []Step
	forEachKey    string
	whileCondition Condition
	maxIterations int
	breakCondition Condition
}

func (s *LoopStep) Name() string    { return s.name }
func (s *LoopStep) Type() StepType  { return StepTypeLoop }

func (s *LoopStep) Execute(ctx context.Context, state *State) error {
	iteration := 0

	// ForEach loop
	if s.forEachKey != "" {
		items, ok := state.Data[s.forEachKey].([]any)
		if !ok {
			return fmt.Errorf("forEach key '%s' is not an array", s.forEachKey)
		}

		for i, item := range items {
			if s.maxIterations > 0 && iteration >= s.maxIterations {
				break
			}

			state.Data["_index"] = i
			state.Data["_item"] = item

			for _, step := range s.steps {
				if err := step.Execute(ctx, state); err != nil {
					return err
				}
			}

			if s.breakCondition != nil && s.breakCondition(state) {
				break
			}

			iteration++
		}
		return nil
	}

	// While loop
	for {
		if s.maxIterations > 0 && iteration >= s.maxIterations {
			break
		}

		if s.whileCondition != nil && !s.whileCondition(state) {
			break
		}

		state.Data["_iteration"] = iteration

		for _, step := range s.steps {
			if err := step.Execute(ctx, state); err != nil {
				return err
			}
		}

		if s.breakCondition != nil && s.breakCondition(state) {
			break
		}

		iteration++
	}

	return nil
}

// LoopBuilder builds loop steps.
type LoopBuilder struct {
	builder *Builder
	step    *LoopStep
}

// ForEach iterates over a collection.
func (lb *LoopBuilder) ForEach(key string) *LoopBuilder {
	lb.step.forEachKey = key
	return lb
}

// While loops while condition is true.
func (lb *LoopBuilder) While(condition Condition) *LoopBuilder {
	lb.step.whileCondition = condition
	return lb
}

// MaxIterations limits iterations.
func (lb *LoopBuilder) MaxIterations(n int) *LoopBuilder {
	lb.step.maxIterations = n
	return lb
}

// BreakWhen sets break condition.
func (lb *LoopBuilder) BreakWhen(condition Condition) *LoopBuilder {
	lb.step.breakCondition = condition
	return lb
}

// Do sets the loop body.
func (lb *LoopBuilder) Do(steps ...Step) *LoopBuilder {
	lb.step.steps = steps
	return lb
}

// End ends the loop block.
func (lb *LoopBuilder) End() *Builder {
	return lb.builder
}

// ============ Parallel Step ============

// WaitStrategy defines how to wait for parallel steps.
type WaitStrategy int

const (
	WaitAll WaitStrategy = iota
	WaitAny
	WaitCount
)

// ParallelStep executes steps in parallel.
type ParallelStep struct {
	name         string
	steps        []Step
	waitStrategy WaitStrategy
	waitCount    int
}

func (s *ParallelStep) Name() string    { return s.name }
func (s *ParallelStep) Type() StepType  { return StepTypeParallel }

func (s *ParallelStep) Execute(ctx context.Context, state *State) error {
	if len(s.steps) == 0 {
		return nil
	}

	results := make(chan struct {
		name string
		err  error
	}, len(s.steps))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, step := range s.steps {
		go func(st Step) {
			err := st.Execute(ctx, state)
			results <- struct {
				name string
				err  error
			}{st.Name(), err}
		}(step)
	}

	completed := 0
	var firstError error

	for completed < len(s.steps) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case result := <-results:
			completed++

			if result.err != nil && firstError == nil {
				firstError = result.err
			}

			switch s.waitStrategy {
			case WaitAny:
				if result.err == nil {
					return nil
				}
			case WaitCount:
				if result.err == nil && completed >= s.waitCount {
					return nil
				}
			}
		}
	}

	return firstError
}

// ParallelBuilder builds parallel steps.
type ParallelBuilder struct {
	builder *Builder
	step    *ParallelStep
}

// WaitFor sets wait strategy.
func (pb *ParallelBuilder) WaitFor(strategy WaitStrategy) *ParallelBuilder {
	pb.step.waitStrategy = strategy
	return pb
}

// WaitForCount waits for N completions.
func (pb *ParallelBuilder) WaitForCount(n int) *ParallelBuilder {
	pb.step.waitStrategy = WaitCount
	pb.step.waitCount = n
	return pb
}

// Then continues building.
func (pb *ParallelBuilder) Then() *Builder {
	return pb.builder
}

// ============ Await Step ============

// AwaitType defines what to wait for.
type AwaitType int

const (
	AwaitTypeSignal AwaitType = iota
	AwaitTypeApproval
)

// AwaitStep waits for external events.
type AwaitStep struct {
	name       string
	signalName string
	approvers  []string
	awaitType  AwaitType
	timeout    time.Duration
	onTimeout  string
}

func (s *AwaitStep) Name() string    { return s.name }
func (s *AwaitStep) Type() StepType  { return StepTypeAwait }

func (s *AwaitStep) Execute(ctx context.Context, state *State) error {
	// In a real implementation, this would:
	// 1. Save workflow state
	// 2. Return and wait for signal/approval via external mechanism
	// 3. Resume when signal received

	state.mu.Lock()
	if s.awaitType == AwaitTypeApproval {
		state.Status = StatusAwaitingApproval
		state.Data["_awaiting_approvers"] = s.approvers
	} else {
		state.Status = StatusAwaitingSignal
		state.Data["_awaiting_signal"] = s.signalName
	}
	state.mu.Unlock()

	// For now, simulate waiting (in production, this would pause execution)
	if s.timeout > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.timeout):
			if s.onTimeout != "" {
				state.Data["_timeout_action"] = s.onTimeout
			}
			return fmt.Errorf("await timed out after %v", s.timeout)
		}
	}

	return nil
}

// AwaitBuilder builds await steps.
type AwaitBuilder struct {
	builder *Builder
	step    *AwaitStep
}

// Timeout sets the await timeout.
func (ab *AwaitBuilder) Timeout(d time.Duration) *AwaitBuilder {
	ab.step.timeout = d
	return ab
}

// OnTimeout sets timeout action.
func (ab *AwaitBuilder) OnTimeout(action string) *AwaitBuilder {
	ab.step.onTimeout = action
	return ab
}

// Then continues building.
func (ab *AwaitBuilder) Then() *Builder {
	return ab.builder
}

// ============ Sleep Step ============

// SleepStep pauses execution.
type SleepStep struct {
	name     string
	duration time.Duration
}

func (s *SleepStep) Name() string    { return s.name }
func (s *SleepStep) Type() StepType  { return StepTypeSleep }

func (s *SleepStep) Execute(ctx context.Context, state *State) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(s.duration):
		return nil
	}
}

// ============ SubWorkflow Step ============

// SubWorkflowStep executes a nested workflow.
type SubWorkflowStep struct {
	name     string
	workflow *Workflow
	input    map[string]any
}

func (s *SubWorkflowStep) Name() string    { return s.name }
func (s *SubWorkflowStep) Type() StepType  { return StepTypeSubWorkflow }

func (s *SubWorkflowStep) Execute(ctx context.Context, state *State) error {
	// Create sub-state
	subState := &State{
		ID:          fmt.Sprintf("%s-%s", state.ID, s.name),
		WorkflowID:  s.workflow.ID,
		Data:        make(map[string]any),
		StepResults: make(map[string]any),
		Checkpoints: make(map[string]int),
		StartedAt:   time.Now(),
	}

	// Copy input
	for k, v := range s.input {
		subState.Data[k] = v
	}

	// Execute sub-workflow
	engine := NewEngine(nil)
	result, err := engine.ExecuteWithState(ctx, s.workflow, subState)

	// Store result
	state.mu.Lock()
	state.StepResults[s.name] = result.StepResults
	state.mu.Unlock()

	return err
}

// SubWorkflowBuilder builds sub-workflow steps.
type SubWorkflowBuilder struct {
	builder *Builder
	step    *SubWorkflowStep
}

// WithInput sets sub-workflow input.
func (sb *SubWorkflowBuilder) WithInput(input map[string]any) *SubWorkflowBuilder {
	sb.step.input = input
	return sb
}

// Then continues building.
func (sb *SubWorkflowBuilder) Then() *Builder {
	return sb.builder
}

// ============ Checkpoint Step ============

// CheckpointStep saves progress for durability.
type CheckpointStep struct {
	name string
}

func (s *CheckpointStep) Name() string    { return s.name }
func (s *CheckpointStep) Type() StepType  { return StepTypeCheckpoint }

func (s *CheckpointStep) Execute(ctx context.Context, state *State) error {
	state.mu.Lock()
	state.Checkpoints[s.name] = state.CurrentStep
	state.mu.Unlock()
	return nil
}

// ============ Retry Policy ============

// RetryPolicy defines retry behavior.
type RetryPolicy struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	Multiplier      float64
	RetryOn         func(error) bool
}

// NewRetryPolicy creates a new retry policy.
func NewRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

// Exponential sets exponential backoff.
func (p *RetryPolicy) Exponential(initial, max time.Duration) *RetryPolicy {
	p.InitialDelay = initial
	p.MaxDelay = max
	return p
}

// Attempts sets max attempts.
func (p *RetryPolicy) Attempts(n int) *RetryPolicy {
	p.MaxAttempts = n
	return p
}

// OnError sets error filter.
func (p *RetryPolicy) OnError(filter func(error) bool) *RetryPolicy {
	p.RetryOn = filter
	return p
}

// Execute runs with retries.
func (p *RetryPolicy) Execute(ctx context.Context, fn func() (any, error)) (any, error) {
	var lastErr error
	delay := p.InitialDelay

	for attempt := 0; attempt < p.MaxAttempts; attempt++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		if p.RetryOn != nil && !p.RetryOn(err) {
			return nil, err
		}

		if attempt < p.MaxAttempts-1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}

			delay = time.Duration(float64(delay) * p.Multiplier)
			if delay > p.MaxDelay {
				delay = p.MaxDelay
			}
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// ============ Persistence ============

// Persistence handles durable workflow storage.
type Persistence struct {
	client *redis.Client
	prefix string
}

// NewPersistence creates persistence with Redis.
func NewPersistence(client *redis.Client) *Persistence {
	return &Persistence{
		client: client,
		prefix: "goflow:workflow",
	}
}

// Save saves workflow state.
func (p *Persistence) Save(ctx context.Context, state *State) error {
	key := fmt.Sprintf("%s:%s", p.prefix, state.ID)
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return p.client.Set(ctx, key, data, 7*24*time.Hour).Err()
}

// Load loads workflow state.
func (p *Persistence) Load(ctx context.Context, id string) (*State, error) {
	key := fmt.Sprintf("%s:%s", p.prefix, id)
	data, err := p.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// Delete removes workflow state.
func (p *Persistence) Delete(ctx context.Context, id string) error {
	key := fmt.Sprintf("%s:%s", p.prefix, id)
	return p.client.Del(ctx, key).Err()
}
