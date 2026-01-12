// Package agent provides consensus and voting mechanisms for multi-agent decisions.
package agent

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/nuulab/goflow/pkg/core"
)

// ConsensusStrategy defines how to reach consensus.
type ConsensusStrategy int

const (
	// MajorityVote requires more than half to agree.
	MajorityVote ConsensusStrategy = iota
	// UnanimousVote requires all to agree.
	UnanimousVote
	// PluralityVote picks the option with most votes.
	PluralityVote
	// WeightedVote uses weighted voting.
	WeightedVote
	// LLMJudge uses an LLM to judge the best answer.
	LLMJudge
)

// Consensus orchestrates multiple agents to reach agreement.
type Consensus struct {
	llm      core.LLM
	agents   []*Agent
	weights  map[*Agent]float64
	strategy ConsensusStrategy
	judge    core.LLM
}

// NewConsensus creates a new consensus mechanism.
func NewConsensus(llm core.LLM) *Consensus {
	return &Consensus{
		llm:      llm,
		agents:   make([]*Agent, 0),
		weights:  make(map[*Agent]float64),
		strategy: MajorityVote,
	}
}

// WithVoters adds agents as voters.
func (c *Consensus) WithVoters(agents ...*Agent) *Consensus {
	c.agents = append(c.agents, agents...)
	for _, a := range agents {
		c.weights[a] = 1.0
	}
	return c
}

// WithWeightedVoter adds an agent with a specific voting weight.
func (c *Consensus) WithWeightedVoter(agent *Agent, weight float64) *Consensus {
	c.agents = append(c.agents, agent)
	c.weights[agent] = weight
	return c
}

// WithStrategy sets the consensus strategy.
func (c *Consensus) WithStrategy(strategy ConsensusStrategy) *Consensus {
	c.strategy = strategy
	return c
}

// WithJudge sets a judge LLM for LLMJudge strategy.
func (c *Consensus) WithJudge(judge core.LLM) *Consensus {
	c.judge = judge
	return c
}

// Vote represents a single agent's vote.
type Vote struct {
	Agent    *Agent
	Response string
	Weight   float64
}

// ConsensusResult holds the outcome of a consensus decision.
type ConsensusResult struct {
	Decision    string
	Votes       []Vote
	Agreement   float64 // Percentage of agreement
	Unanimous   bool
	VoteCounts  map[string]int
}

// Decide runs all agents and reaches a consensus.
func (c *Consensus) Decide(ctx context.Context, question string) (*ConsensusResult, error) {
	if len(c.agents) == 0 {
		return nil, fmt.Errorf("no voters configured")
	}
	
	// Collect votes in parallel
	votes := make([]Vote, len(c.agents))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	
	for i, agent := range c.agents {
		wg.Add(1)
		go func(idx int, a *Agent) {
			defer wg.Done()
			
			result, err := a.Run(ctx, question)
			mu.Lock()
			defer mu.Unlock()
			
			if err != nil && firstErr == nil {
				firstErr = err
				return
			}
			
			votes[idx] = Vote{
				Agent:    a,
				Response: result.Output,
				Weight:   c.weights[a],
			}
		}(i, agent)
	}
	
	wg.Wait()
	
	if firstErr != nil {
		return nil, firstErr
	}
	
	// Apply consensus strategy
	return c.applyStrategy(ctx, question, votes)
}

func (c *Consensus) applyStrategy(ctx context.Context, question string, votes []Vote) (*ConsensusResult, error) {
	result := &ConsensusResult{
		Votes:      votes,
		VoteCounts: make(map[string]int),
	}
	
	// Count votes
	for _, v := range votes {
		result.VoteCounts[v.Response]++
	}
	
	switch c.strategy {
	case MajorityVote, PluralityVote:
		return c.pluralityDecision(result)
	case UnanimousVote:
		return c.unanimousDecision(result)
	case WeightedVote:
		return c.weightedDecision(votes, result)
	case LLMJudge:
		return c.judgeDecision(ctx, question, votes, result)
	default:
		return c.pluralityDecision(result)
	}
}

