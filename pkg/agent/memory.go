// Package agent provides memory implementations for agents.
package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/nuulab/goflow/pkg/core"
)

// Memory defines the interface for agent memory systems.
type Memory interface {
	// Add stores a message in memory.
	Add(message core.Message)
	// Get retrieves all messages from memory.
	Get() []core.Message
	// GetContext returns messages formatted for LLM context.
	GetContext() string
	// Clear removes all messages from memory.
	Clear()
}

// BufferMemory implements a simple sliding window memory.
// It keeps the last N messages in a circular buffer.
type BufferMemory struct {
	mu       sync.RWMutex
	messages []core.Message
	maxSize  int
}

// NewBufferMemory creates a new buffer memory with the given max size.
func NewBufferMemory(maxSize int) *BufferMemory {
	return &BufferMemory{
		messages: make([]core.Message, 0, maxSize),
		maxSize:  maxSize,
	}
}

// Add stores a message, evicting oldest if at capacity.
func (b *BufferMemory) Add(message core.Message) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.messages) >= b.maxSize {
		// Remove oldest message
		b.messages = b.messages[1:]
	}
	b.messages = append(b.messages, message)
}

// Get returns all stored messages.
func (b *BufferMemory) Get() []core.Message {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return append([]core.Message{}, b.messages...)
}

// GetContext formats messages for LLM context.
func (b *BufferMemory) GetContext() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var sb strings.Builder
	for _, msg := range b.messages {
		sb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}
	return sb.String()
}

// Clear removes all messages.
func (b *BufferMemory) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messages = make([]core.Message, 0, b.maxSize)
}

// SummaryMemory uses an LLM to summarize conversation history.
// When the buffer fills, it summarizes and compresses the history.
type SummaryMemory struct {
	mu           sync.RWMutex
	llm          core.LLM
	messages     []core.Message
	summary      string
	bufferSize   int
	summarySize  int // Target size for summaries
}

// NewSummaryMemory creates a new summary-based memory.
func NewSummaryMemory(llm core.LLM, bufferSize int) *SummaryMemory {
	return &SummaryMemory{
		llm:         llm,
		messages:    make([]core.Message, 0, bufferSize),
		bufferSize:  bufferSize,
		summarySize: 500, // Default summary target length
	}
}

// Add stores a message and triggers summarization if needed.
func (s *SummaryMemory) Add(message core.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = append(s.messages, message)

	// Check if we need to summarize
	if len(s.messages) >= s.bufferSize {
		// Summarize in background (don't block Add)
		go s.summarize()
	}
}

// summarize compresses the message history using the LLM.
func (s *SummaryMemory) summarize() {
	s.mu.Lock()
	if len(s.messages) < s.bufferSize/2 {
		s.mu.Unlock()
		return
	}

	// Keep last few messages, summarize the rest
	keepCount := s.bufferSize / 4
	toSummarize := s.messages[:len(s.messages)-keepCount]
	toKeep := s.messages[len(s.messages)-keepCount:]

	// Build summarization prompt
	var sb strings.Builder
	sb.WriteString("Summarize the following conversation, keeping key information:\n\n")
	if s.summary != "" {
		sb.WriteString(fmt.Sprintf("Previous summary: %s\n\n", s.summary))
	}
	for _, msg := range toSummarize {
		sb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}
	s.mu.Unlock()

	// Generate summary (outside lock)
	ctx := context.Background()
	newSummary, err := s.llm.Generate(ctx, sb.String())
	if err != nil {
		// If summarization fails, just drop old messages
		s.mu.Lock()
		s.messages = toKeep
		s.mu.Unlock()
		return
	}

	s.mu.Lock()
	s.summary = newSummary
	s.messages = toKeep
	s.mu.Unlock()
}

// Get returns all stored messages.
func (s *SummaryMemory) Get() []core.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]core.Message{}, s.messages...)
}

// GetContext returns the summary plus recent messages.
func (s *SummaryMemory) GetContext() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sb strings.Builder
	if s.summary != "" {
		sb.WriteString(fmt.Sprintf("Conversation summary: %s\n\n", s.summary))
	}
	sb.WriteString("Recent messages:\n")
	for _, msg := range s.messages {
		sb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}
	return sb.String()
}

// Clear removes all messages and summary.
func (s *SummaryMemory) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = make([]core.Message, 0, s.bufferSize)
	s.summary = ""
}

// GetSummary returns the current conversation summary.
func (s *SummaryMemory) GetSummary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.summary
}

// WindowMemory keeps a fixed window of the most important messages.
// It prioritizes system messages and recent interactions.
type WindowMemory struct {
	mu         sync.RWMutex
	messages   []core.Message
	windowSize int
}

// NewWindowMemory creates a new window memory.
func NewWindowMemory(windowSize int) *WindowMemory {
	return &WindowMemory{
		messages:   make([]core.Message, 0, windowSize),
		windowSize: windowSize,
	}
}

// Add stores a message with priority-based eviction.
func (w *WindowMemory) Add(message core.Message) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.messages = append(w.messages, message)

	// If over capacity, evict non-system messages from the middle
	for len(w.messages) > w.windowSize {
		evicted := false
		// Try to evict from middle (keep first and last messages)
		for i := 1; i < len(w.messages)-1; i++ {
			if w.messages[i].Role != core.RoleSystem {
				w.messages = append(w.messages[:i], w.messages[i+1:]...)
				evicted = true
				break
			}
		}
		// If no evictable message found, remove second message
		if !evicted && len(w.messages) > 1 {
			w.messages = append(w.messages[:1], w.messages[2:]...)
		}
	}
}

// Get returns all stored messages.
func (w *WindowMemory) Get() []core.Message {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return append([]core.Message{}, w.messages...)
}

// GetContext formats messages for LLM context.
func (w *WindowMemory) GetContext() string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var sb strings.Builder
	for _, msg := range w.messages {
		sb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}
	return sb.String()
}

// Clear removes all messages.
func (w *WindowMemory) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.messages = make([]core.Message, 0, w.windowSize)
}
