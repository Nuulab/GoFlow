// GoFlow Server - Standalone AI Orchestration Service
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nuulab/goflow/pkg/api"
	"github.com/nuulab/goflow/pkg/cache"
	"github.com/nuulab/goflow/pkg/core"
	"github.com/nuulab/goflow/pkg/tools"
	"github.com/nuulab/goflow/pkg/workflow"
)

func main() {
	// Command line flags
	port := flag.Int("port", 8080, "Server port")
	redisAddr := flag.String("redis", "", "Redis/DragonflyDB address (optional)")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	// Environment variable overrides
	if envPort := os.Getenv("GOFLOW_PORT"); envPort != "" {
		fmt.Sscanf(envPort, "%d", port)
	}
	if envRedis := os.Getenv("GOFLOW_REDIS"); envRedis != "" {
		*redisAddr = envRedis
	}

	// Banner
	printBanner()

	// Initialize LLM (users should implement their own)
	llm := &StubLLM{} // Replace with real LLM

	// Initialize tool registry with all built-in tools
	registry := tools.BuiltinTools()
	registry.Register(tools.CalculatorTool())

	// Register all toolkits
	tools.WebToolkit().RegisterTo(registry)
	tools.DataToolkit().RegisterTo(registry)
	tools.MathToolkit().RegisterTo(registry)

	log.Printf("📦 Loaded %d tools", len(registry.List()))

	// Initialize cache if Redis is configured
	var cacheInstance cache.Cache
	if *redisAddr != "" {
		var err error
		cacheInstance, err = cache.NewDragonflyCache(cache.Config{
			Address: *redisAddr,
			Prefix:  "goflow",
		})
		if err != nil {
			log.Printf("⚠️  Cache connection failed: %v (continuing without cache)", err)
		} else {
			log.Printf("✅ Connected to cache at %s", *redisAddr)
		}
	}

	// Initialize workflow engine
	var persistence *workflow.Persistence
	if cacheInstance != nil {
		if dc, ok := cacheInstance.(*cache.DragonflyCache); ok {
			persistence = workflow.NewPersistence(dc.Client())
		}
	}
	workflowEngine := workflow.NewEngine(persistence)
	log.Printf("🔄 Workflow engine initialized")

	// Initialize cron scheduler
	cron := workflow.NewCron(workflowEngine)
	cron.Start(context.Background())
	log.Printf("⏰ Cron scheduler started")

	// Create API server
	server := api.NewServer(api.Config{
		Port:     *port,
		LLM:      llm,
		Registry: registry,
		Settings: &api.Settings{
			MaxIterations:   10,
			VerboseLogging:  *verbose,
			AllowedOrigins:  []string{"*"},
		},
	})

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("\n🛑 Shutting down...")
		cancel()
		server.Stop(context.Background())
		cron.Stop()
		if cacheInstance != nil {
			cacheInstance.Close()
		}
	}()

	// Start server
	if err := server.Start(*port); err != nil {
		log.Fatalf("Server error: %v", err)
	}

	<-ctx.Done()
}

func printBanner() {
	fmt.Println(`
   ____       _____ _               
  / ___| ___ |  ___| | _____      __
 | |  _ / _ \| |_  | |/ _ \ \ /\ / /
 | |_| | (_) |  _| | | (_) \ V  V / 
  \____|\___/|_|   |_|\___/ \_/\_/  
                                    
  AI Orchestration Framework
  `)
}

// StubLLM is a placeholder - replace with real implementation
type StubLLM struct{}

func (s *StubLLM) Generate(ctx context.Context, prompt string, opts ...core.Option) (string, error) {
	return `{"action": "final_answer", "action_input": "This is a stub LLM. Configure a real LLM provider."}`, nil
}

func (s *StubLLM) GenerateChat(ctx context.Context, messages []core.Message, opts ...core.Option) (string, error) {
	return `{"action": "final_answer", "action_input": "This is a stub LLM. Configure a real LLM provider."}`, nil
}

func (s *StubLLM) Stream(ctx context.Context, prompt string, opts ...core.Option) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- `{"action": "final_answer", "action_input": "This is a stub LLM."}`
	close(ch)
	return ch, nil
}

func (s *StubLLM) StreamChat(ctx context.Context, messages []core.Message, opts ...core.Option) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- `{"action": "final_answer", "action_input": "This is a stub LLM."}`
	close(ch)
	return ch, nil
}

func (s *StubLLM) CountTokens(ctx context.Context, text string) (int, error) {
	return len(text) / 4, nil
}
