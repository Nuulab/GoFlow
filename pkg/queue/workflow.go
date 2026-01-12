// Package queue provides workflow orchestration capabilities.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Workflow defines a multi-step job workflow.
type Workflow struct {
	ID          string
	Name        string
	Steps       []*WorkflowStep
	OnError     WorkflowErrorHandler
	OnComplete  func(ctx context.Context, results map[string]any)
	client      *redis.Client
	queue       Queue
}

// WorkflowStep represents a single step in a workflow.
type WorkflowStep struct {
	Name        string
	Handler     func(ctx context.Context, input any) (any, error)
	Retry       int
	Timeout     time.Duration
	OnError     string // Step to go to on error (empty = fail workflow)
}

// WorkflowErrorHandler handles workflow errors.
type WorkflowErrorHandler func(ctx context.Context, step string, err error) error

// WorkflowBuilder provides fluent API for building workflows.
type WorkflowBuilder struct {
	workflow *Workflow
}

// NewWorkflow creates a new workflow builder.
func NewWorkflowBuilder(name string) *WorkflowBuilder {
	return &WorkflowBuilder{
		workflow: &Workflow{
			ID:    fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
			Name:  name,
			Steps: make([]*WorkflowStep, 0),
		},
	}
}

// Step adds a step to the workflow.
func (wb *WorkflowBuilder) Step(name string, handler func(ctx context.Context, input any) (any, error)) *WorkflowBuilder {
	wb.workflow.Steps = append(wb.workflow.Steps, &WorkflowStep{
		Name:    name,
		Handler: handler,
		Retry:   3,
		Timeout: 30 * time.Second,
	})
	return wb
}

// StepWithRetry adds a step with custom retry count.
func (wb *WorkflowBuilder) StepWithRetry(name string, retries int, handler func(ctx context.Context, input any) (any, error)) *WorkflowBuilder {
	wb.workflow.Steps = append(wb.workflow.Steps, &WorkflowStep{
		Name:    name,
		Handler: handler,
		Retry:   retries,
		Timeout: 30 * time.Second,
	})
	return wb
}

// StepWithTimeout adds a step with custom timeout.
func (wb *WorkflowBuilder) StepWithTimeout(name string, timeout time.Duration, handler func(ctx context.Context, input any) (any, error)) *WorkflowBuilder {
	wb.workflow.Steps = append(wb.workflow.Steps, &WorkflowStep{
		Name:    name,
		Handler: handler,
		Retry:   3,
		Timeout: timeout,
	})
	return wb
}

// OnError sets the global error handler.
func (wb *WorkflowBuilder) OnError(handler WorkflowErrorHandler) *WorkflowBuilder {
	wb.workflow.OnError = handler
	return wb
}

// OnComplete sets the completion handler.
func (wb *WorkflowBuilder) OnComplete(handler func(ctx context.Context, results map[string]any)) *WorkflowBuilder {
	wb.workflow.OnComplete = handler
	return wb
}

// Build creates the workflow.
func (wb *WorkflowBuilder) Build() *Workflow {
	return wb.workflow
}

// WorkflowState tracks workflow execution state.
type WorkflowState struct {
	WorkflowID   string           `json:"workflow_id"`
	CurrentStep  int              `json:"current_step"`
	Status       WorkflowStatus   `json:"status"`
	Results      map[string]any   `json:"results"`
	Errors       []string         `json:"errors"`
	StartedAt    time.Time        `json:"started_at"`
	CompletedAt  time.Time        `json:"completed_at,omitempty"`
}

// WorkflowStatus represents workflow execution status.
type WorkflowStatus string

const (
	WorkflowPending   WorkflowStatus = "pending"
	WorkflowRunning   WorkflowStatus = "running"
	WorkflowCompleted WorkflowStatus = "completed"
	WorkflowFailed    WorkflowStatus = "failed"
	WorkflowPaused    WorkflowStatus = "paused"
)

// WorkflowEngine executes workflows.
type WorkflowEngine struct {
	client    *redis.Client
	queue     Queue
	workflows map[string]*Workflow
	mu        sync.RWMutex
}

// NewWorkflowEngine creates a workflow engine.
func NewWorkflowEngine(client *redis.Client, queue Queue) *WorkflowEngine {
	return &WorkflowEngine{
		client:    client,
		queue:     queue,
		workflows: make(map[string]*Workflow),
	}
}

// Register registers a workflow.
func (we *WorkflowEngine) Register(workflow *Workflow) {
	we.mu.Lock()
	defer we.mu.Unlock()
	workflow.client = we.client
	workflow.queue = we.queue
	we.workflows[workflow.Name] = workflow
}

// Start begins a workflow execution.
func (we *WorkflowEngine) Start(ctx context.Context, workflowName string, input any) (string, error) {
	we.mu.RLock()
	workflow, ok := we.workflows[workflowName]
	we.mu.RUnlock()
	
	if !ok {
		return "", fmt.Errorf("workflow not found: %s", workflowName)
	}
	
	// Create execution state
	state := &WorkflowState{
		WorkflowID:  workflow.ID,
		CurrentStep: 0,
		Status:      WorkflowRunning,
		Results:     make(map[string]any),
		StartedAt:   time.Now(),
	}
	state.Results["_input"] = input
	
	if err := we.saveState(ctx, state); err != nil {
		return "", err
	}
	
	// Start execution
	go we.execute(ctx, workflow, state)
	
	return workflow.ID, nil
}

