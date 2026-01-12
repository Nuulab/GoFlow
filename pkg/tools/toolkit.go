// Package tools provides pre-built tool collections (toolkits).
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Toolkit is a collection of related tools.
type Toolkit struct {
	Name        string
	Description string
	Tools       []*Tool
}

// RegisterTo registers all toolkit tools to a registry.
func (tk *Toolkit) RegisterTo(registry *Registry) error {
	for _, tool := range tk.Tools {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

// WebToolkit returns tools for web interactions.
func WebToolkit() *Toolkit {
	return &Toolkit{
		Name:        "web",
		Description: "Tools for web interactions: HTTP requests, APIs, and web content",
		Tools: []*Tool{
			httpGetTool(),
			httpPostTool(),
			jsonAPITool(),
			urlEncodeTool(),
		},
	}
}

func httpGetTool() *Tool {
	return Build("http_get").
		Description("Fetch content from a URL using HTTP GET").
		Param("url", "string", "The URL to fetch").
		OptionalParam("headers", "object", "Optional HTTP headers").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				URL     string            `json:"url"`
				Headers map[string]string `json:"headers"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				params.URL = input
			}

			req, err := http.NewRequestWithContext(ctx, "GET", params.URL, nil)
			if err != nil {
				return "", fmt.Errorf("failed to create request: %w", err)
			}

			for k, v := range params.Headers {
				req.Header.Set(k, v)
			}

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return "", fmt.Errorf("request failed: %w", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // 1MB limit
			if err != nil {
				return "", fmt.Errorf("failed to read response: %w", err)
			}

			return string(body), nil
		}).
		Create()
}

func httpPostTool() *Tool {
	return Build("http_post").
		Description("Send data to a URL using HTTP POST").
		Param("url", "string", "The URL to post to").
		Param("body", "string", "The request body").
		OptionalParam("content_type", "string", "Content-Type header (default: application/json)").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				URL         string `json:"url"`
				Body        string `json:"body"`
				ContentType string `json:"content_type"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}

			if params.ContentType == "" {
				params.ContentType = "application/json"
			}

			req, err := http.NewRequestWithContext(ctx, "POST", params.URL, strings.NewReader(params.Body))
			if err != nil {
				return "", fmt.Errorf("failed to create request: %w", err)
			}
			req.Header.Set("Content-Type", params.ContentType)

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return "", fmt.Errorf("request failed: %w", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
			if err != nil {
				return "", fmt.Errorf("failed to read response: %w", err)
			}

			return string(body), nil
		}).
		Create()
}

func jsonAPITool() *Tool {
	return Build("json_api").
		Description("Make a JSON API request and parse the response").
		EnumParam("method", "HTTP method", "GET", "POST", "PUT", "DELETE").
		Param("url", "string", "The API endpoint URL").
		OptionalParam("data", "object", "JSON data to send (for POST/PUT)").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				Method string         `json:"method"`
				URL    string         `json:"url"`
				Data   map[string]any `json:"data"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}

			var body io.Reader
			if params.Data != nil && (params.Method == "POST" || params.Method == "PUT") {
				jsonData, _ := json.Marshal(params.Data)
				body = strings.NewReader(string(jsonData))
			}

			req, err := http.NewRequestWithContext(ctx, params.Method, params.URL, body)
			if err != nil {
				return "", err
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json")

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return "", err
			}
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(resp.Body)

			// Pretty-print if JSON
			var prettyJSON map[string]any
			if err := json.Unmarshal(respBody, &prettyJSON); err == nil {
				formatted, _ := json.MarshalIndent(prettyJSON, "", "  ")
				return string(formatted), nil
			}

			return string(respBody), nil
		}).
		Create()
}

func urlEncodeTool() *Tool {
	return Build("url_encode").
		Description("URL encode/decode strings").
		EnumParam("action", "Action to perform", "encode", "decode").
		Param("text", "string", "Text to encode or decode").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				Action string `json:"action"`
				Text   string `json:"text"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				return "", err
			}

			switch params.Action {
			case "encode":
				return url.QueryEscape(params.Text), nil
			case "decode":
				return url.QueryUnescape(params.Text)
			default:
				return "", fmt.Errorf("unknown action: %s", params.Action)
			}
		}).
		Create()
}

// DataToolkit returns tools for data manipulation.
func DataToolkit() *Toolkit {
	return &Toolkit{
		Name:        "data",
		Description: "Tools for data manipulation: JSON, text processing, formatting",
		Tools: []*Tool{
			jsonParseTool(),
			jsonFormatTool(),
			textTransformTool(),
			templateTool(),
		},
	}
}

func jsonParseTool() *Tool {
	return Build("json_parse").
		Description("Parse JSON and extract values using dot notation (e.g., 'user.name')").
		Param("json", "string", "JSON string to parse").
		OptionalParam("path", "string", "Dot-notation path to extract (e.g., 'data.items[0].name')").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				JSON string `json:"json"`
				Path string `json:"path"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				params.JSON = input
			}

			var data any
			if err := json.Unmarshal([]byte(params.JSON), &data); err != nil {
				return "", fmt.Errorf("invalid JSON: %w", err)
			}

			if params.Path == "" {
				formatted, _ := json.MarshalIndent(data, "", "  ")
				return string(formatted), nil
			}

			// Simple path extraction
			result := extractPath(data, params.Path)
			if result == nil {
				return "null", nil
			}

			out, _ := json.Marshal(result)
			return string(out), nil
		}).
		Create()
}

func extractPath(data any, path string) any {
	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			current = v[part]
		default:
			return nil
		}
	}

	return current
}

func jsonFormatTool() *Tool {
	return Build("json_format").
		Description("Format/prettify JSON").
		Param("json", "string", "JSON to format").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				JSON string `json:"json"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				params.JSON = input
			}

			var data any
			if err := json.Unmarshal([]byte(params.JSON), &data); err != nil {
				return "", err
			}

			formatted, err := json.MarshalIndent(data, "", "  ")
			return string(formatted), err
		}).
		Create()
}

