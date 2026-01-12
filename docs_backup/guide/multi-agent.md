# Multi-Agent Systems

Build complex systems with multiple collaborating agents.

## Supervisor Pattern

A supervisor agent delegates tasks to specialized workers:

```go
import "github.com/goflow/goflow/pkg/agent"

supervisor := agent.NewSupervisor(llm,
    agent.WithWorkers(
        agent.Worker{Name: "researcher", Agent: researchAgent},
        agent.Worker{Name: "writer", Agent: writeAgent},
        agent.Worker{Name: "reviewer", Agent: reviewAgent},
    ),
)

result, _ := supervisor.Run(ctx, "Write a blog post about AI")
// Supervisor delegates: research → write → review
```

## Routing

Route tasks to the right agent:

```go
// LLM-based routing
router := agent.NewLLMRouter(llm, []agent.Agent{
    codeAgent,
    mathAgent,
    researchAgent,
})

// Keyword-based routing
router := agent.NewKeywordRouter(map[string]agent.Agent{
    "code":      codeAgent,
    "calculate": mathAgent,
    "search":    researchAgent,
})

result, _ := router.Route(ctx, task)
```

## Teams

Agents working together on a problem:

```go
team := agent.NewTeam(llm,
    agent.WithMembers(analyst, developer, tester),
    agent.WithStrategy(agent.RoundRobin),
)

result, _ := team.Collaborate(ctx, "Build a REST API")
```

## Pipeline

Sequential processing through multiple agents:

```go
pipeline := agent.NewPipeline(
    extractAgent,   // Step 1: Extract data
    transformAgent, // Step 2: Transform
    loadAgent,      // Step 3: Load
)

result, _ := pipeline.Run(ctx, input)
```

## Hierarchical Agents

Agents that can spawn sub-agents:

```go
supervisor := agent.NewHierarchicalSupervisor(llm, registry)

// Root agent can spawn children up to maxDepth
root := supervisor.CreateRoot("main-task")
result, _ := root.Run(ctx, "Complex multi-step task")

// View the agent tree
tree := supervisor.GetTree(root.ID)
```

## Inter-Agent Communication

Agents communicating via channels:

```go
hub := agent.NewChannelHub()

// Create named channels
hub.Create("results", 100)
hub.Create("commands", 10)

// Agent 1 publishes
hub.Publish("results", agent.Message{Data: result})

// Agent 2 subscribes
sub := hub.Subscribe("results")
for msg := range sub {
    process(msg)
}
```

## Consensus & Voting

Multiple agents reaching agreement:

```go
consensus := agent.NewConsensus(
    []agent.Agent{expert1, expert2, expert3},
    agent.MajorityVote,
)

result, _ := consensus.Decide(ctx, "What's the best approach?")
```

Voting strategies:
- `MajorityVote` - Simple majority
- `UnanimousVote` - All must agree
- `WeightedVote` - Weighted by confidence
- `LLMJudge` - LLM picks the best answer

## Debate Format

Agents debating to reach the best answer:

```go
debate := agent.NewDebate(
    agent.WithParticipants(agent1, agent2),
    agent.WithModerator(moderatorLLM),
    agent.WithRounds(3),
)

result, _ := debate.Run(ctx, "Should we use microservices?")
```