func (c *Consensus) pluralityDecision(result *ConsensusResult) (*ConsensusResult, error) {
	var maxCount int
	for response, count := range result.VoteCounts {
		if count > maxCount {
			maxCount = count
			result.Decision = response
		}
	}
	
	result.Agreement = float64(maxCount) / float64(len(result.Votes))
	result.Unanimous = maxCount == len(result.Votes)
	
	if c.strategy == MajorityVote && result.Agreement <= 0.5 {
		return result, fmt.Errorf("no majority reached (%.0f%% agreement)", result.Agreement*100)
	}
	
	return result, nil
}

func (c *Consensus) unanimousDecision(result *ConsensusResult) (*ConsensusResult, error) {
	if len(result.VoteCounts) == 1 {
		for response := range result.VoteCounts {
			result.Decision = response
			result.Agreement = 1.0
			result.Unanimous = true
			return result, nil
		}
	}
	
	return result, fmt.Errorf("no unanimous decision (got %d different responses)", len(result.VoteCounts))
}

func (c *Consensus) weightedDecision(votes []Vote, result *ConsensusResult) (*ConsensusResult, error) {
	weightedCounts := make(map[string]float64)
	var totalWeight float64
	
	for _, v := range votes {
		weightedCounts[v.Response] += v.Weight
		totalWeight += v.Weight
	}
	
	var maxWeight float64
	for response, weight := range weightedCounts {
		if weight > maxWeight {
			maxWeight = weight
			result.Decision = response
		}
	}
	
	result.Agreement = maxWeight / totalWeight
	return result, nil
}

func (c *Consensus) judgeDecision(ctx context.Context, question string, votes []Vote, result *ConsensusResult) (*ConsensusResult, error) {
	judge := c.judge
	if judge == nil {
		judge = c.llm
	}
	
	// Build prompt for judge
	prompt := fmt.Sprintf(`You are a judge evaluating multiple responses to a question.

Question: %s

Responses:
`, question)
	
	for i, v := range votes {
		prompt += fmt.Sprintf("\nResponse %d:\n%s\n", i+1, v.Response)
	}
	
	prompt += "\nWhich response is the best? Respond with just the response number (1, 2, etc.) and then explain briefly why."
	
	judgment, err := judge.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("judge failed: %w", err)
	}
	
	// Parse the judgment (simple: take first digit)
	for _, r := range judgment {
		if r >= '1' && r <= '9' {
			idx := int(r - '1')
			if idx < len(votes) {
				result.Decision = votes[idx].Response
				result.Agreement = 1.0 / float64(len(votes)) // Judge picked one
				return result, nil
			}
		}
	}
	
	// Fallback to first vote
	result.Decision = votes[0].Response
	return result, nil
}

// Debate runs agents in a debate format where they can respond to each other.
type Debate struct {
	llm        core.LLM
	agents     []*Agent
	rounds     int
	moderator  core.LLM
}

// NewDebate creates a new debate orchestrator.
func NewDebate(llm core.LLM) *Debate {
	return &Debate{
		llm:    llm,
		agents: make([]*Agent, 0),
		rounds: 3,
	}
}

// WithDebaters adds agents as debaters.
func (d *Debate) WithDebaters(agents ...*Agent) *Debate {
	d.agents = append(d.agents, agents...)
	return d
}

// WithRounds sets the number of debate rounds.
func (d *Debate) WithRounds(rounds int) *Debate {
	d.rounds = rounds
	return d
}

// WithModerator sets a moderator LLM.
func (d *Debate) WithModerator(moderator core.LLM) *Debate {
	d.moderator = moderator
	return d
}

// DebateResult holds the outcome of a debate.
type DebateResult struct {
	Topic      string
	Rounds     []DebateRound
	Conclusion string
}

// DebateRound represents one round of debate.
type DebateRound struct {
	RoundNum   int
	Statements []DebateStatement
}

