// Package agent provides the execution loop implementations.
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nuulab/goflow/pkg/core"
	"github.com/nuulab/goflow/pkg/tools"
)

// LoopType defines the type of agent execution loop.
type LoopType string

const (
	// LoopReAct uses the ReAct (Reason + Act) pattern.
	LoopReAct LoopType = "react"
	// LoopPlanExecute creates a plan upfront then executes it.
	LoopPlanExecute LoopType = "plan_execute"
)

// ReActLoop implements the ReAct (Reasoning and Acting) pattern.
// The agent alternates between thinking and acting until done.
type ReActLoop struct {
	agent         *Agent
	maxIterations int
	onStep        func(step StepResult)
}

// NewReActLoop creates a new ReAct execution loop.
func NewReActLoop(agent *Agent, maxIter int) *ReActLoop {
	return &ReActLoop{
		agent:         agent,
		maxIterations: maxIter,
	}
}

// OnStep sets a callback for each step completion.
func (r *ReActLoop) OnStep(fn func(StepResult)) *ReActLoop {
	r.onStep = fn
	return r
}

// Execute runs the ReAct loop until completion.
func (r *ReActLoop) Execute(ctx context.Context, task string) (*RunResult, error) {
	return r.agent.Run(ctx, task)
}

// PlanExecuteLoop creates an upfront plan and then executes steps sequentially.
type PlanExecuteLoop struct {
	llm           core.LLM
	tools         *tools.Registry
	maxIterations int
	planPrompt    string
}

// Plan represents a multi-step execution plan.
type Plan struct {
	Goal  string   `json:"goal"`
	Steps []string `json:"steps"`
}

// PlanStepResult represents the result of executing a plan step.
type PlanStepResult struct {
	StepIndex   int
	StepText    string
	Observation string
	Error       error
}

// PlanExecuteResult represents the outcome of plan-and-execute.
type PlanExecuteResult struct {
	Plan        Plan
	StepResults []PlanStepResult
	FinalOutput string
	Error       error
}

// NewPlanExecuteLoop creates a plan-and-execute loop.
func NewPlanExecuteLoop(llm core.LLM, registry *tools.Registry) *PlanExecuteLoop {
	return &PlanExecuteLoop{
		llm:           llm,
		tools:         registry,
		maxIterations: 20,
		planPrompt:    defaultPlanPrompt,
	}
}

const defaultPlanPrompt = `Given the following task, create a step-by-step plan to accomplish it.
Output your plan as a JSON object with this format:
{"goal": "the overall goal", "steps": ["step 1", "step 2", "step 3"]}

Task: %s`

// Execute creates a plan and executes it step by step.
func (p *PlanExecuteLoop) Execute(ctx context.Context, task string) (*PlanExecuteResult, error) {
	result := &PlanExecuteResult{
		StepResults: make([]PlanStepResult, 0),
	}

	// Step 1: Generate the plan
	planPrompt := fmt.Sprintf(p.planPrompt, task)
	planResponse, err := p.llm.Generate(ctx, planPrompt)
	if err != nil {
		result.Error = fmt.Errorf("failed to generate plan: %w", err)
		return result, result.Error
	}

	// Parse the plan
	plan, err := parsePlan(planResponse)
	if err != nil {
		result.Error = fmt.Errorf("failed to parse plan: %w", err)
		return result, result.Error
	}
	result.Plan = plan

	// Step 2: Execute each step
	agent := New(p.llm, p.tools, WithMaxIterations(5))

	for i, step := range plan.Steps {
		select {
		case <-ctx.Done():
			result.Error = ctx.Err()
			return result, ctx.Err()
		default:
		}

		stepResult := PlanStepResult{
			StepIndex: i,
			StepText:  step,
		}

		// Execute this step using the agent
		runResult, err := agent.Run(ctx, fmt.Sprintf("Execute this step: %s\n\nContext from previous steps:\n%s", step, summarizeSteps(result.StepResults)))
		if err != nil {
			stepResult.Error = err
			stepResult.Observation = fmt.Sprintf("Error: %s", err)
		} else {
			stepResult.Observation = runResult.Output
		}

		result.StepResults = append(result.StepResults, stepResult)

		// Reset agent for next step
		agent.Reset()
	}

	// Generate final summary
	if len(result.StepResults) > 0 {
		result.FinalOutput = summarizeSteps(result.StepResults)
	}

	return result, nil
}

