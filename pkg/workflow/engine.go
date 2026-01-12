// Package workflow provides the workflow execution engine.
package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Engine executes workflows.
type Engine struct {
	persistence *Persistence
	signals     *SignalManager
	approvals   *ApprovalManager
	workflows   map[string]*Workflow
	running     map[string]*State
	mu          sync.RWMutex
}

// NewEngine creates a new workflow engine.
func NewEngine(persistence *Persistence) *Engine {
	return &Engine{
		persistence: persistence,
		signals:     NewSignalManager(),
		approvals:   NewApprovalManager(),
		workflows:   make(map[string]*Workflow),
		running:     make(map[string]*State),
	}
}

// Register registers a workflow.
func (e *Engine) Register(workflow *Workflow) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.workflows[workflow.Name] = workflow
}

// Start begins a workflow execution.
func (e *Engine) Start(ctx context.Context, workflowName string, input map[string]any) (string, error) {
	e.mu.RLock()
	workflow, ok := e.workflows[workflowName]
	e.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("workflow not found: %s", workflowName)
	}

	state := &State{
		ID:          fmt.Sprintf("%s-%d", workflowName, time.Now().UnixNano()),
		WorkflowID:  workflow.ID,
		Status:      StatusRunning,
		Data:        input,
		StepResults: make(map[string]any),
		Checkpoints: make(map[string]int),
		StartedAt:   time.Now(),
	}

	if input == nil {
		state.Data = make(map[string]any)
	}

	e.mu.Lock()
	e.running[state.ID] = state
	e.mu.Unlock()

	go e.execute(ctx, workflow, state)

	return state.ID, nil
}

// Execute runs a workflow synchronously.
func (e *Engine) Execute(ctx context.Context, workflow *Workflow, input map[string]any) (*State, error) {
	state := &State{
		ID:          fmt.Sprintf("%s-%d", workflow.Name, time.Now().UnixNano()),
		WorkflowID:  workflow.ID,
		Status:      StatusRunning,
		Data:        input,
		StepResults: make(map[string]any),
		Checkpoints: make(map[string]int),
		StartedAt:   time.Now(),
	}

	if input == nil {
		state.Data = make(map[string]any)
	}

	return e.ExecuteWithState(ctx, workflow, state)
}

// ExecuteWithState executes with existing state.
func (e *Engine) ExecuteWithState(ctx context.Context, workflow *Workflow, state *State) (*State, error) {
	e.mu.Lock()
	e.running[state.ID] = state
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.running, state.ID)
		e.mu.Unlock()
	}()

	err := e.executeSteps(ctx, workflow, state)

	state.CompletedAt = time.Now()
	if err != nil {
		state.Status = StatusFailed
		state.Errors = append(state.Errors, err.Error())

		// Run compensations (saga pattern)
		if len(state.Compensations) > 0 {
			state.Status = StatusCompensating
			e.runCompensations(ctx, state)
		}
	} else {
		state.Status = StatusCompleted
	}

	// Save final state
	if e.persistence != nil {
		e.persistence.Save(ctx, state)
	}

	if workflow.OnComplete != nil {
		workflow.OnComplete(ctx, state)
	}

	return state, err
}

func (e *Engine) execute(ctx context.Context, workflow *Workflow, state *State) {
	e.ExecuteWithState(ctx, workflow, state)
}

func (e *Engine) executeSteps(ctx context.Context, workflow *Workflow, state *State) error {
	for i := state.CurrentStep; i < len(workflow.Steps); i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		state.mu.Lock()
		state.CurrentStep = i
		state.mu.Unlock()

		step := workflow.Steps[i]

		// Save checkpoint before executing
		if e.persistence != nil {
			e.persistence.Save(ctx, state)
		}

		err := step.Execute(ctx, state)
		if err != nil {
			if workflow.OnError != nil {
				if handleErr := workflow.OnError(ctx, state, err); handleErr != nil {
					return handleErr
				}
				continue // Error was handled
			}
			return fmt.Errorf("step '%s' failed: %w", step.Name(), err)
		}
	}

	return nil
}

func (e *Engine) runCompensations(ctx context.Context, state *State) {
	// Run compensations in reverse order
	for i := len(state.Compensations) - 1; i >= 0; i-- {
		comp := state.Compensations[i]
		if err := comp.Handler(ctx, state); err != nil {
			state.Errors = append(state.Errors, fmt.Sprintf("compensation '%s' failed: %v", comp.StepName, err))
		}
	}
}

// Resume resumes a paused workflow.
func (e *Engine) Resume(ctx context.Context, stateID string) error {
	if e.persistence == nil {
		return fmt.Errorf("persistence not configured")
	}

	state, err := e.persistence.Load(ctx, stateID)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	e.mu.RLock()
	workflow, ok := e.workflows[state.WorkflowID]
	e.mu.RUnlock()

	if !ok {
		return fmt.Errorf("workflow not found: %s", state.WorkflowID)
	}

	state.Status = StatusRunning
	go e.execute(ctx, workflow, state)

	return nil
}

