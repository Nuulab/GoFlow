// Package prompt provides template rendering logic for GoFlow.
// It uses Go's text/template engine with strict input validation.
package prompt

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// Template wraps Go's text/template with input validation.
type Template struct {
	name      string
	tmpl      *template.Template
	variables []string
}

// New creates a new prompt template.
// The templateStr should use Go template syntax: {{.VariableName}}
func New(name, templateStr string) (*Template, error) {
	// Parse to validate syntax
	tmpl, err := template.New(name).Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("prompt: invalid template syntax: %w", err)
	}

	// Extract variable names from template
	variables := extractVariables(templateStr)

	return &Template{
		name:      name,
		tmpl:      tmpl,
		variables: variables,
	}, nil
}

// MustNew creates a new template and panics if parsing fails.
func MustNew(name, templateStr string) *Template {
	t, err := New(name, templateStr)
	if err != nil {
		panic(err)
	}
	return t
}

// Render executes the template with the given variables.
// Returns an error if required variables are missing.
func (t *Template) Render(vars map[string]any) (string, error) {
	// Validate all required variables are present
	missing := make([]string, 0)
	for _, v := range t.variables {
		if _, ok := vars[v]; !ok {
			missing = append(missing, v)
		}
	}

	if len(missing) > 0 {
		return "", fmt.Errorf("prompt: missing required variables: %s", strings.Join(missing, ", "))
	}

	var buf bytes.Buffer
	if err := t.tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("prompt: template execution failed: %w", err)
	}

	return buf.String(), nil
}

// Variables returns the list of variables expected by this template.
func (t *Template) Variables() []string {
	return append([]string{}, t.variables...)
}

// Name returns the template name.
func (t *Template) Name() string {
	return t.name
}

// extractVariables extracts variable names from a template string.
// This is a simple implementation that looks for {{.VarName}} patterns.
func extractVariables(templateStr string) []string {
	vars := make(map[string]struct{})
	
	// Find all {{.VarName}} patterns
	inBrace := false
	current := strings.Builder{}
	
	for i := 0; i < len(templateStr)-1; i++ {
		if templateStr[i] == '{' && templateStr[i+1] == '{' {
			inBrace = true
			current.Reset()
			i++ // Skip the second brace
			continue
		}
		
		if inBrace && templateStr[i] == '}' && i+1 < len(templateStr) && templateStr[i+1] == '}' {
			inBrace = false
			varExpr := strings.TrimSpace(current.String())
			
			// Handle simple .VarName patterns
			if strings.HasPrefix(varExpr, ".") {
				varName := strings.TrimPrefix(varExpr, ".")
				// Handle field access like .User.Name - just take the first part
				if idx := strings.Index(varName, "."); idx != -1 {
					varName = varName[:idx]
				}
				// Skip built-in functions and complex expressions
				if idx := strings.Index(varName, " "); idx != -1 {
					varName = varName[:idx]
				}
				if varName != "" && isValidIdentifier(varName) {
					vars[varName] = struct{}{}
				}
			}
			i++ // Skip the second brace
			continue
		}
		
		if inBrace {
			current.WriteByte(templateStr[i])
		}
	}
	
	result := make([]string, 0, len(vars))
	for v := range vars {
		result = append(result, v)
	}
	return result
}

// isValidIdentifier checks if a string is a valid Go identifier.
func isValidIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_') {
				return false
			}
		} else {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
				return false
			}
		}
	}
	return true
}

// Builder provides a fluent API for constructing prompts.
type Builder struct {
	parts []string
}

// NewBuilder creates a new prompt builder.
func NewBuilder() *Builder {
	return &Builder{
		parts: make([]string, 0),
	}
}

// System adds a system message section.
func (b *Builder) System(content string) *Builder {
	b.parts = append(b.parts, fmt.Sprintf("[SYSTEM]\n%s\n", content))
	return b
}

// User adds a user message section.
func (b *Builder) User(content string) *Builder {
	b.parts = append(b.parts, fmt.Sprintf("[USER]\n%s\n", content))
	return b
}

// Context adds context information.
func (b *Builder) Context(content string) *Builder {
	b.parts = append(b.parts, fmt.Sprintf("[CONTEXT]\n%s\n", content))
	return b
}

// Build returns the constructed prompt.
func (b *Builder) Build() string {
	return strings.Join(b.parts, "\n")
}