// parsePlan extracts a Plan from LLM response.
func parsePlan(response string) (Plan, error) {
	var plan Plan

	// Find JSON in response
	start := -1
	end := -1
	braceCount := 0

	for i, c := range response {
		if c == '{' {
			if start == -1 {
				start = i
			}
			braceCount++
		} else if c == '}' {
			braceCount--
			if braceCount == 0 && start != -1 {
				end = i
				break
			}
		}
	}

	if start == -1 || end == -1 {
		return plan, fmt.Errorf("no valid JSON found in response")
	}

	jsonStr := response[start : end+1]
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return plan, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	if len(plan.Steps) == 0 {
		return plan, fmt.Errorf("plan has no steps")
	}

	return plan, nil
}

// summarizeSteps creates a summary of completed steps.
func summarizeSteps(steps []PlanStepResult) string {
	if len(steps) == 0 {
		return "No steps completed yet."
	}

	var summary string
	for _, step := range steps {
		status := "✓"
		if step.Error != nil {
			status = "✗"
		}
		summary += fmt.Sprintf("%s Step %d: %s\n  Result: %s\n", status, step.StepIndex+1, step.StepText, step.Observation)
	}
	return summary
}

// StreamingLoop provides real-time streaming of agent execution.
type StreamingLoop struct {
	agent     *Agent
	onThought func(thought string)
	onAction  func(action AgentAction)
	onResult  func(observation string)
}

// NewStreamingLoop creates a streaming execution loop.
func NewStreamingLoop(agent *Agent) *StreamingLoop {
	return &StreamingLoop{agent: agent}
}

// OnThought sets callback for agent thoughts.
func (s *StreamingLoop) OnThought(fn func(string)) *StreamingLoop {
	s.onThought = fn
	return s
}

// OnAction sets callback for agent actions.
func (s *StreamingLoop) OnAction(fn func(AgentAction)) *StreamingLoop {
	s.onAction = fn
	return s
}

// OnResult sets callback for tool results.
func (s *StreamingLoop) OnResult(fn func(string)) *StreamingLoop {
	s.onResult = fn
	return s
}

// Execute runs the agent with streaming callbacks.
func (s *StreamingLoop) Execute(ctx context.Context, task string) (*RunResult, error) {
	// Initialize conversation
	s.agent.messages = []core.Message{
		{Role: core.RoleSystem, Content: s.agent.buildSystemPrompt()},
		{Role: core.RoleUser, Content: task},
	}

	result := &RunResult{Steps: make([]StepResult, 0)}

	for i := 0; i < s.agent.config.MaxIterations; i++ {
		select {
		case <-ctx.Done():
			result.Error = ctx.Err()
			return result, ctx.Err()
		default:
		}

		result.Iterations = i + 1

		stepResult, err := s.agent.Step(ctx)
		if err != nil && s.agent.config.StopOnError {
			result.Error = err
			return result, err
		}

		// Fire callbacks
		if s.onThought != nil && stepResult.Action.Thought != "" {
			s.onThought(stepResult.Action.Thought)
		}
		if s.onAction != nil {
			s.onAction(stepResult.Action)
		}
		if s.onResult != nil && stepResult.Observation != "" {
			s.onResult(stepResult.Observation)
		}

		result.Steps = append(result.Steps, stepResult)

		if stepResult.IsFinal {
			result.Output = stepResult.Observation
			return result, nil
		}

		// Continue to next iteration
		if stepResult.Observation != "" {
			obsMsg := core.Message{
				Role:    core.RoleUser,
				Content: fmt.Sprintf("Observation: %s", stepResult.Observation),
			}
			s.agent.messages = append(s.agent.messages, obsMsg)
		}
	}

	result.Error = fmt.Errorf("max iterations reached")
	return result, result.Error
}
