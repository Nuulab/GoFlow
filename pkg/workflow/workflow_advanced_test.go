// Package workflow_test provides advanced tests for the workflow package.
package workflow_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nuulab/goflow/pkg/workflow"
)

// ============ Step Execution Tests ============

func TestActionStep_Execute(t *testing.T) {
	executed := false
	wf := workflow.New("action-exec").
		Step("test-step", func(ctx context.Context, state *workflow.State) (any, error) {
			executed = true
			return "result", nil
		}).Then().
		Build()

	state := &workflow.State{
		Data:        make(map[string]any),
		StepResults: make(map[string]any),
	}

	err := wf.Steps[0].Execute(context.Background(), state)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !executed {
		t.Error("Step was not executed")
	}
	if state.StepResults["test-step"] != "result" {
		t.Error("Step result not stored")
	}
}

func TestActionStep_ExecuteError(t *testing.T) {
	expectedErr := errors.New("step failed")
	wf := workflow.New("action-error").
		Step("error-step", func(ctx context.Context, state *workflow.State) (any, error) {
			return nil, expectedErr
		}).Then().
		Build()

	state := &workflow.State{
		Data:        make(map[string]any),
		StepResults: make(map[string]any),
	}

	err := wf.Steps[0].Execute(context.Background(), state)
	if err != expectedErr {
		t.Errorf("Expected specific error, got: %v", err)
	}
}

// ============ Parallel Step Tests ============

func TestParallelStep_Execute(t *testing.T) {
	var count int32

	step1 := &countingStep{name: "p1", counter: &count}
	step2 := &countingStep{name: "p2", counter: &count}
	step3 := &countingStep{name: "p3", counter: &count}

	wf := workflow.New("parallel-exec").
		Parallel("parallel", step1, step2, step3).
		Then().
		Build()

	state := &workflow.State{
		Data:        make(map[string]any),
		StepResults: make(map[string]any),
	}

	err := wf.Steps[0].Execute(context.Background(), state)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if atomic.LoadInt32(&count) != 3 {
		t.Errorf("Expected 3 executions, got %d", count)
	}
}

func TestParallelStep_PartialFailure(t *testing.T) {
	step1 := &mockStep{name: "success1"}
	step2 := &errorStep{name: "failure", err: errors.New("step failed")}
	step3 := &mockStep{name: "success2"}

	wf := workflow.New("parallel-fail").
		Parallel("parallel", step1, step2, step3).
		Then().
		Build()

	state := &workflow.State{
		Data:        make(map[string]any),
		StepResults: make(map[string]any),
	}

	err := wf.Steps[0].Execute(context.Background(), state)
	if err == nil {
		t.Error("Expected error from parallel execution")
	}
}

// ============ Sleep Step Tests ============

func TestSleepStep_Execute(t *testing.T) {
	wf := workflow.New("sleep-exec").
		Sleep("short-sleep", 50*time.Millisecond).
		Build()

	state := &workflow.State{
		Data:        make(map[string]any),
		StepResults: make(map[string]any),
	}

	start := time.Now()
	err := wf.Steps[0].Execute(context.Background(), state)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if elapsed < 50*time.Millisecond {
		t.Error("Sleep was too short")
	}
	if elapsed > 200*time.Millisecond {
		t.Error("Sleep was too long")
	}
}

func TestSleepStep_ContextCancellation(t *testing.T) {
	wf := workflow.New("sleep-cancel").
		Sleep("long-sleep", 10*time.Second).
		Build()

	state := &workflow.State{
		Data:        make(map[string]any),
		StepResults: make(map[string]any),
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error)
	go func() {
		done <- wf.Steps[0].Execute(ctx, state)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Sleep did not respond to cancellation")
	}
}

// ============ Condition Step Tests ============

func TestConditionStep_TrueBranch(t *testing.T) {
	thenExecuted := false
	elseExecuted := false

	thenStep := &callbackStep{name: "then", callback: func() { thenExecuted = true }}
	elseStep := &callbackStep{name: "else", callback: func() { elseExecuted = true }}

	wf := workflow.New("cond-true").
		If("check", func(state *workflow.State) bool {
			return state.Data["flag"] == true
		}).
		Then(thenStep).
		Else(elseStep).
		End().
		Build()

	state := &workflow.State{
		Data:        map[string]any{"flag": true},
		StepResults: make(map[string]any),
	}

	err := wf.Steps[0].Execute(context.Background(), state)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !thenExecuted {
		t.Error("Then branch should have executed")
	}
	if elseExecuted {
		t.Error("Else branch should not have executed")
	}
}

