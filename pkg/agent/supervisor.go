// Package agent provides multi-agent orchestration capabilities.
package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/nuulab/goflow/pkg/core"
	"github.com/nuulab/goflow/pkg/tools"
)

// Supervisor orchestrates multiple specialized agents.
type Supervisor struct {
	llm      core.LLM
	agents   map[string]*Agent
	router   Router
	config   SupervisorConfig
	messages []core.Message
}

// SupervisorConfig holds configuration for the supervisor.
type SupervisorConfig struct {
	// MaxDelegations limits how many times work can be delegated.
	MaxDelegations int
	// SystemPrompt for the supervisor's routing decisions.
	SystemPrompt string
}

// DefaultSupervisorConfig returns sensible defaults.
func DefaultSupervisorConfig() SupervisorConfig {
	return SupervisorConfig{
		MaxDelegations: 5,
		SystemPrompt: `You are a supervisor that routes tasks to specialized agents.
Analyze the user's request and decide which agent should handle it.
Respond with the agent name that should handle this task.`,
	}
}

// Router determines which agent should handle a task.
type Router interface {
	Route(ctx context.Context, task string, available []string) (string, error)
}

// LLMRouter uses an LLM to decide routing.
type LLMRouter struct {
	llm core.LLM
}

// NewLLMRouter creates a router that uses an LLM for decisions.
func NewLLMRouter(llm core.LLM) *LLMRouter {
	return &LLMRouter{llm: llm}
}

// Route asks the LLM which agent should handle the task.
func (r *LLMRouter) Route(ctx context.Context, task string, available []string) (string, error) {
	prompt := fmt.Sprintf(`Given this task: "%s"

Which of these agents should handle it?
Available agents: %v

Respond with just the agent name, nothing else.`, task, available)

	response, err := r.llm.Generate(ctx, prompt)
	if err != nil {
		return "", err
	}

	// Find matching agent
	for _, name := range available {
		if name == response || fmt.Sprintf("%q", name) == response {
			return name, nil
		}
	}

	// Default to first agent
	if len(available) > 0 {
		return available[0], nil
	}

	return "", fmt.Errorf("no agents available")
}

// KeywordRouter routes based on keywords in the task.
type KeywordRouter struct {
	keywords map[string][]string // agent -> keywords
}

// NewKeywordRouter creates a keyword-based router.
func NewKeywordRouter() *KeywordRouter {
	return &KeywordRouter{
		keywords: make(map[string][]string),
	}
}

// AddKeywords associates keywords with an agent.
func (r *KeywordRouter) AddKeywords(agent string, words ...string) *KeywordRouter {
	r.keywords[agent] = append(r.keywords[agent], words...)
	return r
}

// Route finds the agent with the most matching keywords.
func (r *KeywordRouter) Route(ctx context.Context, task string, available []string) (string, error) {
	scores := make(map[string]int)

	for agent, words := range r.keywords {
		for _, word := range words {
			if contains(task, word) {
				scores[agent]++
			}
		}
	}

	// Find best match
	var best string
	var bestScore int
	for _, agent := range available {
		if scores[agent] > bestScore {
			best = agent
			bestScore = scores[agent]
		}
	}

	if best == "" && len(available) > 0 {
		return available[0], nil
	}

	return best, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsIgnoreCase(s, substr))
}

func containsIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalIgnoreCase(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// NewSupervisor creates a new supervisor.
func NewSupervisor(llm core.LLM, router Router) *Supervisor {
	return &Supervisor{
		llm:    llm,
		agents: make(map[string]*Agent),
		router: router,
		config: DefaultSupervisorConfig(),
	}
}

// AddAgent registers a specialized agent.
func (s *Supervisor) AddAgent(name string, agent *Agent) *Supervisor {
	s.agents[name] = agent
	return s
}

// CreateAgent creates and registers a new agent with the given tools.
func (s *Supervisor) CreateAgent(name string, registry *tools.Registry, opts ...Option) *Supervisor {
	agent := New(s.llm, registry, opts...)
	s.agents[name] = agent
	return s
}

// Run executes a task by routing to the appropriate agent.
func (s *Supervisor) Run(ctx context.Context, task string) (*SupervisorResult, error) {
	result := &SupervisorResult{
		Delegations: make([]Delegation, 0),
	}

	// Get available agents
	available := make([]string, 0, len(s.agents))
	for name := range s.agents {
		available = append(available, name)
	}

	if len(available) == 0 {
		return nil, fmt.Errorf("no agents registered")
	}

	// Route to appropriate agent
	agentName, err := s.router.Route(ctx, task, available)
	if err != nil {
		return nil, fmt.Errorf("routing failed: %w", err)
	}

	agent, ok := s.agents[agentName]
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", agentName)
	}

	// Execute
	agentResult, err := agent.Run(ctx, task)
	
	result.Delegations = append(result.Delegations, Delegation{
		Agent:  agentName,
		Task:   task,
		Result: agentResult,
		Error:  err,
	})

	if err != nil {
		result.Error = err
		return result, err
	}

	result.Output = agentResult.Output
	return result, nil
}

// SupervisorResult holds the outcome of a supervised run.
type SupervisorResult struct {
	Output      string
	Delegations []Delegation
	Error       error
}

// Delegation represents a task delegation to an agent.
type Delegation struct {
	Agent  string
	Task   string
	Result *RunResult
	Error  error
}

// Team executes multiple agents in parallel.
type Team struct {
	agents map[string]*Agent
	llm    core.LLM
}

// NewTeam creates a new agent team.
func NewTeam(llm core.LLM) *Team {
	return &Team{
		agents: make(map[string]*Agent),
		llm:    llm,
	}
}

// AddAgent adds an agent to the team.
func (t *Team) AddAgent(name string, agent *Agent) *Team {
	t.agents[name] = agent
	return t
}

// RunAll executes the task on all agents in parallel.
func (t *Team) RunAll(ctx context.Context, task string) map[string]*RunResult {
	results := make(map[string]*RunResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, agent := range t.agents {
		wg.Add(1)
		go func(n string, a *Agent) {
			defer wg.Done()
			result, _ := a.Run(ctx, task)
			mu.Lock()
			results[n] = result
			mu.Unlock()
		}(name, agent)
	}

	wg.Wait()
	return results
}

// RunSelected executes the task on selected agents.
func (t *Team) RunSelected(ctx context.Context, task string, names ...string) map[string]*RunResult {
	results := make(map[string]*RunResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, name := range names {
		if agent, ok := t.agents[name]; ok {
			wg.Add(1)
			go func(n string, a *Agent) {
				defer wg.Done()
				result, _ := a.Run(ctx, task)
				mu.Lock()
				results[n] = result
				mu.Unlock()
			}(name, agent)
		}
	}

	wg.Wait()
	return results
}

// Pipeline runs agents sequentially, passing output to the next.
type Pipeline struct {
	agents []*Agent
}

// NewPipeline creates a sequential agent pipeline.
func NewPipeline() *Pipeline {
	return &Pipeline{
		agents: make([]*Agent, 0),
	}
}

// Add appends an agent to the pipeline.
func (p *Pipeline) Add(agent *Agent) *Pipeline {
	p.agents = append(p.agents, agent)
	return p
}

// Run executes agents in sequence.
func (p *Pipeline) Run(ctx context.Context, task string) (*RunResult, error) {
	currentTask := task
	var lastResult *RunResult

	for i, agent := range p.agents {
		result, err := agent.Run(ctx, currentTask)
		if err != nil {
			return result, fmt.Errorf("agent %d failed: %w", i, err)
		}
		lastResult = result
		currentTask = result.Output // Pass output as next task
		agent.Reset()
	}

	return lastResult, nil
}
