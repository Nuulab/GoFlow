// Package tools provides a fluent builder API for creating tools.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// ToolBuilder provides a fluent API for constructing tools.
type ToolBuilder struct {
	name        string
	description string
	params      []paramDef
	handler     any
	examples    []string
	category    string
	tags        []string
}

type paramDef struct {
	name        string
	paramType   string
	description string
	required    bool
	enumValues  []string
	defaultVal  any
}

// Build starts building a new tool with the given name.
func Build(name string) *ToolBuilder {
	return &ToolBuilder{
		name:   name,
		params: make([]paramDef, 0),
		tags:   make([]string, 0),
	}
}

// Description sets the tool description.
func (b *ToolBuilder) Description(desc string) *ToolBuilder {
	b.description = desc
	return b
}

// Category sets the tool category for organization.
func (b *ToolBuilder) Category(cat string) *ToolBuilder {
	b.category = cat
	return b
}

// Tags adds tags for tool discovery.
func (b *ToolBuilder) Tags(tags ...string) *ToolBuilder {
	b.tags = append(b.tags, tags...)
	return b
}

// Param adds a required parameter.
func (b *ToolBuilder) Param(name, paramType, description string) *ToolBuilder {
	b.params = append(b.params, paramDef{
		name:        name,
		paramType:   paramType,
		description: description,
		required:    true,
	})
	return b
}

// OptionalParam adds an optional parameter.
func (b *ToolBuilder) OptionalParam(name, paramType, description string) *ToolBuilder {
	b.params = append(b.params, paramDef{
		name:        name,
		paramType:   paramType,
		description: description,
		required:    false,
	})
	return b
}

// ParamWithDefault adds a parameter with a default value.
func (b *ToolBuilder) ParamWithDefault(name, paramType, description string, defaultVal any) *ToolBuilder {
	b.params = append(b.params, paramDef{
		name:        name,
		paramType:   paramType,
		description: description,
		required:    false,
		defaultVal:  defaultVal,
	})
	return b
}

// EnumParam adds a parameter with enum constraints.
func (b *ToolBuilder) EnumParam(name, description string, values ...string) *ToolBuilder {
	b.params = append(b.params, paramDef{
		name:        name,
		paramType:   "string",
		description: description,
		required:    true,
		enumValues:  values,
	})
	return b
}

// Example adds a usage example.
func (b *ToolBuilder) Example(example string) *ToolBuilder {
	b.examples = append(b.examples, example)
	return b
}

// Handler sets a simple string->string handler function.
func (b *ToolBuilder) Handler(fn func(ctx context.Context, input string) (string, error)) *ToolBuilder {
	b.handler = fn
	return b
}

// HandlerFunc sets a typed handler with automatic JSON parsing.
func (b *ToolBuilder) HandlerFunc(fn any) *ToolBuilder {
	b.handler = fn
	return b
}

// Create builds and returns the Tool.
func (b *ToolBuilder) Create() *Tool {
	schema := b.buildSchema()

	return &Tool{
		Name:        b.name,
		Description: b.buildDescription(),
		Parameters:  schema,
		Execute:     b.createExecutor(),
	}
}

// buildSchema creates the JSON schema from parameters.
func (b *ToolBuilder) buildSchema() Schema {
	props := make(map[string]Property)
	required := make([]string, 0)

	for _, p := range b.params {
		prop := Property{
			Type:        p.paramType,
			Description: p.description,
		}
		if len(p.enumValues) > 0 {
			prop.Enum = p.enumValues
		}
		props[p.name] = prop

		if p.required {
			required = append(required, p.name)
		}
	}

	return Schema{
		Type:       "object",
		Properties: props,
		Required:   required,
	}
}

// buildDescription creates the full description with examples.
func (b *ToolBuilder) buildDescription() string {
	desc := b.description
	if len(b.examples) > 0 {
		desc += "\n\nExamples:"
		for _, ex := range b.examples {
			desc += "\n- " + ex
		}
	}
	return desc
}

// createExecutor wraps the handler in a JSON executor.
func (b *ToolBuilder) createExecutor() func(ctx context.Context, jsonInput string) (string, error) {
	return func(ctx context.Context, jsonInput string) (string, error) {
		switch fn := b.handler.(type) {
		case func(ctx context.Context, input string) (string, error):
			// Simple string handler - extract first param or use raw
			var params map[string]any
			if err := json.Unmarshal([]byte(jsonInput), &params); err == nil {
				// Try to get the first param value
				for _, p := range b.params {
					if val, ok := params[p.name]; ok {
						if strVal, ok := val.(string); ok {
							return fn(ctx, strVal)
						}
					}
				}
			}
			return fn(ctx, jsonInput)
		case nil:
			return "", fmt.Errorf("no handler configured for tool %s", b.name)
		default:
			return "", fmt.Errorf("unsupported handler type for tool %s", b.name)
		}
	}
}

// QuickTool creates a simple tool with minimal configuration.
func QuickTool(name, description string, fn func(ctx context.Context, input string) (string, error)) *Tool {
	return Build(name).
		Description(description).
		Param("input", "string", "The input to process").
		Handler(fn).
		Create()
}

// MapTool creates a tool from a map of param name -> description.
func MapTool(name, description string, params map[string]string, fn func(ctx context.Context, params map[string]any) (string, error)) *Tool {
	props := make(map[string]Property)
	required := make([]string, 0)

	for pName, pDesc := range params {
		props[pName] = Property{
			Type:        "string",
			Description: pDesc,
		}
		required = append(required, pName)
	}

	return &Tool{
		Name:        name,
		Description: description,
		Parameters: Schema{
			Type:       "object",
			Properties: props,
			Required:   required,
		},
		Execute: func(ctx context.Context, jsonInput string) (string, error) {
			var p map[string]any
			if err := json.Unmarshal([]byte(jsonInput), &p); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}
			return fn(ctx, p)
		},
	}
}

// FuncTool creates a typed tool from a function using generics.
// The function should have the signature: func(ctx, Input) (Output, error)
func FuncTool[I, O any](name, description string, fn func(ctx context.Context, input I) (O, error)) *Tool {
	return NewTool(name, description, fn)
}

// ToolGroup helps organize related tools.
type ToolGroup struct {
	name        string
	description string
	tools       []*Tool
}

// NewToolGroup creates a new tool group.
func NewToolGroup(name, description string) *ToolGroup {
	return &ToolGroup{
		name:        name,
		description: description,
		tools:       make([]*Tool, 0),
	}
}

// Add adds a tool to the group.
func (g *ToolGroup) Add(tool *Tool) *ToolGroup {
	g.tools = append(g.tools, tool)
	return g
}

// Tools returns all tools in the group.
func (g *ToolGroup) Tools() []*Tool {
	return g.tools
}

// RegisterTo registers all tools in the group to a registry.
func (g *ToolGroup) RegisterTo(registry *Registry) error {
	for _, tool := range g.tools {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}
