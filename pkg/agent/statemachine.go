// Package agent provides state machine workflow capabilities.
package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/nuulab/goflow/pkg/core"
	"github.com/nuulab/goflow/pkg/tools"
)

// StateMachine defines agent workflows as state transitions.
type StateMachine struct {
	llm          core.LLM
	registry     *tools.Registry
	states       map[string]*State
	initialState string
	currentState string
	context      map[string]any
	mu           sync.RWMutex
}

// State represents a state in the machine.
type State struct {
	Name        string
	Description string
	Agent       *Agent
	OnEnter     func(ctx context.Context, data map[string]any) error
	OnExit      func(ctx context.Context, data map[string]any) error
	Transitions map[string]string // event -> next state
}

// NewStateMachine creates a new state machine.
func NewStateMachine(llm core.LLM, registry *tools.Registry) *StateMachine {
	return &StateMachine{
		llm:      llm,
		registry: registry,
		states:   make(map[string]*State),
		context:  make(map[string]any),
	}
}

// State adds or configures a state.
func (sm *StateMachine) State(name string) *StateBuilder {
	return &StateBuilder{
		sm:   sm,
		name: name,
	}
}

// StateBuilder provides fluent API for state configuration.
type StateBuilder struct {
	sm          *StateMachine
	name        string
	description string
	agent       *Agent
	onEnter     func(ctx context.Context, data map[string]any) error
	onExit      func(ctx context.Context, data map[string]any) error
	transitions map[string]string
}

// Description sets the state description.
func (sb *StateBuilder) Description(desc string) *StateBuilder {
	sb.description = desc
	return sb
}

// WithAgent assigns an agent to handle this state.
func (sb *StateBuilder) WithAgent(agent *Agent) *StateBuilder {
	sb.agent = agent
	return sb
}

// OnEnter sets the entry callback.
func (sb *StateBuilder) OnEnter(fn func(ctx context.Context, data map[string]any) error) *StateBuilder {
	sb.onEnter = fn
	return sb
}

// OnExit sets the exit callback.
func (sb *StateBuilder) OnExit(fn func(ctx context.Context, data map[string]any) error) *StateBuilder {
	sb.onExit = fn
	return sb
}

// OnComplete transitions to next state when agent completes.
func (sb *StateBuilder) OnComplete(nextState string) *StateBuilder {
	if sb.transitions == nil {
		sb.transitions = make(map[string]string)
	}
	sb.transitions["complete"] = nextState
	return sb
}

// OnEvent defines a transition for a specific event.
func (sb *StateBuilder) OnEvent(event, nextState string) *StateBuilder {
	if sb.transitions == nil {
		sb.transitions = make(map[string]string)
	}
	sb.transitions[event] = nextState
	return sb
}

// Build finalizes and registers the state.
func (sb *StateBuilder) Build() *StateMachine {
	state := &State{
		Name:        sb.name,
		Description: sb.description,
		Agent:       sb.agent,
		OnEnter:     sb.onEnter,
		OnExit:      sb.onExit,
		Transitions: sb.transitions,
	}
	
	if state.Agent == nil {
		state.Agent = New(sb.sm.llm, sb.sm.registry)
	}
	
	sb.sm.states[sb.name] = state
	
	if sb.sm.initialState == "" {
		sb.sm.initialState = sb.name
	}
	
	return sb.sm
}

// SetInitialState sets the starting state.
func (sm *StateMachine) SetInitialState(name string) *StateMachine {
	sm.initialState = name
	return sm
}

// SetContext sets workflow context data.
func (sm *StateMachine) SetContext(key string, value any) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.context[key] = value
}

// GetContext retrieves workflow context data.
func (sm *StateMachine) GetContext(key string) any {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.context[key]
}

// StateMachineResult holds the result of a workflow run.
type StateMachineResult struct {
	FinalState     string
	StatesVisited  []string
	StateResults   map[string]*RunResult
	Context        map[string]any
	Error          error
}

// Run executes the state machine until it reaches a terminal state.
func (sm *StateMachine) Run(ctx context.Context, initialTask string) (*StateMachineResult, error) {
	result := &StateMachineResult{
		StatesVisited: make([]string, 0),
		StateResults:  make(map[string]*RunResult),
		Context:       make(map[string]any),
	}
	
	sm.currentState = sm.initialState
	currentTask := initialTask
	
	for {
		select {
		case <-ctx.Done():
			result.Error = ctx.Err()
			return result, ctx.Err()
		default:
		}
		
		state, ok := sm.states[sm.currentState]
		if !ok {
			result.Error = fmt.Errorf("unknown state: %s", sm.currentState)
			return result, result.Error
		}
		
		result.StatesVisited = append(result.StatesVisited, sm.currentState)
		
		// OnEnter callback
		if state.OnEnter != nil {
			if err := state.OnEnter(ctx, sm.context); err != nil {
				result.Error = fmt.Errorf("state %s OnEnter failed: %w", sm.currentState, err)
				return result, result.Error
			}
		}
		
		// Run the state's agent
		stateResult, err := state.Agent.Run(ctx, currentTask)
		result.StateResults[sm.currentState] = stateResult
		
		// OnExit callback
		if state.OnExit != nil {
			if err := state.OnExit(ctx, sm.context); err != nil {
				result.Error = fmt.Errorf("state %s OnExit failed: %w", sm.currentState, err)
				return result, result.Error
			}
		}
		
		// Determine next state
		var event string
		if err != nil {
			event = "error"
		} else {
			event = "complete"
		}
		
		nextState, hasTransition := state.Transitions[event]
		if !hasTransition {
			// Terminal state
			result.FinalState = sm.currentState
			break
		}
		
		// Transition
		sm.currentState = nextState
		currentTask = stateResult.Output // Pass output to next state
		state.Agent.Reset()
	}
	
	// Copy context to result
	sm.mu.RLock()
	for k, v := range sm.context {
		result.Context[k] = v
	}
	sm.mu.RUnlock()
	
	return result, nil
}

// Trigger manually triggers an event transition.
func (sm *StateMachine) Trigger(event string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	state, ok := sm.states[sm.currentState]
	if !ok {
		return fmt.Errorf("unknown current state: %s", sm.currentState)
	}
	
	nextState, ok := state.Transitions[event]
	if !ok {
		return fmt.Errorf("no transition for event '%s' from state '%s'", event, sm.currentState)
	}
	
	sm.currentState = nextState
	return nil
}

// CurrentState returns the current state name.
func (sm *StateMachine) CurrentState() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState
}

// WorkflowBuilder provides a simplified API for common workflow patterns.
type WorkflowBuilder struct {
	sm    *StateMachine
	steps []string
}

// NewWorkflow creates a simple linear workflow.
func NewWorkflow(llm core.LLM, registry *tools.Registry) *WorkflowBuilder {
	return &WorkflowBuilder{
		sm:    NewStateMachine(llm, registry),
		steps: make([]string, 0),
	}
}

// Step adds a step to the workflow.
func (wb *WorkflowBuilder) Step(name string, agent *Agent) *WorkflowBuilder {
	wb.steps = append(wb.steps, name)
	wb.sm.State(name).WithAgent(agent).Build()
	return wb
}

// Build finalizes the workflow with automatic transitions.
func (wb *WorkflowBuilder) Build() *StateMachine {
	// Link steps together
	for i := 0; i < len(wb.steps)-1; i++ {
		wb.sm.states[wb.steps[i]].Transitions = map[string]string{
			"complete": wb.steps[i+1],
		}
	}
	
	if len(wb.steps) > 0 {
		wb.sm.initialState = wb.steps[0]
	}
	
	return wb.sm
}