func (we *WorkflowEngine) execute(ctx context.Context, workflow *Workflow, state *WorkflowState) {
	defer func() {
		state.CompletedAt = time.Now()
		we.saveState(ctx, state)
	}()
	
	input := state.Results["_input"]
	
	for i, step := range workflow.Steps {
		state.CurrentStep = i
		we.saveState(ctx, state)
		
		// Execute step with retries
		var result any
		var err error
		
		for attempt := 0; attempt <= step.Retry; attempt++ {
			stepCtx, cancel := context.WithTimeout(ctx, step.Timeout)
			result, err = step.Handler(stepCtx, input)
			cancel()
			
			if err == nil {
				break
			}
			
			if attempt < step.Retry {
				time.Sleep(time.Duration(attempt+1) * time.Second) // Backoff
			}
		}
		
		if err != nil {
			state.Errors = append(state.Errors, fmt.Sprintf("%s: %v", step.Name, err))
			
			if workflow.OnError != nil {
				if handlerErr := workflow.OnError(ctx, step.Name, err); handlerErr != nil {
					state.Status = WorkflowFailed
					return
				}
				continue // Error was handled, continue
			}
			
			state.Status = WorkflowFailed
			return
		}
		
		state.Results[step.Name] = result
		input = result // Pass to next step
	}
	
	state.Status = WorkflowCompleted
	
	if workflow.OnComplete != nil {
		workflow.OnComplete(ctx, state.Results)
	}
}

func (we *WorkflowEngine) saveState(ctx context.Context, state *WorkflowState) error {
	key := fmt.Sprintf("goflow:workflow:%s", state.WorkflowID)
	
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	
	return we.client.Set(ctx, key, data, 7*24*time.Hour).Err()
}

// GetState retrieves workflow execution state.
func (we *WorkflowEngine) GetState(ctx context.Context, workflowID string) (*WorkflowState, error) {
	key := fmt.Sprintf("goflow:workflow:%s", workflowID)
	
	data, err := we.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	
	var state WorkflowState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	
	return &state, nil
}

// Pause pauses a running workflow.
func (we *WorkflowEngine) Pause(ctx context.Context, workflowID string) error {
	state, err := we.GetState(ctx, workflowID)
	if err != nil {
		return err
	}
	
	if state.Status != WorkflowRunning {
		return fmt.Errorf("workflow is not running")
	}
	
	state.Status = WorkflowPaused
	return we.saveState(ctx, state)
}

// Resume resumes a paused workflow.
func (we *WorkflowEngine) Resume(ctx context.Context, workflowID string) error {
	state, err := we.GetState(ctx, workflowID)
	if err != nil {
		return err
	}
	
	if state.Status != WorkflowPaused {
		return fmt.Errorf("workflow is not paused")
	}
	
	we.mu.RLock()
	var workflow *Workflow
	for _, w := range we.workflows {
		if w.ID == workflowID {
			workflow = w
			break
		}
	}
	we.mu.RUnlock()
	
	if workflow == nil {
		return fmt.Errorf("workflow not found")
	}
	
	state.Status = WorkflowRunning
	go we.execute(context.Background(), workflow, state)
	
	return nil
}

// DAGWorkflow represents a Directed Acyclic Graph workflow.
type DAGWorkflow struct {
	nodes map[string]*DAGNode
	edges map[string][]string // node -> dependencies
}

// DAGNode is a node in the DAG workflow.
type DAGNode struct {
	Name    string
	Handler func(ctx context.Context, inputs map[string]any) (any, error)
}

// NewDAGWorkflow creates a DAG workflow.
func NewDAGWorkflow() *DAGWorkflow {
	return &DAGWorkflow{
		nodes: make(map[string]*DAGNode),
		edges: make(map[string][]string),
	}
}

// Node adds a node to the DAG.
func (dw *DAGWorkflow) Node(name string, handler func(ctx context.Context, inputs map[string]any) (any, error)) *DAGWorkflow {
	dw.nodes[name] = &DAGNode{
		Name:    name,
		Handler: handler,
	}
	return dw
}

// Edge adds a dependency edge (from depends on to).
func (dw *DAGWorkflow) Edge(from, to string) *DAGWorkflow {
	dw.edges[from] = append(dw.edges[from], to)
	return dw
}

// Execute runs the DAG workflow.
func (dw *DAGWorkflow) Execute(ctx context.Context, input any) (map[string]any, error) {
	results := make(map[string]any)
	results["_input"] = input
	
	completed := make(map[string]bool)
	var mu sync.Mutex
	
	// Find nodes with no dependencies
	var ready []string
	for name := range dw.nodes {
		if len(dw.edges[name]) == 0 {
			ready = append(ready, name)
		}
	}
	
	for len(completed) < len(dw.nodes) {
		if len(ready) == 0 {
			return results, fmt.Errorf("deadlock detected in DAG")
		}
		
		// Execute ready nodes in parallel
		var wg sync.WaitGroup
		for _, name := range ready {
			wg.Add(1)
			go func(nodeName string) {
				defer wg.Done()
				
				node := dw.nodes[nodeName]
				
				// Gather inputs from dependencies
				inputs := make(map[string]any)
				for _, dep := range dw.edges[nodeName] {
					mu.Lock()
					inputs[dep] = results[dep]
					mu.Unlock()
				}
				inputs["_input"] = input
				
				result, err := node.Handler(ctx, inputs)
				
				mu.Lock()
				if err != nil {
					results[nodeName] = fmt.Sprintf("error: %v", err)
				} else {
					results[nodeName] = result
				}
				completed[nodeName] = true
				mu.Unlock()
			}(name)
		}
		wg.Wait()
		
		// Find newly ready nodes
		ready = nil
		for name := range dw.nodes {
			if completed[name] {
				continue
			}
			
			allDepsComplete := true
			for _, dep := range dw.edges[name] {
				if !completed[dep] {
					allDepsComplete = false
					break
				}
			}
			
			if allDepsComplete {
				ready = append(ready, name)
			}
		}
	}
	
	return results, nil
}
