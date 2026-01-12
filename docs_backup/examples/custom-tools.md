# Custom Tools

Create custom tools for your agents.

## Simple Tool

```go
package main

import (
	"context"
	"fmt"
	
	"github.com/goflow/goflow/pkg/tools"
)

func main() {
	// Simple tool with string input/output
	greetTool := tools.NewTool(
		"greet",
		"Greet a person by name",
		func(ctx context.Context, name string) (string, error) {
			return fmt.Sprintf("Hello, %s! Welcome to GoFlow.", name), nil
		},
	)

	registry := tools.NewRegistry()
	registry.Register(greetTool)
}
```

## Tool with Structured Input

```go
// Define input schema
type WeatherInput struct {
	City    string `json:"city" description:"City name"`
	Country string `json:"country" description:"Country code (optional)"`
	Units   string `json:"units" description:"celsius or fahrenheit"`
}

weatherTool := tools.NewToolWithSchema(
	"get_weather",
	"Get current weather for a city",
	WeatherInput{}, // Schema is derived from struct
	func(ctx context.Context, input WeatherInput) (string, error) {
		// Call weather API
		weather := callWeatherAPI(input.City, input.Country)
		return fmt.Sprintf("Weather in %s: %s, %d°%s", 
			input.City, 
			weather.Condition, 
			weather.Temp,
			input.Units[0:1],
		), nil
	},
)
```

## Async Tool

```go
type ProcessResult struct {
	Status  string
	Records int
}

asyncTool := tools.NewAsyncTool(
	"process_large_file",
	"Process a large file in the background",
	func(ctx context.Context, filename string) <-chan tools.Result {
		ch := make(chan tools.Result, 1)
		
		go func() {
			defer close(ch)
			
			// Simulate long processing
			records := processFile(filename)
			
			ch <- tools.Result{
				Output: fmt.Sprintf("Processed %d records", records),
			}
		}()
		
		return ch
	},
)
```

## Tool with Validation

```go
type EmailInput struct {
	To      string `json:"to" validate:"required,email"`
	Subject string `json:"subject" validate:"required,min=1,max=200"`
	Body    string `json:"body" validate:"required"`
}

emailTool := tools.NewToolWithSchema(
	"send_email",
	"Send an email",
	EmailInput{},
	func(ctx context.Context, input EmailInput) (string, error) {
		if err := sendEmail(input.To, input.Subject, input.Body); err != nil {
			return "", err
		}
		return "Email sent successfully", nil
	},
)
```

## Tool with Context

```go
type DatabaseTool struct {
	db *sql.DB
}

func (d *DatabaseTool) QueryTool() *tools.Tool {
	return tools.NewTool(
		"query_db",
		"Run a SQL query",
		func(ctx context.Context, query string) (string, error) {
			rows, err := d.db.QueryContext(ctx, query)
			if err != nil {
				return "", err
			}
			return formatResults(rows), nil
		},
	)
}

// Usage
dbTool := &DatabaseTool{db: database}
registry.Register(dbTool.QueryTool())
```

## Creating a Toolkit

```go
func CRMToolkit(apiClient *crm.Client) *tools.Toolkit {
	return &tools.Toolkit{
		Name: "crm",
		Tools: []*tools.Tool{
			tools.NewTool("crm_get_contact", "Get contact by ID", 
				func(ctx context.Context, id string) (string, error) {
					contact, _ := apiClient.GetContact(id)
					return json.Marshal(contact)
				}),
			tools.NewTool("crm_create_contact", "Create a new contact",
				func(ctx context.Context, data string) (string, error) {
					var contact Contact
					json.Unmarshal([]byte(data), &contact)
					return apiClient.CreateContact(contact)
				}),
			tools.NewTool("crm_search", "Search contacts",
				func(ctx context.Context, query string) (string, error) {
					results, _ := apiClient.Search(query)
					return json.Marshal(results)
				}),
		},
	}
}

// Usage
registry.RegisterToolkit(CRMToolkit(crmClient))
```
