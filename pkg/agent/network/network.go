// Package network provides multi-agent network orchestration.
package network

import (
	"context"
	"fmt"
	"time"

	"github.com/nuulab/goflow/pkg/agent"
	"github.com/nuulab/goflow/pkg/agent/state"
	"github.com/nuulab/goflow/pkg/core"
)

// Network orchestrates multiple agents with shared state and routing.
type Network[T any] struct {
	name         string
	agents       map[string]*agent.Agent
	router       Router[T]
	defaultModel core.LLM
	state        *state.State[T]
	maxIter      int
	hooks        *NetworkHooks[T]
}

// Router determines which agent to run next.
type Router[T any] interface {
	Route(ctx *RoutingContext[T]) *agent.Agent
}

// RoutingContext provides context for routing decisions.
type RoutingContext[T any] struct {
	Network     *Network[T]
	State       *state.State[T]
	LastResult  *state.HistoryEntry
	CallCount   int
	Input       string
	Agents      map[string]*agent.Agent
}

// NetworkHooks provides lifecycle hooks for network execution.
type NetworkHooks[T any] struct {
	OnStart      func(input string, state *state.State[T])
	OnAgentStart func(agent *agent.Agent, input string)
	OnAgentEnd   func(agent *agent.Agent, result *agent.RunResult)
	OnRoute      func(selected *agent.Agent, ctx *RoutingContext[T])
	OnComplete   func(state *state.State[T])
	OnError      func(err error)
}

// Config configures a network.
type Config[T any] struct {
	Name         string
	Agents       []*agent.Agent
	DefaultModel core.LLM
	Router       Router[T]
	MaxIter      int
	InitialState T
	Hooks        *NetworkHooks[T]
}

// New creates a new agent network.
func New[T any](cfg Config[T]) *Network[T] {
	agents := make(map[string]*agent.Agent)
	for _, a := range cfg.Agents {
		agents[a.Name()] = a
	}

	maxIter := cfg.MaxIter
	if maxIter == 0 {
		maxIter = 10
	}

	return &Network[T]{
		name:         cfg.Name,
		agents:       agents,
		router:       cfg.Router,
		defaultModel: cfg.DefaultModel,
		state:        state.New(cfg.InitialState),
		maxIter:      maxIter,
		hooks:        cfg.Hooks,
	}
}

// Run executes the network with the given input.
func (n *Network[T]) Run(ctx context.Context, input string) (*NetworkResult[T], error) {
	if n.hooks != nil && n.hooks.OnStart != nil {
		n.hooks.OnStart(input, n.state)
	}

	var lastResult *agent.RunResult
	currentInput := input

	for i := 0; i < n.maxIter; i++ {
		// Build routing context
		routingCtx := &RoutingContext[T]{
			Network:    n,
			State:      n.state,
			LastResult: n.state.LastResult(),
			CallCount:  n.state.CallCount(),
			Input:      currentInput,
			Agents:     n.agents,
		}

		// Route to next agent
		selectedAgent := n.router.Route(routingCtx)
		if selectedAgent == nil {
			// No agent selected, we're done
			break
		}

		if n.hooks != nil && n.hooks.OnRoute != nil {
			n.hooks.OnRoute(selectedAgent, routingCtx)
		}

		if n.hooks != nil && n.hooks.OnAgentStart != nil {
			n.hooks.OnAgentStart(selectedAgent, currentInput)
		}

		// Run the agent
		start := time.Now()
		result, err := selectedAgent.Run(ctx, currentInput)
		if err != nil {
			if n.hooks != nil && n.hooks.OnError != nil {
				n.hooks.OnError(err)
			}
			return nil, fmt.Errorf("agent %s failed: %w", selectedAgent.Name(), err)
		}

		// Record in history
		entry := state.HistoryEntry{
			AgentName: selectedAgent.Name(),
			Input:     currentInput,
			Output:    result.Output,
			Duration:  time.Since(start),
		}
		for _, tc := range result.ToolCalls {
			entry.ToolCalls = append(entry.ToolCalls, state.ToolCallInfo{
				Name:   tc.Name,
				Input:  tc.Input,
				Output: tc.Output,
			})
		}
		n.state.AddHistory(entry)

		if n.hooks != nil && n.hooks.OnAgentEnd != nil {
			n.hooks.OnAgentEnd(selectedAgent, result)
		}

		lastResult = result
		currentInput = result.Output
	}

	if n.hooks != nil && n.hooks.OnComplete != nil {
		n.hooks.OnComplete(n.state)
	}

	return &NetworkResult[T]{
		Output:  lastResult.Output,
		State:   n.state,
		History: n.state.History(),
	}, nil
}

