// Package agent provides autonomous AI agent capabilities for GoFlow.
// Agents can reason, plan, and execute tools to accomplish tasks.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nuulab/goflow/pkg/core"
	"github.com/nuulab/goflow/pkg/tools"
)

// Config holds configuration for an Agent.
type Config struct {
	// MaxIterations limits the number of think/act cycles. Default: 10.
	MaxIterations int
	// SystemPrompt is prepended to all conversations.
	SystemPrompt string
	// Verbose enables detailed logging of agent steps.
	Verbose bool
	// StopOnError halts execution on first tool error. Default: false.
	StopOnError bool
}

// DefaultConfig returns sensible defaults for agent configuration.
func DefaultConfig() Config {
	return Config{
		MaxIterations: 10,
		SystemPrompt:  defaultSystemPrompt,
		Verbose:       false,
		StopOnError:   false,
	}
}

const defaultSystemPrompt = `You are a helpful AI assistant that can use tools to accomplish tasks.

When you need to use a tool, respond with a JSON object in this exact format:
{"action": "tool_name", "action_input": {"param1": "value1"}}

When you have the final answer and no more tools are needed, respond with:
{"action": "final_answer", "action_input": "Your final response here"}

Think step by step about what tools you need to use and in what order.
Always explain your reasoning before taking an action.`

// Agent is an autonomous AI that can reason and use tools to complete tasks.
type Agent struct {
	llm      core.LLM
	tools    *tools.Registry
	memory   Memory
	config   Config
	messages []core.Message
	hooks    Hooks
}

// New creates a new Agent with the given LLM and tools.
func New(llm core.LLM, registry *tools.Registry, opts ...Option) *Agent {
	agent := &Agent{
		llm:      llm,
		tools:    registry,
		memory:   NewBufferMemory(20), // Default to 20 message buffer
		config:   DefaultConfig(),
		messages: make([]core.Message, 0),
	}

	for _, opt := range opts {
		opt(agent)
	}

	return agent
}

// Option configures an Agent.
type Option func(*Agent)

// WithConfig sets the agent configuration.
func WithConfig(cfg Config) Option {
	return func(a *Agent) {
		a.config = cfg
	}
}

// WithMemory sets a custom memory implementation.
func WithMemory(mem Memory) Option {
	return func(a *Agent) {
		a.memory = mem
	}
}

// WithMaxIterations sets the maximum number of iterations.
func WithMaxIterations(n int) Option {
	return func(a *Agent) {
		a.config.MaxIterations = n
	}
}

// WithSystemPrompt sets a custom system prompt.
func WithSystemPrompt(prompt string) Option {
	return func(a *Agent) {
		a.config.SystemPrompt = prompt
	}
}

// WithVerbose enables verbose logging.
func WithVerbose(v bool) Option {
	return func(a *Agent) {
		a.config.Verbose = v
	}
}

// AgentAction represents a parsed action from the LLM response.
type AgentAction struct {
	// Action is the tool name to execute.
	Action string `json:"action"`
	// ActionInput is the input to the tool (can be string or object).
	ActionInput json.RawMessage `json:"action_input"`
	// Thought is the reasoning before the action (if extracted).
	Thought string `json:"-"`
	// RawResponse is the full LLM response.
	RawResponse string `json:"-"`
}

// StepResult represents the outcome of a single agent step.
type StepResult struct {
	// Action taken by the agent.
	Action AgentAction
	// Observation from tool execution.
	Observation string
	// Error if the step failed.
	Error error
	// IsFinal indicates this is the final answer.
	IsFinal bool
}

// RunResult represents the final outcome of an agent run.
type RunResult struct {
	// Output is the final answer from the agent.
	Output string
	// Steps contains all intermediate steps taken.
	Steps []StepResult
	// Iterations is the number of think/act cycles performed.
	Iterations int
	// Error if the run failed.
	Error error
	// ToolCalls contains all tool invocations made during execution.
	ToolCalls []ToolCallRecord
}

// ToolCallRecord represents a tool invocation during agent execution.
type ToolCallRecord struct {
	Name   string
	Input  string
	Output string
}

// Run executes the agent on a task until completion or max iterations.
func (a *Agent) Run(ctx context.Context, task string) (*RunResult, error) {
	// Initialize conversation
	a.messages = []core.Message{
		{Role: core.RoleSystem, Content: a.buildSystemPrompt()},
		{Role: core.RoleUser, Content: task},
	}

	// Store in memory
	a.memory.Add(core.Message{Role: core.RoleUser, Content: task})

	result := &RunResult{
		Steps: make([]StepResult, 0),
	}

	for i := 0; i < a.config.MaxIterations; i++ {
		select {
		case <-ctx.Done():
			result.Error = ctx.Err()
			return result, ctx.Err()
		default:
		}

		result.Iterations = i + 1

		// Execute one step
		stepResult, err := a.Step(ctx)
		if err != nil {
			result.Error = err
			if a.config.StopOnError {
				return result, err
			}
		}

		result.Steps = append(result.Steps, stepResult)

		// Check if we have a final answer
		if stepResult.IsFinal {
			result.Output = stepResult.Observation
			return result, nil
		}

		// Add observation to conversation for next iteration
		if stepResult.Observation != "" {
			obsMsg := core.Message{
				Role:    core.RoleUser,
				Content: fmt.Sprintf("Observation: %s", stepResult.Observation),
			}
			a.messages = append(a.messages, obsMsg)
			a.memory.Add(obsMsg)
		}
	}

	result.Error = fmt.Errorf("agent reached max iterations (%d) without final answer", a.config.MaxIterations)
	return result, result.Error
}

