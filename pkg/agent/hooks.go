// Package agent provides lifecycle hooks for agent execution.
package agent

import (
	"context"
	"time"
)

// Hooks provides callbacks for agent lifecycle events.
type Hooks struct {
	// OnStart is called when the agent begins a task.
	OnStart func(ctx context.Context, task string)
	// OnBeforeStep is called before each think/act cycle.
	OnBeforeStep func(ctx context.Context, iteration int)
	// OnAfterStep is called after each step completes.
	OnAfterStep func(ctx context.Context, step StepResult)
	// OnToolCall is called before a tool is executed.
	OnToolCall func(ctx context.Context, toolName string, input string)
	// OnToolResult is called after a tool returns.
	OnToolResult func(ctx context.Context, toolName string, result string, err error)
	// OnThought is called when the agent expresses reasoning.
	OnThought func(ctx context.Context, thought string)
	// OnError is called when an error occurs.
	OnError func(ctx context.Context, err error)
	// OnComplete is called when the agent finishes.
	OnComplete func(ctx context.Context, result *RunResult)
}

// HookBuilder provides a fluent API for building hooks.
type HookBuilder struct {
	hooks Hooks
}

// NewHooks creates a new hook builder.
func NewHooks() *HookBuilder {
	return &HookBuilder{}
}

// OnStart sets the start callback.
func (b *HookBuilder) OnStart(fn func(ctx context.Context, task string)) *HookBuilder {
	b.hooks.OnStart = fn
	return b
}

// OnBeforeStep sets the before-step callback.
func (b *HookBuilder) OnBeforeStep(fn func(ctx context.Context, iteration int)) *HookBuilder {
	b.hooks.OnBeforeStep = fn
	return b
}

// OnAfterStep sets the after-step callback.
func (b *HookBuilder) OnAfterStep(fn func(ctx context.Context, step StepResult)) *HookBuilder {
	b.hooks.OnAfterStep = fn
	return b
}

// OnToolCall sets the tool-call callback.
func (b *HookBuilder) OnToolCall(fn func(ctx context.Context, toolName string, input string)) *HookBuilder {
	b.hooks.OnToolCall = fn
	return b
}

// OnToolResult sets the tool-result callback.
func (b *HookBuilder) OnToolResult(fn func(ctx context.Context, toolName string, result string, err error)) *HookBuilder {
	b.hooks.OnToolResult = fn
	return b
}

// OnThought sets the thought callback.
func (b *HookBuilder) OnThought(fn func(ctx context.Context, thought string)) *HookBuilder {
	b.hooks.OnThought = fn
	return b
}

// OnError sets the error callback.
func (b *HookBuilder) OnError(fn func(ctx context.Context, err error)) *HookBuilder {
	b.hooks.OnError = fn
	return b
}

// OnComplete sets the completion callback.
func (b *HookBuilder) OnComplete(fn func(ctx context.Context, result *RunResult)) *HookBuilder {
	b.hooks.OnComplete = fn
	return b
}

// Build returns the constructed Hooks.
func (b *HookBuilder) Build() Hooks {
	return b.hooks
}

// WithHooks sets hooks on an agent.
func WithHooks(hooks Hooks) Option {
	return func(a *Agent) {
		a.hooks = hooks
	}
}

// LoggingHooks returns hooks that log agent activity.
func LoggingHooks(logFn func(string, ...any)) Hooks {
	return NewHooks().
		OnStart(func(ctx context.Context, task string) {
			logFn("🚀 Agent started: %s", truncate(task, 50))
		}).
		OnBeforeStep(func(ctx context.Context, iteration int) {
			logFn("📍 Step %d starting...", iteration)
		}).
		OnAfterStep(func(ctx context.Context, step StepResult) {
			if step.IsFinal {
				logFn("✅ Final answer reached")
			} else {
				logFn("🔧 Action: %s", step.Action.Action)
			}
		}).
		OnToolCall(func(ctx context.Context, toolName string, input string) {
			logFn("⚙️  Calling tool: %s", toolName)
		}).
		OnToolResult(func(ctx context.Context, toolName string, result string, err error) {
			if err != nil {
				logFn("❌ Tool error: %v", err)
			} else {
				logFn("📋 Tool result: %s", truncate(result, 100))
			}
		}).
		OnError(func(ctx context.Context, err error) {
			logFn("⚠️  Error: %v", err)
		}).
		OnComplete(func(ctx context.Context, result *RunResult) {
			logFn("🏁 Completed in %d steps", result.Iterations)
		}).
		Build()
}

// MetricsHooks returns hooks that collect metrics.
func MetricsHooks() (*Hooks, *AgentMetrics) {
	metrics := &AgentMetrics{}

	hooks := NewHooks().
		OnStart(func(ctx context.Context, task string) {
			metrics.StartTime = time.Now()
		}).
		OnAfterStep(func(ctx context.Context, step StepResult) {
			metrics.TotalSteps++
			if step.Error != nil {
				metrics.Errors++
			}
		}).
		OnToolCall(func(ctx context.Context, toolName string, input string) {
			metrics.ToolCalls++
			if metrics.ToolUsage == nil {
				metrics.ToolUsage = make(map[string]int)
			}
			metrics.ToolUsage[toolName]++
		}).
		OnComplete(func(ctx context.Context, result *RunResult) {
			metrics.EndTime = time.Now()
			metrics.Duration = metrics.EndTime.Sub(metrics.StartTime)
			metrics.Success = result.Error == nil
		}).
		Build()

	return &hooks, metrics
}

// AgentMetrics holds execution metrics.
type AgentMetrics struct {
	StartTime  time.Time
	EndTime    time.Time
	Duration   time.Duration
	TotalSteps int
	ToolCalls  int
	ToolUsage  map[string]int
	Errors     int
	Success    bool
}

// truncate limits a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// DebugHooks returns hooks that print detailed debug information.
func DebugHooks() Hooks {
	return NewHooks().
		OnStart(func(ctx context.Context, task string) {
			println("[DEBUG] Task:", task)
		}).
		OnBeforeStep(func(ctx context.Context, iteration int) {
			println("[DEBUG] === Step", iteration, "===")
		}).
		OnAfterStep(func(ctx context.Context, step StepResult) {
			println("[DEBUG] Action:", step.Action.Action)
			println("[DEBUG] Thought:", step.Action.Thought)
			println("[DEBUG] Observation:", truncate(step.Observation, 200))
			if step.Error != nil {
				println("[DEBUG] Error:", step.Error.Error())
			}
		}).
		OnComplete(func(ctx context.Context, result *RunResult) {
			println("[DEBUG] === Complete ===")
			println("[DEBUG] Output:", truncate(result.Output, 200))
			println("[DEBUG] Iterations:", result.Iterations)
		}).
		Build()
}
