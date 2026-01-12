// Package core defines the fundamental interfaces for the GoFlow AI orchestration framework.
// These interfaces allow swapping providers (OpenAI, Anthropic, Ollama, etc.) seamlessly.
package core

import "context"

// Option represents a configuration option for LLM calls.
// Use functional options pattern for extensibility.
type Option func(*CallOptions)

// CallOptions holds configuration for an LLM generation call.
type CallOptions struct {
	Temperature      float64
	MaxTokens        int
	TopP             float64
	StopSequences    []string
	PresencePenalty  float64
	FrequencyPenalty float64
}

// WithTemperature sets the temperature for generation.
func WithTemperature(t float64) Option {
	return func(o *CallOptions) {
		o.Temperature = t
	}
}

// WithMaxTokens sets the maximum tokens for generation.
func WithMaxTokens(max int) Option {
	return func(o *CallOptions) {
		o.MaxTokens = max
	}
}

// WithTopP sets the top-p (nucleus sampling) parameter.
func WithTopP(p float64) Option {
	return func(o *CallOptions) {
		o.TopP = p
	}
}

// WithStopSequences sets stop sequences for generation.
func WithStopSequences(seqs ...string) Option {
	return func(o *CallOptions) {
		o.StopSequences = seqs
	}
}

// Message represents a chat message with a role and content.
type Message struct {
	Role    Role
	Content string
}

// Role represents the role of a message sender.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// LLM defines the interface for Large Language Model providers.
// All implementations must support context for cancellation and timeouts.
type LLM interface {
	// Generate produces a completion for the given prompt.
	// Returns the generated text or an error if the generation fails.
	Generate(ctx context.Context, prompt string, opts ...Option) (string, error)

	// GenerateChat produces a completion for a conversation of messages.
	// Returns the generated response or an error if the generation fails.
	GenerateChat(ctx context.Context, messages []Message, opts ...Option) (string, error)

	// Stream produces a streaming completion for the given prompt.
	// Returns a channel that yields text chunks as they are generated.
	// The channel is closed when generation completes or an error occurs.
	Stream(ctx context.Context, prompt string, opts ...Option) (<-chan string, error)

	// StreamChat produces a streaming completion for a conversation.
	// Returns a channel that yields text chunks as they are generated.
	StreamChat(ctx context.Context, messages []Message, opts ...Option) (<-chan string, error)
}

// Embedder defines the interface for text embedding providers.
// Embeddings convert text into dense vector representations.
type Embedder interface {
	// Embed generates embeddings for the given texts.
	// Returns a slice of embedding vectors (one per input text).
	// Each embedding is a slice of float32 values.
	Embed(ctx context.Context, texts []string) ([][]float32, error)

	// EmbedQuery generates an embedding optimized for query/search use cases.
	// Some providers use different models or processing for queries vs documents.
	EmbedQuery(ctx context.Context, query string) ([]float32, error)
}

// StreamEvent represents an event from a streaming LLM response.
type StreamEvent struct {
	// Content is the text chunk for this event.
	Content string
	// Err is set if an error occurred during streaming.
	Err error
	// Done indicates this is the final event in the stream.
	Done bool
}

// TokenCounter provides token counting functionality for LLM inputs.
type TokenCounter interface {
	// CountTokens returns the number of tokens in the given text.
	CountTokens(ctx context.Context, text string) (int, error)
}
