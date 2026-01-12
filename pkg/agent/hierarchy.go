// Package agent provides hierarchical agent capabilities.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/nuulab/goflow/pkg/core"
	"github.com/nuulab/goflow/pkg/tools"
)

// HierarchicalSupervisor manages a tree of agents that can spawn sub-agents.
type HierarchicalSupervisor struct {
	llm          core.LLM
	rootAgent    *Agent
	spawnLimit   int32
	activeAgents int32
	maxDepth     int
	registry     *tools.Registry
	mu           sync.RWMutex
	children     map[string][]*HierarchicalAgent
}

// HierarchicalAgent wraps an agent with hierarchy awareness.
type HierarchicalAgent struct {
	*Agent
	id       string
	parentID string
	depth    int
	super    *HierarchicalSupervisor
}

// NewHierarchicalSupervisor creates a hierarchical supervisor.
func NewHierarchicalSupervisor(llm core.LLM, registry *tools.Registry) *HierarchicalSupervisor {
	return &HierarchicalSupervisor{
		llm:        llm,
		spawnLimit: 10,
		maxDepth:   3,
		registry:   registry,
		children:   make(map[string][]*HierarchicalAgent),
	}
}

// WithSpawnLimit sets the maximum number of concurrent sub-agents.
func (hs *HierarchicalSupervisor) WithSpawnLimit(limit int) *HierarchicalSupervisor {
	hs.spawnLimit = int32(limit)
	return hs
}

// WithDepth sets the maximum delegation depth.
func (hs *HierarchicalSupervisor) WithDepth(depth int) *HierarchicalSupervisor {
	hs.maxDepth = depth
	return hs
}

// Run executes a task with the ability to spawn sub-agents.
func (hs *HierarchicalSupervisor) Run(ctx context.Context, task string) (*HierarchicalResult, error) {
	rootAgent := hs.createAgent("root", "", 0)
	
	// Add spawn tool to the agent
	spawnTool := hs.createSpawnTool(rootAgent)
	hs.registry.Register(spawnTool)
	
	result, err := rootAgent.Run(ctx, task)
	
	return &HierarchicalResult{
		RunResult:   result,
		TotalAgents: int(atomic.LoadInt32(&hs.activeAgents)),
		AgentTree:   hs.buildTree("root"),
	}, err
}

// createAgent creates a new hierarchical agent.
func (hs *HierarchicalSupervisor) createAgent(id, parentID string, depth int) *HierarchicalAgent {
	agent := New(hs.llm, hs.registry)
	
	ha := &HierarchicalAgent{
		Agent:    agent,
		id:       id,
		parentID: parentID,
		depth:    depth,
		super:    hs,
	}
	
	hs.mu.Lock()
	hs.children[parentID] = append(hs.children[parentID], ha)
	hs.mu.Unlock()
	
	atomic.AddInt32(&hs.activeAgents, 1)
	return ha
}

// SpawnSubAgent creates a child agent for a specific subtask.
func (ha *HierarchicalAgent) SpawnSubAgent(ctx context.Context, subtask string) (*RunResult, error) {
	if ha.depth >= ha.super.maxDepth {
		return nil, fmt.Errorf("max depth reached (%d)", ha.super.maxDepth)
	}
	
	current := atomic.LoadInt32(&ha.super.activeAgents)
	if current >= ha.super.spawnLimit {
		return nil, fmt.Errorf("spawn limit reached (%d)", ha.super.spawnLimit)
	}
	
	childID := fmt.Sprintf("%s.%d", ha.id, len(ha.super.children[ha.id]))
	child := ha.super.createAgent(childID, ha.id, ha.depth+1)
	
	return child.Run(ctx, subtask)
}

// createSpawnTool creates a tool that allows agents to spawn sub-agents.
func (hs *HierarchicalSupervisor) createSpawnTool(agent *HierarchicalAgent) *tools.Tool {
	return tools.Build("spawn_agent").
		Description("Spawn a sub-agent to handle a subtask. Use for complex tasks that can be broken down.").
		Param("subtask", "string", "The subtask for the sub-agent to handle").
		Param("reason", "string", "Why this subtask needs a separate agent").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				Subtask string `json:"subtask"`
				Reason  string `json:"reason"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				return "", err
			}
			
			result, err := agent.SpawnSubAgent(ctx, params.Subtask)
			if err != nil {
				return "", err
			}
			
			return fmt.Sprintf("Sub-agent completed: %s", result.Output), nil
		}).
		Create()
}

// buildTree constructs the agent hierarchy tree.
func (hs *HierarchicalSupervisor) buildTree(rootID string) *AgentNode {
	hs.mu.RLock()
	defer hs.mu.RUnlock()
	
	return hs.buildNode(rootID)
}

func (hs *HierarchicalSupervisor) buildNode(id string) *AgentNode {
	node := &AgentNode{
		ID:       id,
		Children: make([]*AgentNode, 0),
	}
	
	for _, child := range hs.children[id] {
		node.Children = append(node.Children, hs.buildNode(child.id))
	}
	
	return node
}

// HierarchicalResult holds the result of a hierarchical run.
type HierarchicalResult struct {
	*RunResult
	TotalAgents int
	AgentTree   *AgentNode
}

// AgentNode represents a node in the agent tree.
type AgentNode struct {
	ID       string
	Children []*AgentNode
}
