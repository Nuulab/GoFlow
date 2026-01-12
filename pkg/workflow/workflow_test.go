// Package workflow_test provides comprehensive tests for the workflow package.
package workflow_test

import (
	"context"
	"testing"
	"time"

	"github.com/nuulab/goflow/pkg/workflow"
)

func TestWorkflowBuilder(t *testing.T) {
	wf := workflow.New("test-workflow").
		Version("1.0.0").
		Build()

	if wf.Name != "test-workflow" {
		t.Errorf("Expected name 'test-workflow', got '%s'", wf.Name)
	}

	if wf.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", wf.Version)
	}
}

func TestWorkflowWithSteps(t *testing.T) {
	wf := workflow.New("step-test").
		Step("first-step", func(ctx context.Context, state *workflow.State) (any, error) {
			state.Data["processed"] = true
			return "step completed", nil
		}).Then().
		Build()

	if len(wf.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(wf.Steps))
	}

	if wf.Steps[0].Name() != "first-step" {
		t.Errorf("Expected step name 'first-step', got '%s'", wf.Steps[0].Name())
	}

	if wf.Steps[0].Type() != workflow.StepTypeAction {
		t.Errorf("Expected action step type, got %s", wf.Steps[0].Type())
	}
}

func TestWorkflowState(t *testing.T) {
	state := &workflow.State{
		ID:          "test-state-1",
		WorkflowID:  "test-workflow",
		Status:      workflow.StatusPending,
		Data:        make(map[string]any),
		StepResults: make(map[string]any),
		Checkpoints: make(map[string]int),
		StartedAt:   time.Now(),
	}

	// Test Data map directly
	state.Data["key1"] = "value1"
	state.Data["key2"] = 42

	if state.Data["key1"] != "value1" {
		t.Error("Expected Data['key1'] to be 'value1'")
	}

	if state.Data["key2"] != 42 {
		t.Error("Expected Data['key2'] to be 42")
	}

	// Test non-existent key
	val := state.Data["nonexistent"]
	if val != nil {
		t.Error("Expected nil for non-existent key")
	}
}

func TestWorkflowStatus(t *testing.T) {
	statuses := []workflow.Status{
		workflow.StatusPending,
		workflow.StatusRunning,
		workflow.StatusPaused,
		workflow.StatusCompleted,
		workflow.StatusFailed,
	}

	expectedStrings := []string{
		"pending",
		"running",
		"paused",
		"completed",
		"failed",
	}

	for i, status := range statuses {
		if string(status) != expectedStrings[i] {
			t.Errorf("Expected status '%s', got '%s'", expectedStrings[i], status)
		}
	}
}

func TestWorkflowConditionalStep(t *testing.T) {
	thenStep := &mockStep{name: "then-action"}
	elseStep := &mockStep{name: "else-action"}

	wf := workflow.New("conditional-test").
		If("check-value", func(state *workflow.State) bool {
			val, ok := state.Data["condition"]
			return ok && val == true
		}).
		Then(thenStep).
		Else(elseStep).
		End().
		Build()

	if len(wf.Steps) != 1 {
		t.Errorf("Expected 1 conditional step, got %d", len(wf.Steps))
	}

	if wf.Steps[0].Type() != workflow.StepTypeCondition {
		t.Errorf("Expected condition step type, got %s", wf.Steps[0].Type())
	}
}

func TestWorkflowParallelStep(t *testing.T) {
	step1 := &mockStep{name: "parallel-1"}
	step2 := &mockStep{name: "parallel-2"}

	wf := workflow.New("parallel-test").
		Parallel("parallel-execution", step1, step2).
		Then().
		Build()

	if len(wf.Steps) != 1 {
		t.Errorf("Expected 1 parallel step, got %d", len(wf.Steps))
	}

	if wf.Steps[0].Type() != workflow.StepTypeParallel {
		t.Errorf("Expected parallel step type, got %s", wf.Steps[0].Type())
	}
}

func TestWorkflowSleepStep(t *testing.T) {
	wf := workflow.New("sleep-test").
		Sleep("wait-step", 100*time.Millisecond).
		Build()

	if len(wf.Steps) != 1 {
		t.Errorf("Expected 1 sleep step, got %d", len(wf.Steps))
	}

	if wf.Steps[0].Type() != workflow.StepTypeSleep {
		t.Errorf("Expected sleep step type, got %s", wf.Steps[0].Type())
	}
}

func TestWorkflowErrorHandler(t *testing.T) {
	wf := workflow.New("error-test").
		OnError(func(ctx context.Context, state *workflow.State, err error) error {
			state.Data["error_handled"] = true
			return nil
		}).
		Build()

	if wf.OnError == nil {
		t.Error("Expected error handler to be set")
	}
}

func TestWorkflowCompleteHandler(t *testing.T) {
	wf := workflow.New("complete-test").
		OnComplete(func(ctx context.Context, state *workflow.State) {
			state.Data["completed"] = true
		}).
		Build()

	if wf.OnComplete == nil {
		t.Error("Expected complete handler to be set")
	}
}

func TestWorkflowLoopStep(t *testing.T) {
	loopStep := &mockStep{name: "loop-body"}

	wf := workflow.New("loop-test").
		Loop("iterate").
		MaxIterations(5).
		While(func(state *workflow.State) bool {
			return true
		}).
		Do(loopStep).
		End().
		Build()

	if len(wf.Steps) != 1 {
		t.Errorf("Expected 1 loop step, got %d", len(wf.Steps))
	}

	if wf.Steps[0].Type() != workflow.StepTypeLoop {
		t.Errorf("Expected loop step type, got %s", wf.Steps[0].Type())
	}
}

func TestWorkflowMultipleSteps(t *testing.T) {
	wf := workflow.New("multi-step").
		Step("step-1", func(ctx context.Context, state *workflow.State) (any, error) {
			return "result-1", nil
		}).Then().
		Step("step-2", func(ctx context.Context, state *workflow.State) (any, error) {
			return "result-2", nil
		}).Then().
		Sleep("wait", 10*time.Millisecond).
		Build()

	if len(wf.Steps) != 3 {
		t.Errorf("Expected 3 steps, got %d", len(wf.Steps))
	}
}

// Mock step for testing
type mockStep struct {
	name string
}

func (m *mockStep) Execute(ctx context.Context, state *workflow.State) error {
	return nil
}

func (m *mockStep) Name() string {
	return m.name
}

func (m *mockStep) Type() workflow.StepType {
	return workflow.StepTypeAction
}
