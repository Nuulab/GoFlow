// Package api provides HTTP and WebSocket server for GoFlow.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/nuulab/goflow/pkg/agent"
	"github.com/nuulab/goflow/pkg/core"
	"github.com/nuulab/goflow/pkg/tools"
)

// Server is the GoFlow API server.
type Server struct {
	llm        core.LLM
	registry   *tools.Registry
	agents     map[string]*ManagedAgent
	settings   *Settings
	hub        *WebSocketHub
	mu         sync.RWMutex
	httpServer *http.Server
}

// ManagedAgent wraps an agent with metadata.
type ManagedAgent struct {
	ID        string       `json:"id"`
	Agent     *agent.Agent `json:"-"`
	Status    AgentStatus  `json:"status"`
	CreatedAt time.Time    `json:"created_at"`
	LastRunAt time.Time    `json:"last_run_at,omitempty"`
}

// AgentStatus represents the current state of an agent.
type AgentStatus string

const (
	AgentIdle    AgentStatus = "idle"
	AgentRunning AgentStatus = "running"
	AgentStopped AgentStatus = "stopped"
)

// Settings holds configurable server settings.
type Settings struct {
	MaxIterations   int           `json:"max_iterations"`
	DefaultTimeout  time.Duration `json:"default_timeout"`
	VerboseLogging  bool          `json:"verbose_logging"`
	AllowedOrigins  []string      `json:"allowed_origins"`
	mu              sync.RWMutex
}

// DefaultSettings returns default server settings.
func DefaultSettings() *Settings {
	return &Settings{
		MaxIterations:  10,
		DefaultTimeout: 5 * time.Minute,
		VerboseLogging: false,
		AllowedOrigins: []string{"*"},
	}
}

// Config holds server configuration.
type Config struct {
	Port     int
	LLM      core.LLM
	Registry *tools.Registry
	Settings *Settings
}

// NewServer creates a new API server.
func NewServer(cfg Config) *Server {
	if cfg.Settings == nil {
		cfg.Settings = DefaultSettings()
	}
	if cfg.Registry == nil {
		cfg.Registry = tools.NewRegistry()
	}

	s := &Server{
		llm:      cfg.LLM,
		registry: cfg.Registry,
		agents:   make(map[string]*ManagedAgent),
		settings: cfg.Settings,
		hub:      NewWebSocketHub(),
	}

	return s
}

// Start starts the HTTP server.
func (s *Server) Start(port int) error {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/agents", s.corsMiddleware(s.handleAgents))
	mux.HandleFunc("/api/agents/", s.corsMiddleware(s.handleAgent))
	mux.HandleFunc("/api/settings", s.corsMiddleware(s.handleSettings))
	mux.HandleFunc("/api/channels", s.corsMiddleware(s.handleChannels))

	// WebSocket
	mux.HandleFunc("/ws", s.handleWebSocket)

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Start WebSocket hub
	go s.hub.Run()

	fmt.Printf("🚀 GoFlow API server starting on http://localhost:%d\n", port)
	return s.httpServer.ListenAndServe()
}

// Stop gracefully stops the server.
func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// corsMiddleware adds CORS headers.
func (s *Server) corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.settings.mu.RLock()
		origins := s.settings.AllowedOrigins
		s.settings.mu.RUnlock()

		origin := r.Header.Get("Origin")
		allowed := false
		for _, o := range origins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// GetAgent retrieves a managed agent by ID.
func (s *Server) GetAgent(id string) (*ManagedAgent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.agents[id]
	return a, ok
}

// CreateAgent creates a new managed agent.
func (s *Server) CreateAgent(id string) *ManagedAgent {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.settings.mu.RLock()
	maxIter := s.settings.MaxIterations
	verbose := s.settings.VerboseLogging
	s.settings.mu.RUnlock()

	// Create agent with hooks for WebSocket events
	hooks := s.createAgentHooks(id)

	a := agent.New(s.llm, s.registry,
		agent.WithMaxIterations(maxIter),
		agent.WithVerbose(verbose),
		agent.WithHooks(hooks),
	)

	managed := &ManagedAgent{
		ID:        id,
		Agent:     a,
		Status:    AgentIdle,
		CreatedAt: time.Now(),
	}

	s.agents[id] = managed
	return managed
}

// createAgentHooks creates hooks that broadcast events via WebSocket.
func (s *Server) createAgentHooks(agentID string) agent.Hooks {
	return agent.NewHooks().
		OnStart(func(ctx context.Context, task string) {
			s.hub.Broadcast(Event{
				Type:    "agent.started",
				AgentID: agentID,
				Data:    map[string]string{"task": task},
			})
		}).
		OnAfterStep(func(ctx context.Context, step agent.StepResult) {
			s.hub.Broadcast(Event{
				Type:    "agent.step",
				AgentID: agentID,
				Data: map[string]any{
					"action":      step.Action.Action,
					"observation": step.Observation,
					"is_final":    step.IsFinal,
				},
			})
		}).
		OnToolCall(func(ctx context.Context, toolName string, input string) {
			s.hub.Broadcast(Event{
				Type:    "agent.tool_call",
				AgentID: agentID,
				Data: map[string]string{
					"tool":  toolName,
					"input": input,
				},
			})
		}).
		OnComplete(func(ctx context.Context, result *agent.RunResult) {
			s.hub.Broadcast(Event{
				Type:    "agent.completed",
				AgentID: agentID,
				Data: map[string]any{
					"output":     result.Output,
					"iterations": result.Iterations,
				},
			})
		}).
		Build()
}
