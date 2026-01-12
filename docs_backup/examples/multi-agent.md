# Multi-Agent Example

Build a research team with multiple collaborating agents.

## Code

```go
package main

import (
	"context"
	"fmt"

	"github.com/goflow/goflow/pkg/agent"
	"github.com/goflow/goflow/pkg/llm/openai"
	"github.com/goflow/goflow/pkg/tools"
)

func main() {
	llm := openai.New(os.Getenv("OPENAI_API_KEY"))

	// 1. Create specialized agents
	researcher := agent.New(llm, tools.WebToolkit(),
		agent.WithSystemPrompt(`You are a research specialist. 
Your job is to search the web and gather information on topics.
Focus on finding accurate, up-to-date information.`),
	)

	analyst := agent.New(llm, tools.DataToolkit(),
		agent.WithSystemPrompt(`You are a data analyst.
Your job is to analyze information and identify patterns.
Provide insights and recommendations based on data.`),
	)

	writer := agent.New(llm, tools.NewRegistry(),
		agent.WithSystemPrompt(`You are a technical writer.
Your job is to synthesize research and analysis into clear reports.
Write in a professional, easy-to-understand style.`),
	)

	// 2. Create a supervisor
	supervisor := agent.NewSupervisor(llm,
		agent.WithWorkers(
			agent.Worker{Name: "researcher", Agent: researcher},
			agent.Worker{Name: "analyst", Agent: analyst},
			agent.Worker{Name: "writer", Agent: writer},
		),
	)

	// 3. Run a complex task
	result, _ := supervisor.Run(context.Background(),
		"Research the current state of electric vehicles, "+
		"analyze market trends, and write a summary report.")

	fmt.Println(result.Output)
}
```

## Pipeline Pattern

Sequential processing through agents:

```go
// Each agent's output becomes the next agent's input
pipeline := agent.NewPipeline(
	researcher,  // Step 1: Gather data
	analyst,     // Step 2: Analyze
	writer,      // Step 3: Write report
)

result, _ := pipeline.Run(ctx, "Topic: AI in Healthcare")
```

## Consensus Pattern

Multiple agents voting on an answer:

```go
experts := []*agent.Agent{expert1, expert2, expert3}

consensus := agent.NewConsensus(experts, agent.MajorityVote)

result, _ := consensus.Decide(ctx, 
	"What's the best programming language for beginners?")

fmt.Println("Consensus answer:", result)
```

## Debate Pattern

Agents debating to find the best answer:

```go
debate := agent.NewDebate(
	agent.WithParticipants(optimist, pessimist),
	agent.WithModerator(llm),
	agent.WithRounds(3),
)

result, _ := debate.Run(ctx, 
	"Should we invest in cryptocurrency?")

fmt.Println("Debate conclusion:", result.Conclusion)
fmt.Println("Key arguments:", result.Arguments)
```

## Inter-Agent Communication

Agents communicating via channels:

```go
hub := agent.NewChannelHub()
hub.Create("research-results", 100)
hub.Create("commands", 10)

// Agent 1 publishes research results
go func() {
	for result := range researchResults {
		hub.Publish("research-results", agent.Message{
			From: "researcher",
			Data: result,
		})
	}
}()

// Agent 2 subscribes and processes
go func() {
	sub := hub.Subscribe("research-results")
	for msg := range sub {
		analyst.Run(ctx, "Analyze: " + msg.Data.(string))
	}
}()
```

## Hierarchical Agents

Agents that can spawn sub-agents:

```go
supervisor := agent.NewHierarchicalSupervisor(llm, registry)
supervisor.SetMaxDepth(3)     // Max 3 levels deep
supervisor.SetSpawnLimit(10)  // Max 10 sub-agents

root := supervisor.CreateRoot("Complex multi-part task")
result, _ := root.Run(ctx, task)

// View the agent tree
tree := supervisor.GetTree(root.ID)
for _, node := range tree {
	fmt.Printf("%s%s: %s\n", 
		strings.Repeat("  ", node.Depth),
		node.Name,
		node.Status,
	)
}
```