func textTransformTool() *Tool {
	return Build("text_transform").
		Description("Transform text: uppercase, lowercase, trim, etc.").
		EnumParam("operation", "Transformation", "uppercase", "lowercase", "trim", "split", "join", "replace").
		Param("text", "string", "Text to transform").
		OptionalParam("arg", "string", "Additional argument (delimiter for split/join, search for replace)").
		OptionalParam("arg2", "string", "Second argument (replacement for replace)").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				Operation string `json:"operation"`
				Text      string `json:"text"`
				Arg       string `json:"arg"`
				Arg2      string `json:"arg2"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				return "", err
			}

			switch params.Operation {
			case "uppercase":
				return strings.ToUpper(params.Text), nil
			case "lowercase":
				return strings.ToLower(params.Text), nil
			case "trim":
				return strings.TrimSpace(params.Text), nil
			case "split":
				parts := strings.Split(params.Text, params.Arg)
				out, _ := json.Marshal(parts)
				return string(out), nil
			case "join":
				var parts []string
				json.Unmarshal([]byte(params.Text), &parts)
				return strings.Join(parts, params.Arg), nil
			case "replace":
				return strings.ReplaceAll(params.Text, params.Arg, params.Arg2), nil
			default:
				return "", fmt.Errorf("unknown operation: %s", params.Operation)
			}
		}).
		Create()
}

func templateTool() *Tool {
	return Build("template").
		Description("Render a template with variables").
		Param("template", "string", "Template with {{variable}} placeholders").
		Param("variables", "object", "Variables to substitute").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				Template  string         `json:"template"`
				Variables map[string]any `json:"variables"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				return "", err
			}

			result := params.Template
			for k, v := range params.Variables {
				placeholder := fmt.Sprintf("{{%s}}", k)
				result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", v))
			}

			return result, nil
		}).
		Create()
}

// MathToolkit returns tools for mathematical operations.
func MathToolkit() *Toolkit {
	return &Toolkit{
		Name:        "math",
		Description: "Tools for mathematical operations",
		Tools: []*Tool{
			CalculatorTool(),
			statisticsTool(),
			conversionTool(),
		},
	}
}

func statisticsTool() *Tool {
	return Build("statistics").
		Description("Calculate statistics on a list of numbers").
		EnumParam("operation", "Statistic to calculate", "sum", "average", "min", "max", "count").
		Param("numbers", "array", "Array of numbers").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				Operation string    `json:"operation"`
				Numbers   []float64 `json:"numbers"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				return "", err
			}

			if len(params.Numbers) == 0 {
				return "0", nil
			}

			var result float64
			switch params.Operation {
			case "sum":
				for _, n := range params.Numbers {
					result += n
				}
			case "average":
				for _, n := range params.Numbers {
					result += n
				}
				result /= float64(len(params.Numbers))
			case "min":
				result = params.Numbers[0]
				for _, n := range params.Numbers[1:] {
					if n < result {
						result = n
					}
				}
			case "max":
				result = params.Numbers[0]
				for _, n := range params.Numbers[1:] {
					if n > result {
						result = n
					}
				}
			case "count":
				result = float64(len(params.Numbers))
			}

			return fmt.Sprintf("%.6g", result), nil
		}).
		Create()
}

func conversionTool() *Tool {
	return Build("convert").
		Description("Convert between units").
		Param("value", "number", "Value to convert").
		Param("from", "string", "Source unit").
		Param("to", "string", "Target unit").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				Value float64 `json:"value"`
				From  string  `json:"from"`
				To    string  `json:"to"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				return "", err
			}

			// Temperature conversions
			if params.From == "celsius" && params.To == "fahrenheit" {
				return fmt.Sprintf("%.2f", params.Value*9/5+32), nil
			}
			if params.From == "fahrenheit" && params.To == "celsius" {
				return fmt.Sprintf("%.2f", (params.Value-32)*5/9), nil
			}

			// Length conversions
			conversionFactors := map[string]map[string]float64{
				"meters":     {"feet": 3.28084, "inches": 39.3701, "km": 0.001, "miles": 0.000621371},
				"feet":       {"meters": 0.3048, "inches": 12, "miles": 0.000189394},
				"km":         {"miles": 0.621371, "meters": 1000},
				"miles":      {"km": 1.60934, "meters": 1609.34, "feet": 5280},
				"kg":         {"lbs": 2.20462, "grams": 1000},
				"lbs":        {"kg": 0.453592, "grams": 453.592},
			}

			if factors, ok := conversionFactors[params.From]; ok {
				if factor, ok := factors[params.To]; ok {
					return fmt.Sprintf("%.6g", params.Value*factor), nil
				}
			}

			return "", fmt.Errorf("unsupported conversion: %s to %s", params.From, params.To)
		}).
		Create()
}

// AllToolkits returns all available toolkits.
func AllToolkits() []*Toolkit {
	return []*Toolkit{
		WebToolkit(),
		DataToolkit(),
		MathToolkit(),
	}
}

// RegisterAllToolkits registers all toolkits to a registry.
func RegisterAllToolkits(registry *Registry) error {
	for _, tk := range AllToolkits() {
		if err := tk.RegisterTo(registry); err != nil {
			return err
		}
	}
	return nil
}
