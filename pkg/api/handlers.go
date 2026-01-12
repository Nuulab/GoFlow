// Package api provides REST handlers for GoFlow.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// handleAgents handles /api/agents
func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.listAgents(w, r)
	case "POST":
		s.createAgentHandler(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// listAgents returns all agents.
func (s *Server) listAgents(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]*AgentInfo, 0, len(s.agents))
	for _, a := range s.agents {
		agents = append(agents, &AgentInfo{
			ID:        a.ID,
			Status:    a.Status,
			CreatedAt: a.CreatedAt,
			LastRunAt: a.LastRunAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"agents": agents})
}

// AgentInfo is a serializable agent representation.
type AgentInfo struct {
	ID        string      `json:"id"`
	Status    AgentStatus `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
	LastRunAt time.Time   `json:"last_run_at,omitempty"`
}

// CreateAgentRequest is the request body for creating an agent.
type CreateAgentRequest struct {
	ID            string `json:"id"`
	MaxIterations int    `json:"max_iterations,omitempty"`
}

// createAgentHandler creates a new agent.
func (s *Server) createAgentHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" {
		req.ID = generateID()
	}

	if _, exists := s.GetAgent(req.ID); exists {
		writeError(w, http.StatusConflict, "agent already exists")
		return
	}

	managed := s.CreateAgent(req.ID)

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         managed.ID,
		"status":     managed.Status,
		"created_at": managed.CreatedAt,
	})
}

// handleAgent handles /api/agents/:id/*
func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
	// Parse path: /api/agents/:id or /api/agents/:id/action
	path := strings.TrimPrefix(r.URL.Path, "/api/agents/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "agent ID required")
		return
	}

	agentID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "":
		s.handleAgentInfo(w, r, agentID)
	case "run":
		s.handleAgentRun(w, r, agentID)
	case "stop":
		s.handleAgentStop(w, r, agentID)
	case "reset":
		s.handleAgentReset(w, r, agentID)
	default:
		writeError(w, http.StatusNotFound, "unknown action")
	}
}

// handleAgentInfo returns agent info.
func (s *Server) handleAgentInfo(w http.ResponseWriter, r *http.Request, agentID string) {
	if r.Method != "GET" && r.Method != "DELETE" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if r.Method == "DELETE" {
		s.mu.Lock()
		delete(s.agents, agentID)
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]string{"deleted": agentID})
		return
	}

	managed, ok := s.GetAgent(agentID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	writeJSON(w, http.StatusOK, &AgentInfo{
		ID:        managed.ID,
		Status:    managed.Status,
		CreatedAt: managed.CreatedAt,
		LastRunAt: managed.LastRunAt,
	})
}

// RunRequest is the request body for running an agent.
type RunRequest struct {
	Task    string `json:"task"`
	Timeout int    `json:"timeout,omitempty"` // seconds
}

// RunResponse is the response from running an agent.
type RunResponse struct {
	Output     string `json:"output"`
	Iterations int    `json:"iterations"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
}

// handleAgentRun runs a task on an agent.
func (s *Server) handleAgentRun(w http.ResponseWriter, r *http.Request, agentID string) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	managed, ok := s.GetAgent(agentID)
	if !ok {
		// Auto-create agent if it doesn't exist
		managed = s.CreateAgent(agentID)
	}

	if managed.Status == AgentRunning {
		writeError(w, http.StatusConflict, "agent is already running")
		return
	}

	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Task == "" {
		writeError(w, http.StatusBadRequest, "task is required")
		return
	}

	// Set timeout
	timeout := time.Duration(req.Timeout) * time.Second
	if timeout == 0 {
		s.settings.mu.RLock()
		timeout = s.settings.DefaultTimeout
		s.settings.mu.RUnlock()
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Update status
	s.mu.Lock()
	managed.Status = AgentRunning
	managed.LastRunAt = time.Now()
	s.mu.Unlock()

	// Run agent
	result, err := managed.Agent.Run(ctx, req.Task)

	// Update status
	s.mu.Lock()
	managed.Status = AgentIdle
	s.mu.Unlock()

	response := RunResponse{
		Iterations: result.Iterations,
		Success:    err == nil,
	}

	if err != nil {
		response.Error = err.Error()
	} else {
		response.Output = result.Output
	}

	writeJSON(w, http.StatusOK, response)
}

// handleAgentStop stops a running agent.
func (s *Server) handleAgentStop(w http.ResponseWriter, r *http.Request, agentID string) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	managed, ok := s.GetAgent(agentID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	s.mu.Lock()
	managed.Status = AgentStopped
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// handleAgentReset resets an agent's state.
func (s *Server) handleAgentReset(w http.ResponseWriter, r *http.Request, agentID string) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	managed, ok := s.GetAgent(agentID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	managed.Agent.Reset()

	s.mu.Lock()
	managed.Status = AgentIdle
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

// handleSettings handles /api/settings
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.settings.mu.RLock()
		defer s.settings.mu.RUnlock()
		writeJSON(w, http.StatusOK, s.settings)

	case "PUT":
		var update Settings
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		s.settings.mu.Lock()
		if update.MaxIterations > 0 {
			s.settings.MaxIterations = update.MaxIterations
		}
		if update.DefaultTimeout > 0 {
			s.settings.DefaultTimeout = update.DefaultTimeout
		}
		s.settings.VerboseLogging = update.VerboseLogging
		if len(update.AllowedOrigins) > 0 {
			s.settings.AllowedOrigins = update.AllowedOrigins
		}
		s.settings.mu.Unlock()

		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleChannels handles /api/channels
func (s *Server) handleChannels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		// Publish a message to a channel
		var req struct {
			Channel string `json:"channel"`
			Topic   string `json:"topic"`
			Data    any    `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		s.hub.Broadcast(Event{
			Type: "channel.message",
			Data: map[string]any{
				"channel": req.Channel,
				"topic":   req.Topic,
				"data":    req.Data,
			},
		})

		writeJSON(w, http.StatusOK, map[string]string{"status": "published"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// generateID creates a unique ID.
func generateID() string {
	return time.Now().Format("20060102-150405.000")
}