// NetworkResult is the result of running a network.
type NetworkResult[T any] struct {
	Output  string
	State   *state.State[T]
	History []state.HistoryEntry
}

// State returns the network's shared state.
func (n *Network[T]) State() *state.State[T] {
	return n.state
}

// Agent returns an agent by name.
func (n *Network[T]) Agent(name string) *agent.Agent {
	return n.agents[name]
}

// AddAgent adds an agent to the network.
func (n *Network[T]) AddAgent(a *agent.Agent) {
	n.agents[a.Name()] = a
}

// ============ Built-in Routers ============

// CodeRouter routes based on custom logic.
type CodeRouter[T any] struct {
	fn func(ctx *RoutingContext[T]) *agent.Agent
}

// NewCodeRouter creates a code-based router.
func NewCodeRouter[T any](fn func(ctx *RoutingContext[T]) *agent.Agent) *CodeRouter[T] {
	return &CodeRouter[T]{fn: fn}
}

func (r *CodeRouter[T]) Route(ctx *RoutingContext[T]) *agent.Agent {
	return r.fn(ctx)
}

// SequentialRouter routes agents in order.
type SequentialRouter[T any] struct {
	agents []*agent.Agent
}

// NewSequentialRouter creates a router that runs agents in sequence.
func NewSequentialRouter[T any](agents ...*agent.Agent) *SequentialRouter[T] {
	return &SequentialRouter[T]{agents: agents}
}

func (r *SequentialRouter[T]) Route(ctx *RoutingContext[T]) *agent.Agent {
	if ctx.CallCount < len(r.agents) {
		return r.agents[ctx.CallCount]
	}
	return nil
}

// HybridRouter combines code-based and LLM-based routing.
type HybridRouter[T any] struct {
	llm          core.LLM
	codeFirst    func(ctx *RoutingContext[T]) *agent.Agent
	useAgent     func(ctx *RoutingContext[T]) bool
	agentPrompt  string
}

// NewHybridRouter creates a hybrid router.
// codeFirst runs first; if useAgent returns true, falls back to LLM routing.
func NewHybridRouter[T any](llm core.LLM, codeFirst func(ctx *RoutingContext[T]) *agent.Agent, useAgent func(ctx *RoutingContext[T]) bool) *HybridRouter[T] {
	return &HybridRouter[T]{
		llm:       llm,
		codeFirst: codeFirst,
		useAgent:  useAgent,
		agentPrompt: `You are a routing agent. Given the current state and available agents, select the most appropriate agent to handle the task.
Available agents: %s
Current input: %s
Last output: %s

Respond with ONLY the agent name, or "DONE" if the task is complete.`,
	}
}

func (r *HybridRouter[T]) Route(ctx *RoutingContext[T]) *agent.Agent {
	// Try code-based first
	if selected := r.codeFirst(ctx); selected != nil {
		return selected
	}

	// Check if we should use agent routing
	if r.useAgent != nil && !r.useAgent(ctx) {
		return nil
	}

	// Fall back to LLM routing
	agentNames := ""
	for name := range ctx.Agents {
		agentNames += name + ", "
	}

	lastOutput := ""
	if ctx.LastResult != nil {
		lastOutput = ctx.LastResult.Output
	}

	prompt := fmt.Sprintf(r.agentPrompt, agentNames, ctx.Input, lastOutput)
	resp, err := r.llm.GenerateChat(context.Background(), []core.Message{{Role: core.RoleUser, Content: prompt}})
	if err != nil {
		return nil
	}

	if resp == "DONE" {
		return nil
	}

	return ctx.Agents[resp]
}
