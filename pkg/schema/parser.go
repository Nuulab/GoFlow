// Package schema provides generic structured output parsing for LLM responses.
// It uses Go generics to ensure type-safe JSON unmarshaling of LLM outputs.
package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/nuulab/goflow/pkg/core"
)

// Parser forces an LLM to output JSON and unmarshals it into a specific struct T.
type Parser[T any] struct {
	llm         core.LLM
	systemPrompt string
}

// NewParser creates a new schema parser with the given LLM.
func NewParser[T any](llm core.LLM) *Parser[T] {
	var zero T
	schema := generateJSONSchema(zero)
	
	systemPrompt := fmt.Sprintf(`You must respond with valid JSON that matches this schema:
%s

Only output the JSON, no additional text or explanation.`, schema)
	
	return &Parser[T]{
		llm:         llm,
		systemPrompt: systemPrompt,
	}
}

// Parse sends a prompt to the LLM and parses the response into type T.
func (p *Parser[T]) Parse(ctx context.Context, prompt string, opts ...core.Option) (T, error) {
	var result T

	messages := []core.Message{
		{Role: core.RoleSystem, Content: p.systemPrompt},
		{Role: core.RoleUser, Content: prompt},
	}

	response, err := p.llm.GenerateChat(ctx, messages, opts...)
	if err != nil {
		return result, fmt.Errorf("schema: LLM generation failed: %w", err)
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return result, fmt.Errorf("schema: failed to parse JSON response: %w", err)
	}

	return result, nil
}

// WithValidator adds a validation function that runs after parsing.
type ValidatedParser[T any] struct {
	parser    *Parser[T]
	validator func(T) error
}

// NewValidatedParser creates a parser with custom validation.
func NewValidatedParser[T any](llm core.LLM, validator func(T) error) *ValidatedParser[T] {
	return &ValidatedParser[T]{
		parser:    NewParser[T](llm),
		validator: validator,
	}
}

// Parse sends a prompt and validates the parsed result.
func (vp *ValidatedParser[T]) Parse(ctx context.Context, prompt string, opts ...core.Option) (T, error) {
	result, err := vp.parser.Parse(ctx, prompt, opts...)
	if err != nil {
		return result, err
	}

	if err := vp.validator(result); err != nil {
		return result, fmt.Errorf("schema: validation failed: %w", err)
	}

	return result, nil
}

// generateJSONSchema creates a simple JSON schema representation for type T.
// This is a basic implementation - a production version would use a proper schema library.
func generateJSONSchema(v any) string {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return fmt.Sprintf(`{"type": "%s"}`, t.Kind().String())
	}

	fields := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			jsonTag = field.Name
		}
		
		fieldType := goTypeToJSONType(field.Type)
		fields = append(fields, fmt.Sprintf(`"%s": {"type": "%s"}`, jsonTag, fieldType))
	}

	return fmt.Sprintf(`{
  "type": "object",
  "properties": {
    %s
  }
}`, joinStrings(fields, ",\n    "))
}

// goTypeToJSONType converts Go types to JSON schema types.
func goTypeToJSONType(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map, reflect.Struct:
		return "object"
	default:
		return "string"
	}
}

// joinStrings joins strings with a separator.
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for _, s := range strs[1:] {
		result += sep + s
	}
	return result
}