// ResumeFromCheckpoint resumes from a checkpoint.
func (e *Engine) ResumeFromCheckpoint(ctx context.Context, stateID, checkpoint string) error {
	if e.persistence == nil {
		return fmt.Errorf("persistence not configured")
	}

	state, err := e.persistence.Load(ctx, stateID)
	if err != nil {
		return err
	}

	stepIndex, ok := state.Checkpoints[checkpoint]
	if !ok {
		return fmt.Errorf("checkpoint not found: %s", checkpoint)
	}

	state.CurrentStep = stepIndex
	state.Status = StatusRunning

	e.mu.RLock()
	workflow := e.workflows[state.WorkflowID]
	e.mu.RUnlock()

	go e.execute(ctx, workflow, state)

	return nil
}

// SendSignal sends a signal to waiting workflows.
func (e *Engine) SendSignal(ctx context.Context, signalName string, data any) {
	e.signals.Send(signalName, data)
}

// Approve sends approval for a workflow.
func (e *Engine) Approve(ctx context.Context, stateID string, approver string) error {
	return e.approvals.Approve(stateID, approver)
}

// Reject rejects a workflow approval.
func (e *Engine) Reject(ctx context.Context, stateID string, approver, reason string) error {
	return e.approvals.Reject(stateID, approver, reason)
}

// GetState returns workflow state.
func (e *Engine) GetState(stateID string) (*State, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	state, ok := e.running[stateID]
	return state, ok
}

// ============ Signal Manager ============

// SignalManager handles workflow signals.
type SignalManager struct {
	waiters map[string][]chan any
	mu      sync.RWMutex
}

// NewSignalManager creates a signal manager.
func NewSignalManager() *SignalManager {
	return &SignalManager{
		waiters: make(map[string][]chan any),
	}
}

// Wait waits for a signal.
func (sm *SignalManager) Wait(ctx context.Context, signalName string) (any, error) {
	ch := make(chan any, 1)

	sm.mu.Lock()
	sm.waiters[signalName] = append(sm.waiters[signalName], ch)
	sm.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case data := <-ch:
		return data, nil
	}
}

// Send sends a signal.
func (sm *SignalManager) Send(signalName string, data any) {
	sm.mu.Lock()
	waiters := sm.waiters[signalName]
	sm.waiters[signalName] = nil
	sm.mu.Unlock()

	for _, ch := range waiters {
		select {
		case ch <- data:
		default:
		}
	}
}

// ============ Approval Manager ============

// ApprovalManager handles human approvals.
type ApprovalManager struct {
	pending map[string]*ApprovalRequest
	mu      sync.RWMutex
}

// ApprovalRequest represents a pending approval.
type ApprovalRequest struct {
	StateID    string
	Approvers  []string
	Approved   map[string]bool
	Rejected   bool
	Reason     string
	ResponseCh chan bool
}

// NewApprovalManager creates an approval manager.
func NewApprovalManager() *ApprovalManager {
	return &ApprovalManager{
		pending: make(map[string]*ApprovalRequest),
	}
}

// RequestApproval creates an approval request.
func (am *ApprovalManager) RequestApproval(stateID string, approvers []string) chan bool {
	ch := make(chan bool, 1)

	am.mu.Lock()
	am.pending[stateID] = &ApprovalRequest{
		StateID:    stateID,
		Approvers:  approvers,
		Approved:   make(map[string]bool),
		ResponseCh: ch,
	}
	am.mu.Unlock()

	return ch
}

// Approve approves a request.
func (am *ApprovalManager) Approve(stateID, approver string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	req, ok := am.pending[stateID]
	if !ok {
		return fmt.Errorf("no pending approval for %s", stateID)
	}

	req.Approved[approver] = true

	// Check if all approved
	allApproved := true
	for _, a := range req.Approvers {
		if !req.Approved[a] {
			allApproved = false
			break
		}
	}

	if allApproved {
		req.ResponseCh <- true
		delete(am.pending, stateID)
	}

	return nil
}

// Reject rejects a request.
func (am *ApprovalManager) Reject(stateID, approver, reason string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	req, ok := am.pending[stateID]
	if !ok {
		return fmt.Errorf("no pending approval for %s", stateID)
	}

	req.Rejected = true
	req.Reason = reason
	req.ResponseCh <- false
	delete(am.pending, stateID)

	return nil
}

// GetPending returns pending approvals.
func (am *ApprovalManager) GetPending() []*ApprovalRequest {
	am.mu.RLock()
	defer am.mu.RUnlock()

	requests := make([]*ApprovalRequest, 0, len(am.pending))
	for _, req := range am.pending {
		requests = append(requests, req)
	}
	return requests
}
