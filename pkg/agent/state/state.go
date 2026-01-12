// Package state provides typed shared state for agent networks.
package state

import (
	"encoding/json"
	"sync"
	"time"
)

// State provides typed, thread-safe shared state for agent networks.
// It includes key-value storage, conversation history, and metadata.
type State[T any] struct {
	mu       sync.RWMutex
	data     T
	kv       map[string]any
	history  []HistoryEntry
	metadata map[string]string
	created  time.Time
	updated  time.Time
}

// HistoryEntry records an agent's inference result.
type HistoryEntry struct {
	AgentName string         `json:"agent_name"`
	Input     string         `json:"input"`
	Output    string         `json:"output"`
	ToolCalls []ToolCallInfo `json:"tool_calls,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	Duration  time.Duration  `json:"duration"`
}

// ToolCallInfo records a tool invocation.
type ToolCallInfo struct {
	Name   string `json:"name"`
	Input  string `json:"input"`
	Output string `json:"output"`
}

// New creates a new typed state with optional initial data.
func New[T any](initial ...T) *State[T] {
	s := &State[T]{
		kv:       make(map[string]any),
		history:  make([]HistoryEntry, 0),
		metadata: make(map[string]string),
		created:  time.Now(),
		updated:  time.Now(),
	}
	if len(initial) > 0 {
		s.data = initial[0]
	}
	return s
}

// Data returns a pointer to the typed data for reading/writing.
func (s *State[T]) Data() *T {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updated = time.Now()
	return &s.data
}

// Get returns a copy of the typed data (read-only).
func (s *State[T]) Get() T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

// Set replaces the typed data.
func (s *State[T]) Set(data T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = data
	s.updated = time.Now()
}

// Update applies a function to modify the data atomically.
func (s *State[T]) Update(fn func(*T)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.data)
	s.updated = time.Now()
}

// ============ Key-Value Store ============

// SetKV sets a key-value pair.
func (s *State[T]) SetKV(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kv[key] = value
	s.updated = time.Now()
}

// GetKV retrieves a value by key.
func (s *State[T]) GetKV(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.kv[key]
	return v, ok
}

// GetKVTyped retrieves a typed value by key.
func GetKVTyped[V any](s *State[any], key string) (V, bool) {
	v, ok := s.GetKV(key)
	if !ok {
		var zero V
		return zero, false
	}
	typed, ok := v.(V)
	return typed, ok
}

// DeleteKV removes a key.
func (s *State[T]) DeleteKV(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.kv, key)
	s.updated = time.Now()
}

// KVKeys returns all keys.
func (s *State[T]) KVKeys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.kv))
	for k := range s.kv {
		keys = append(keys, k)
	}
	return keys
}

// ============ History ============

// AddHistory records an agent's result in history.
func (s *State[T]) AddHistory(entry HistoryEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry.Timestamp = time.Now()
	s.history = append(s.history, entry)
	s.updated = time.Now()
}

// History returns all history entries.
func (s *State[T]) History() []HistoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]HistoryEntry, len(s.history))
	copy(result, s.history)
	return result
}

// LastHistory returns the last N history entries.
func (s *State[T]) LastHistory(n int) []HistoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if n >= len(s.history) {
		result := make([]HistoryEntry, len(s.history))
		copy(result, s.history)
		return result
	}
	result := make([]HistoryEntry, n)
	copy(result, s.history[len(s.history)-n:])
	return result
}

// LastResult returns the most recent history entry.
func (s *State[T]) LastResult() *HistoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.history) == 0 {
		return nil
	}
	entry := s.history[len(s.history)-1]
	return &entry
}

// ClearHistory removes all history.
func (s *State[T]) ClearHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = make([]HistoryEntry, 0)
	s.updated = time.Now()
}

// ============ Metadata ============

// SetMeta sets metadata.
func (s *State[T]) SetMeta(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metadata[key] = value
	s.updated = time.Now()
}

// GetMeta retrieves metadata.
func (s *State[T]) GetMeta(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metadata[key]
}

// ============ Serialization ============

// StateSnapshot is a serializable version of state.
type StateSnapshot struct {
	Data     json.RawMessage   `json:"data"`
	KV       map[string]any    `json:"kv"`
	History  []HistoryEntry    `json:"history"`
	Metadata map[string]string `json:"metadata"`
	Created  time.Time         `json:"created"`
	Updated  time.Time         `json:"updated"`
}

// Snapshot creates a serializable snapshot of the state.
func (s *State[T]) Snapshot() (*StateSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.Marshal(s.data)
	if err != nil {
		return nil, err
	}

	return &StateSnapshot{
		Data:     data,
		KV:       s.kv,
		History:  s.history,
		Metadata: s.metadata,
		Created:  s.created,
		Updated:  s.updated,
	}, nil
}

// LoadSnapshot restores state from a snapshot.
func (s *State[T]) LoadSnapshot(snap *StateSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := json.Unmarshal(snap.Data, &s.data); err != nil {
		return err
	}

	s.kv = snap.KV
	s.history = snap.History
	s.metadata = snap.Metadata
	s.created = snap.Created
	s.updated = snap.Updated
	return nil
}

// ============ Convenience Functions ============

// CallCount returns the number of agent calls in history.
func (s *State[T]) CallCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.history)
}

// AgentCalls returns history entries for a specific agent.
func (s *State[T]) AgentCalls(agentName string) []HistoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []HistoryEntry
	for _, h := range s.history {
		if h.AgentName == agentName {
			result = append(result, h)
		}
	}
	return result
}
