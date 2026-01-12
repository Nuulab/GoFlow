// Package prompt provides dynamic system prompt generation.
package prompt

import (
	"bytes"
	"strings"
	"text/template"
)

// DynamicPrompt generates system prompts based on runtime context.
type DynamicPrompt struct {
	template *template.Template
	base     string
	sections []Section
}

// Section is a conditional prompt section.
type Section struct {
	Name      string
	Content   string
	Condition func(ctx *PromptContext) bool
}

// PromptContext provides data for prompt generation.
type PromptContext struct {
	AgentName    string
	Input        string
	History      []HistoryItem
	State        map[string]any
	Metadata     map[string]string
	CallCount    int
	Custom       map[string]any
}

// HistoryItem represents a conversation turn.
type HistoryItem struct {
	Role    string
	Content string
}

// Builder creates dynamic prompts.
type Builder struct {
	base     string
	sections []Section
	helpers  template.FuncMap
}

// NewBuilder creates a new prompt builder.
func NewBuilder(base string) *Builder {
	return &Builder{
		base:     base,
		sections: make([]Section, 0),
		helpers: template.FuncMap{
			"upper": strings.ToUpper,
			"lower": strings.ToLower,
			"trim":  strings.TrimSpace,
			"join":  strings.Join,
		},
	}
}

// AddSection adds a conditional section.
func (b *Builder) AddSection(name, content string, condition func(ctx *PromptContext) bool) *Builder {
	b.sections = append(b.sections, Section{
		Name:      name,
		Content:   content,
		Condition: condition,
	})
	return b
}

// AddHelper adds a template helper function.
func (b *Builder) AddHelper(name string, fn any) *Builder {
	b.helpers[name] = fn
	return b
}

// Always adds a section that always appears.
func (b *Builder) Always(name, content string) *Builder {
	return b.AddSection(name, content, func(ctx *PromptContext) bool { return true })
}

// When adds a section that appears when state key exists.
func (b *Builder) When(name, content, stateKey string) *Builder {
	return b.AddSection(name, content, func(ctx *PromptContext) bool {
		_, ok := ctx.State[stateKey]
		return ok
	})
}

// WhenNot adds a section that appears when state key doesn't exist.
func (b *Builder) WhenNot(name, content, stateKey string) *Builder {
	return b.AddSection(name, content, func(ctx *PromptContext) bool {
		_, ok := ctx.State[stateKey]
		return !ok
	})
}

// OnFirstCall adds a section only on the first call.
func (b *Builder) OnFirstCall(name, content string) *Builder {
	return b.AddSection(name, content, func(ctx *PromptContext) bool {
		return ctx.CallCount == 0
	})
}

// AfterFirstCall adds a section after the first call.
func (b *Builder) AfterFirstCall(name, content string) *Builder {
	return b.AddSection(name, content, func(ctx *PromptContext) bool {
		return ctx.CallCount > 0
	})
}

// Build creates the dynamic prompt.
func (b *Builder) Build() (*DynamicPrompt, error) {
	fullTemplate := b.base + "\n"
	for _, s := range b.sections {
		fullTemplate += "{{if ." + s.Name + "}}\n" + s.Content + "\n{{end}}\n"
	}

	tmpl, err := template.New("prompt").Funcs(b.helpers).Parse(fullTemplate)
	if err != nil {
		return nil, err
	}

	return &DynamicPrompt{
		template: tmpl,
		base:     b.base,
		sections: b.sections,
	}, nil
}

// Generate creates a prompt from the context.
func (dp *DynamicPrompt) Generate(ctx *PromptContext) (string, error) {
	// Build template data with section conditions
	data := map[string]any{
		"AgentName": ctx.AgentName,
		"Input":     ctx.Input,
		"History":   ctx.History,
		"State":     ctx.State,
		"Metadata":  ctx.Metadata,
		"CallCount": ctx.CallCount,
		"Custom":    ctx.Custom,
	}

	// Evaluate section conditions
	for _, s := range dp.sections {
		data[s.Name] = s.Condition(ctx)
	}

	var buf bytes.Buffer
	if err := dp.template.Execute(&buf, data); err != nil {
		return "", err
	}

	return strings.TrimSpace(buf.String()), nil
}

// ============ Pre-built Prompt Templates ============

// AssistantPrompt creates a standard assistant prompt.
func AssistantPrompt() *Builder {
	return NewBuilder(`You are a helpful AI assistant.`).
		OnFirstCall("greeting", `Start by greeting the user warmly.`).
		When("user_name", `The user's name is {{.State.user_name}}. Address them by name.`, "user_name").
		AfterFirstCall("context", `Continue the conversation naturally based on previous context.`)
}

// ResearcherPrompt creates a research-focused prompt.
func ResearcherPrompt() *Builder {
	return NewBuilder(`You are a research assistant. Your job is to gather accurate information.`).
		Always("instructions", `
- Search for reliable sources
- Verify information across multiple sources
- Cite your sources
- Be thorough but concise`).
		When("topic", `Focus your research on: {{.State.topic}}`, "topic")
}

// CoderPrompt creates a coding assistant prompt.
func CoderPrompt() *Builder {
	return NewBuilder(`You are an expert programmer.`).
		When("language", `You specialize in {{.State.language}}.`, "language").
		When("codebase", `You are working on: {{.State.codebase}}`, "codebase").
		Always("style", `
- Write clean, readable code
- Include comments for complex logic
- Follow best practices
- Handle errors appropriately`)
}

// ============ Simple Dynamic Prompt ============

// Simple creates a simple template-based dynamic prompt.
func Simple(template string) *DynamicPrompt {
	builder := NewBuilder(template)
	dp, _ := builder.Build()
	return dp
}

// WithState creates a prompt that includes state data.
func WithState(base string, stateKeys ...string) *DynamicPrompt {
	builder := NewBuilder(base)
	for _, key := range stateKeys {
		builder.When(key, "{{index .State \""+key+"\"}}", key)
	}
	dp, _ := builder.Build()
	return dp
}
