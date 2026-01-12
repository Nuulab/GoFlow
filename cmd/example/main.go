// Package main demonstrates basic usage of the GoFlow framework.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nuulab/goflow/pkg/agent"
	"github.com/nuulab/goflow/pkg/cache"
	"github.com/nuulab/goflow/pkg/core"
	"github.com/nuulab/goflow/pkg/engine"
	"github.com/nuulab/goflow/pkg/prompt"
	"github.com/nuulab/goflow/pkg/queue"
	"github.com/nuulab/goflow/pkg/tools"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Example 1: Using prompt templates
	fmt.Println("=== Prompt Template Example ===")
	tmpl, err := prompt.New("greeting", "Hello, {{.Name}}! You are interested in {{.Topic}}.")
	if err != nil {
		log.Fatal(err)
	}

	rendered, err := tmpl.Render(map[string]any{
		"Name":  "Developer",
		"Topic": "AI orchestration",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Rendered prompt:", rendered)
	fmt.Println()

	// Example 2: Using the pipeline engine with generics
	fmt.Println("=== Pipeline Example ===")

	parseInput := func(ctx context.Context, input string) (int, error) {
		var num int
		_, err := fmt.Sscanf(input, "%d", &num)
		if err != nil {
			return 0, fmt.Errorf("failed to parse input: %w", err)
		}
		return num, nil
	}

	double := func(ctx context.Context, n int) (int, error) {
		return n * 2, nil
	}

	toString := func(ctx context.Context, n int) (string, error) {
		return fmt.Sprintf("Result: %d", n), nil
	}

	parseAndDouble := engine.Chain(parseInput, double)
	fullPipeline := engine.Chain(parseAndDouble, toString)

	result, err := fullPipeline(ctx, "21")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result)
	fmt.Println()

	// Example 3: Parallel execution
	fmt.Println("=== Parallel Execution Example ===")

	slowOp := func(multiplier int) engine.Link[int, int] {
		return func(ctx context.Context, n int) (int, error) {
			time.Sleep(100 * time.Millisecond)
			return n * multiplier, nil
		}
	}

	results, err := engine.Parallel(ctx, 10,
		slowOp(2),
		slowOp(3),
		slowOp(4),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Parallel results: %v\n", results)
	fmt.Println()

	// Example 4: Map for concurrent processing
	fmt.Println("=== Map Example ===")

	square := func(ctx context.Context, n int) (int, error) {
		return n * n, nil
	}

	inputs := []int{1, 2, 3, 4, 5}
	squared, err := engine.Map(ctx, inputs, square)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Squared: %v\n", squared)
	fmt.Println()

	// Example 5: Tool registration
	fmt.Println("=== Tools Example ===")

	registry := tools.NewRegistry()

	type CalcInput struct {
		A  int    `json:"a"`
		B  int    `json:"b"`
		Op string `json:"op"`
	}

	type CalcOutput struct {
		Result int `json:"result"`
	}

	calcTool := tools.NewTool("calculator", "Performs basic arithmetic", func(ctx context.Context, input CalcInput) (CalcOutput, error) {
		var result int
		switch input.Op {
		case "add":
			result = input.A + input.B
		case "mul":
			result = input.A * input.B
		default:
			return CalcOutput{}, fmt.Errorf("unknown operation: %s", input.Op)
		}
		return CalcOutput{Result: result}, nil
	})

	if err := registry.Register(calcTool); err != nil {
		log.Fatal(err)
	}

	output, err := registry.Execute(ctx, "calculator", `{"a": 10, "b": 5, "op": "mul"}`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Calculator result:", output)
	fmt.Println()

	// Example 6: Cache (using in-memory for demo)
	fmt.Println("=== Cache Example (In-Memory) ===")

	memCache := cache.NewMemoryCache(cache.Config{
		Prefix:     "goflow",
		DefaultTTL: 5 * time.Minute,
	})
	defer memCache.Close()

	// Set a value
	err = memCache.Set(ctx, "user:123", []byte(`{"name":"Alice","score":100}`), 0)
	if err != nil {
		log.Fatal(err)
	}

	// Get the value
	data, err := memCache.Get(ctx, "user:123")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Cached value:", string(data))

	// Use typed cache for type-safe operations
	type User struct {
		Name  string `json:"name"`
		Score int    `json:"score"`
	}

	typedCache := cache.NewTypedCache[User](memCache)
	if err := typedCache.Set(ctx, "user:456", User{Name: "Bob", Score: 200}, 0); err != nil {
		log.Fatal(err)
	}

	user, err := typedCache.Get(ctx, "user:456")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Typed cache user: %+v\n", user)

	stats, _ := memCache.Stats(ctx)
	fmt.Printf("Cache stats: %d keys\n", stats.KeyCount)
	fmt.Println()

	// Example 7: Job Queue (in-memory simulation)
	fmt.Println("=== Queue Example ===")

	type EmailJob struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
	}

	job, err := queue.NewJob("send_email", EmailJob{
		To:      "user@example.com",
		Subject: "Welcome!",
	})
	if err != nil {
		log.Fatal(err)
	}
	job.WithPriority(10).WithMaxRetries(3)

	fmt.Printf("Created job: ID=%s Type=%s Priority=%d\n", job.ID, job.Type, job.Priority)
	fmt.Println()

	// Example 8: Agent (Conceptual)
	fmt.Println("=== Agent Example ===")

	agentRegistry := tools.BuiltinTools()
	agentRegistry.Register(tools.CalculatorTool())

	fmt.Println("Available built-in tools:")
	for _, tool := range agentRegistry.List() {
		fmt.Printf("  - %s: %s\n", tool.Name, tool.Description)
	}

	bufferMem := agent.NewBufferMemory(20)
	bufferMem.Add(core.Message{Role: core.RoleUser, Content: "Hello!"})
	fmt.Printf("Agent memory: %d messages\n", len(bufferMem.Get()))
	fmt.Println()

	// DragonflyDB usage note
	fmt.Println("=== DragonflyDB Integration ===")
	fmt.Println(`
For production use with DragonflyDB:

  // Start DragonflyDB: docker run -p 6379:6379 docker.dragonflydb.io/dragonflydb/dragonfly
  
  // Connect to DragonflyDB cache
  dfCache, err := cache.NewDragonflyCache(cache.Config{
      Address:    "localhost:6379",
      Prefix:     "goflow",
      DefaultTTL: 5 * time.Minute,
  })
  
  // Connect to DragonflyDB queue
  dfQueue, err := queue.NewDragonflyQueue(queue.Config{
      Address:   "localhost:6379",
      QueueName: "goflow:jobs",
  })
  
  // Process jobs with workers
  worker := queue.NewWorker(dfQueue)
  worker.Handle("send_email", func(ctx context.Context, job *queue.Job) error {
      // Process the job
      return nil
  })
  worker.Start(ctx, 5) // 5 concurrent workers
	`)

	fmt.Println("✅ GoFlow framework ready for production use!")
}