// DebateStatement is a single statement in a debate.
type DebateStatement struct {
	AgentIndex int
	Content    string
}

// Run executes the debate.
func (d *Debate) Run(ctx context.Context, topic string) (*DebateResult, error) {
	result := &DebateResult{
		Topic:  topic,
		Rounds: make([]DebateRound, 0, d.rounds),
	}
	
	// Initial statements
	prevStatements := make([]string, len(d.agents))
	
	for round := 0; round < d.rounds; round++ {
		roundResult := DebateRound{
			RoundNum:   round + 1,
			Statements: make([]DebateStatement, 0, len(d.agents)),
		}
		
		for i, agent := range d.agents {
			prompt := fmt.Sprintf("Topic: %s\n\n", topic)
			
			if round > 0 {
				prompt += "Previous statements from other participants:\n"
				for j, stmt := range prevStatements {
					if j != i && stmt != "" {
						prompt += fmt.Sprintf("- Participant %d: %s\n", j+1, stmt)
					}
				}
				prompt += "\nProvide your response, considering and addressing the other viewpoints."
			} else {
				prompt += "Provide your initial position on this topic."
			}
			
			agentResult, err := agent.Run(ctx, prompt)
			if err != nil {
				continue
			}
			
			prevStatements[i] = agentResult.Output
			roundResult.Statements = append(roundResult.Statements, DebateStatement{
				AgentIndex: i,
				Content:    agentResult.Output,
			})
			
			agent.Reset()
		}
		
		result.Rounds = append(result.Rounds, roundResult)
	}
	
	// Generate conclusion
	if d.moderator != nil {
		conclusion, _ := d.generateConclusion(ctx, result)
		result.Conclusion = conclusion
	}
	
	return result, nil
}

func (d *Debate) generateConclusion(ctx context.Context, result *DebateResult) (string, error) {
	prompt := fmt.Sprintf("Summarize this debate on '%s' and provide a balanced conclusion:\n\n", result.Topic)
	
	for _, round := range result.Rounds {
		prompt += fmt.Sprintf("Round %d:\n", round.RoundNum)
		for _, stmt := range round.Statements {
			prompt += fmt.Sprintf("- Participant %d: %s\n", stmt.AgentIndex+1, stmt.Content)
		}
		prompt += "\n"
	}
	
	return d.moderator.Generate(ctx, prompt)
}

// Ensemble runs multiple agents and combines their outputs.
type Ensemble struct {
	agents   []*Agent
	combiner func([]string) string
}

// NewEnsemble creates a new ensemble.
func NewEnsemble() *Ensemble {
	return &Ensemble{
		agents: make([]*Agent, 0),
		combiner: func(outputs []string) string {
			// Default: return longest output
			sort.Slice(outputs, func(i, j int) bool {
				return len(outputs[i]) > len(outputs[j])
			})
			if len(outputs) > 0 {
				return outputs[0]
			}
			return ""
		},
	}
}

// Add adds an agent to the ensemble.
func (e *Ensemble) Add(agent *Agent) *Ensemble {
	e.agents = append(e.agents, agent)
	return e
}

// WithCombiner sets a custom output combiner.
func (e *Ensemble) WithCombiner(fn func([]string) string) *Ensemble {
	e.combiner = fn
	return e
}

// Run executes all agents and combines outputs.
func (e *Ensemble) Run(ctx context.Context, task string) (string, error) {
	outputs := make([]string, 0, len(e.agents))
	var mu sync.Mutex
	var wg sync.WaitGroup
	
	for _, agent := range e.agents {
		wg.Add(1)
		go func(a *Agent) {
			defer wg.Done()
			result, err := a.Run(ctx, task)
			if err == nil {
				mu.Lock()
				outputs = append(outputs, result.Output)
				mu.Unlock()
			}
		}(agent)
	}
	
	wg.Wait()
	
	if len(outputs) == 0 {
		return "", fmt.Errorf("all agents failed")
	}
	
	return e.combiner(outputs), nil
}