func TestConditionStep_FalseBranch(t *testing.T) {
	thenExecuted := false
	elseExecuted := false

	thenStep := &callbackStep{name: "then", callback: func() { thenExecuted = true }}
	elseStep := &callbackStep{name: "else", callback: func() { elseExecuted = true }}

	wf := workflow.New("cond-false").
		If("check", func(state *workflow.State) bool {
			return state.Data["flag"] == true
		}).
		Then(thenStep).
		Else(elseStep).
		End().
		Build()

	state := &workflow.State{
		Data:        map[string]any{"flag": false},
		StepResults: make(map[string]any),
	}

	err := wf.Steps[0].Execute(context.Background(), state)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if thenExecuted {
		t.Error("Then branch should not have executed")
	}
	if !elseExecuted {
		t.Error("Else branch should have executed")
	}
}

// ============ Loop Step Tests ============

func TestLoopStep_MaxIterations(t *testing.T) {
	var count int32
	loopStep := &countingStep{name: "count", counter: &count}

	wf := workflow.New("loop-max").
		Loop("limited-loop").
		MaxIterations(5).
		While(func(state *workflow.State) bool { return true }). // Always true
		Do(loopStep).
		End().
		Build()

	state := &workflow.State{
		Data:        make(map[string]any),
		StepResults: make(map[string]any),
	}

	err := wf.Steps[0].Execute(context.Background(), state)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if atomic.LoadInt32(&count) != 5 {
		t.Errorf("Expected 5 iterations, got %d", count)
	}
}

func TestLoopStep_WhileCondition(t *testing.T) {
	var count int32

	wf := workflow.New("loop-while").
		Loop("while-loop").
		While(func(state *workflow.State) bool {
			val, ok := state.Data["_iteration"].(int)
			return !ok || val < 3
		}).
		MaxIterations(10).
		Do(&countingStep{name: "count", counter: &count}).
		End().
		Build()

	state := &workflow.State{
		Data:        make(map[string]any),
		StepResults: make(map[string]any),
	}

	err := wf.Steps[0].Execute(context.Background(), state)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	// Loop runs while condition is true (iterations 0, 1, 2, 3) = 4 times
	// because condition is checked BEFORE _iteration is updated
	if atomic.LoadInt32(&count) != 4 {
		t.Errorf("Expected 4 iterations, got %d", count)
	}
}

// ============ Concurrency Tests ============

func TestState_ConcurrentRead(t *testing.T) {
	// Test concurrent READ access is safe (writes happen before goroutines)
	state := &workflow.State{
		Data:        make(map[string]any),
		StepResults: make(map[string]any),
		Checkpoints: make(map[string]int),
	}

	// Initialize data first
	for i := 0; i < 100; i++ {
		state.StepResults["step_"+string(rune('a'+i%26))] = i
	}

	// Now read concurrently - this is safe
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = state.StepResults["step_"+string(rune('a'+i%26))]
		}(i)
	}
	wg.Wait()

	t.Log("Concurrent read access completed successfully")
}

func TestState_SequentialMutations(t *testing.T) {
	// Test that sequential state mutations work correctly
	state := &workflow.State{
		Data:        make(map[string]any),
		StepResults: make(map[string]any),
		Checkpoints: make(map[string]int),
	}

	// Mutate state sequentially
	for i := 0; i < 100; i++ {
		state.Data["key_"+string(rune('a'+i%26))] = i
		state.StepResults["result_"+string(rune('a'+i%26))] = "value"
		state.Checkpoints["checkpoint_"+string(rune('a'+i%26))] = i
	}

	// Verify data integrity
	if len(state.Data) != 26 {
		t.Errorf("Expected 26 unique data keys, got %d", len(state.Data))
	}
	if len(state.StepResults) != 26 {
		t.Errorf("Expected 26 unique result keys, got %d", len(state.StepResults))
	}
}

// ============ Helper Types ============

type countingStep struct {
	name    string
	counter *int32
}

func (s *countingStep) Execute(ctx context.Context, state *workflow.State) error {
	atomic.AddInt32(s.counter, 1)
	return nil
}

func (s *countingStep) Name() string          { return s.name }
func (s *countingStep) Type() workflow.StepType { return workflow.StepTypeAction }

type errorStep struct {
	name string
	err  error
}

func (s *errorStep) Execute(ctx context.Context, state *workflow.State) error {
	return s.err
}

func (s *errorStep) Name() string          { return s.name }
func (s *errorStep) Type() workflow.StepType { return workflow.StepTypeAction }

type callbackStep struct {
	name     string
	callback func()
}

func (s *callbackStep) Execute(ctx context.Context, state *workflow.State) error {
	s.callback()
	return nil
}

func (s *callbackStep) Name() string          { return s.name }
func (s *callbackStep) Type() workflow.StepType { return workflow.StepTypeAction }
