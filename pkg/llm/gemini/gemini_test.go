// Package gemini provides integration tests for the Gemini LLM provider.
// These tests require a valid GEMINI_API_KEY environment variable.
package gemini_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nuulab/goflow/pkg/core"
	"github.com/nuulab/goflow/pkg/llm/gemini"
)

func skipIfNoAPIKey(t *testing.T) string {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		key = os.Getenv("GOOGLE_API_KEY")
	}
	if key == "" {
		t.Skip("GEMINI_API_KEY or GOOGLE_API_KEY not set, skipping integration test")
	}
	return key
}

func TestGemini_Generate(t *testing.T) {
	apiKey := skipIfNoAPIKey(t)

	client := gemini.New(apiKey, gemini.WithModel("gemini-3-flash-preview"))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := client.Generate(ctx, "What is 2 + 2? Answer with just the number.")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if response == "" {
		t.Error("Expected non-empty response")
	}

	// The response should contain "4"
	if !strings.Contains(response, "4") {
		t.Errorf("Expected response to contain '4', got: %s", response)
	}

	t.Logf("Generate response: %s", response)
}

func TestGemini_GenerateChat(t *testing.T) {
	apiKey := skipIfNoAPIKey(t)

	client := gemini.New(apiKey, gemini.WithModel("gemini-3-flash-preview"))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	messages := []core.Message{
		{Role: core.RoleSystem, Content: "You are a helpful math tutor. Be concise."},
		{Role: core.RoleUser, Content: "What is the square root of 16?"},
	}

	response, err := client.GenerateChat(ctx, messages)
	if err != nil {
		t.Fatalf("GenerateChat failed: %v", err)
	}

	if response == "" {
		t.Error("Expected non-empty response")
	}

	// The response should mention "4"
	if !strings.Contains(response, "4") {
		t.Errorf("Expected response to contain '4', got: %s", response)
	}

	t.Logf("GenerateChat response: %s", response)
}

func TestGemini_Stream(t *testing.T) {
	apiKey := skipIfNoAPIKey(t)

	client := gemini.New(apiKey, gemini.WithModel("gemini-3-flash-preview"))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := client.Stream(ctx, "Count from 1 to 5, one number per line.")
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	var chunks []string
	for chunk := range stream {
		chunks = append(chunks, chunk)
	}

	if len(chunks) == 0 {
		t.Error("Expected at least one chunk from stream")
	}

	fullResponse := strings.Join(chunks, "")
	t.Logf("Stream response (%d chunks): %s", len(chunks), fullResponse)

	// Should contain numbers 1-5
	for _, num := range []string{"1", "2", "3", "4", "5"} {
		if !strings.Contains(fullResponse, num) {
			t.Errorf("Expected response to contain '%s'", num)
		}
	}
}

func TestGemini_WithOptions(t *testing.T) {
	apiKey := skipIfNoAPIKey(t)

	client := gemini.New(apiKey,
		gemini.WithModel("gemini-3-flash-preview"),
		gemini.WithTimeout(60*time.Second),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test with temperature and max tokens
	response, err := client.Generate(ctx, "Say hello",
		core.WithTemperature(0.1),
		core.WithMaxTokens(50),
	)
	if err != nil {
		t.Fatalf("Generate with options failed: %v", err)
	}

	if response == "" {
		t.Error("Expected non-empty response")
	}

	t.Logf("Response with options: %s", response)
}

func TestGemini_InvalidAPIKey(t *testing.T) {
	client := gemini.New("invalid-api-key", gemini.WithModel("gemini-3-flash-preview"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.Generate(ctx, "Hello")
	if err == nil {
		t.Error("Expected error with invalid API key, got nil")
	}

	t.Logf("Expected error received: %v", err)
}