// Step executes a single think/act cycle.
func (a *Agent) Step(ctx context.Context) (StepResult, error) {
	var result StepResult

	// Get LLM response
	response, err := a.llm.GenerateChat(ctx, a.messages)
	if err != nil {
		result.Error = fmt.Errorf("LLM generation failed: %w", err)
		return result, result.Error
	}

	// Add assistant response to messages
	a.messages = append(a.messages, core.Message{
		Role:    core.RoleAssistant,
		Content: response,
	})
	a.memory.Add(core.Message{Role: core.RoleAssistant, Content: response})

	// Parse the action from response
	action, err := a.parseAction(response)
	if err != nil {
		result.Error = fmt.Errorf("failed to parse action: %w", err)
		result.Observation = fmt.Sprintf("Error: Could not parse your response as a valid action. Please respond with valid JSON. Error: %s", err)
		return result, nil // Don't return error, let agent self-correct
	}

	result.Action = action

	// Check for final answer
	if action.Action == "final_answer" {
		result.IsFinal = true
		// ActionInput could be a string or object, try to extract
		var strInput string
		if err := json.Unmarshal(action.ActionInput, &strInput); err != nil {
			// Not a string, use raw JSON
			result.Observation = string(action.ActionInput)
		} else {
			result.Observation = strInput
		}
		return result, nil
	}

	// Execute the tool
	observation, err := a.executeTool(ctx, action)
	if err != nil {
		result.Error = err
		result.Observation = fmt.Sprintf("Error executing tool '%s': %s", action.Action, err)
	} else {
		result.Observation = observation
	}

	return result, nil
}

// buildSystemPrompt constructs the full system prompt with tool descriptions.
func (a *Agent) buildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString(a.config.SystemPrompt)
	sb.WriteString("\n\nAvailable tools:\n")

	for _, tool := range a.tools.List() {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", tool.Name, tool.Description))
	}

	return sb.String()
}

// parseAction extracts the action JSON from the LLM response.
func (a *Agent) parseAction(response string) (AgentAction, error) {
	var action AgentAction
	action.RawResponse = response

	// Try to find JSON in the response
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")

	if start == -1 || end == -1 || end < start {
		return action, fmt.Errorf("no JSON object found in response")
	}

	// Extract thought (text before JSON)
	if start > 0 {
		action.Thought = strings.TrimSpace(response[:start])
	}

	jsonStr := response[start : end+1]

	if err := json.Unmarshal([]byte(jsonStr), &action); err != nil {
		return action, fmt.Errorf("invalid JSON: %w", err)
	}

	if action.Action == "" {
		return action, fmt.Errorf("action field is required")
	}

	return action, nil
}

// executeTool runs the specified tool with the given input.
func (a *Agent) executeTool(ctx context.Context, action AgentAction) (string, error) {
	tool, exists := a.tools.Get(action.Action)
	if !exists {
		return "", fmt.Errorf("unknown tool: %s", action.Action)
	}

	// Convert ActionInput to string for tool execution
	inputStr := string(action.ActionInput)

	// If it's a quoted string, unquote it
	var strInput string
	if err := json.Unmarshal(action.ActionInput, &strInput); err == nil {
		// It was a JSON string, use the unquoted value for simple tools
		// But pass as JSON object for complex tools
		inputStr = string(action.ActionInput)
	}

	return tool.Execute(ctx, inputStr)
}

// GetMessages returns the current conversation messages.
func (a *Agent) GetMessages() []core.Message {
	return append([]core.Message{}, a.messages...)
}

// Reset clears the agent's conversation state.
func (a *Agent) Reset() {
	a.messages = make([]core.Message, 0)
	a.memory.Clear()
}

// Name returns the agent's name.
func (a *Agent) Name() string {
	// Extract a name from the system prompt or return default
	if strings.HasPrefix(a.config.SystemPrompt, "You are a ") || strings.HasPrefix(a.config.SystemPrompt, "You are an ") {
		parts := strings.SplitN(a.config.SystemPrompt, ".", 2)
		if len(parts) > 0 {
			name := strings.TrimPrefix(parts[0], "You are a ")
			name = strings.TrimPrefix(name, "You are an ")
			return strings.TrimSpace(name)
		}
	}
	return "agent"
}
