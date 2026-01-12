// GoFlow Worker - Processes queue jobs in Swarm mode
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nuulab/goflow/pkg/core"
	"github.com/nuulab/goflow/pkg/queue"
	"github.com/nuulab/goflow/pkg/tools"
)

func main() {
	// Flags
	concurrency := flag.Int("concurrency", 5, "Number of concurrent workers")
	redisAddr := flag.String("redis", "localhost:6379", "Redis/DragonflyDB address")
	flag.Parse()

	// Environment overrides
	if envRedis := os.Getenv("GOFLOW_REDIS"); envRedis != "" {
		*redisAddr = envRedis
	}
	if envConc := os.Getenv("GOFLOW_WORKER_CONCURRENCY"); envConc != "" {
		fmt.Sscanf(envConc, "%d", concurrency)
	}

	// Banner
	fmt.Println("🔧 GoFlow Worker")
	fmt.Printf("   Redis: %s\n", *redisAddr)
	fmt.Printf("   Concurrency: %d\n", *concurrency)

	// Connect to queue
	q, err := queue.NewDragonflyQueue(queue.Config{
		Address:   *redisAddr,
		QueueName: "goflow:jobs",
	})
	if err != nil {
		log.Fatalf("Failed to connect to queue: %v", err)
	}
	log.Println("✅ Connected to queue")

	// Create LLM (stub - replace with real)
	llm := &StubLLM{}

	// Create tool registry
	registry := tools.BuiltinTools()
	_ = registry // Used for agent tasks
	_ = llm

	// Create worker
	worker := queue.NewWorker(q)

	// Register job handlers
	worker.Handle("agent_task", agentTaskHandler())
	worker.Handle("workflow_step", workflowStepHandler())
	worker.Handle("send_email", sendEmailHandler())
	worker.Handle("webhook", webhookHandler())

	log.Printf("📋 Registered %d job handlers", 4)

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("\n🛑 Shutting down worker...")
		cancel()
		worker.Stop()
	}()

	// Start processing
	log.Printf("🚀 Starting %d workers...", *concurrency)
	worker.Start(ctx, *concurrency)

	// Wait for shutdown
	<-ctx.Done()
	log.Println("👋 Worker stopped")
}

// ============ Job Handlers ============

// AgentTaskPayload is the payload for agent tasks
type AgentTaskPayload struct {
	Task    string `json:"task"`
	AgentID string `json:"agent_id"`
}

// agentTaskHandler processes agent tasks from the queue
func agentTaskHandler() queue.Handler {
	return func(ctx context.Context, job *queue.Job) error {
		log.Printf("🤖 Processing agent task: %s", job.ID)

		var payload AgentTaskPayload
		if err := job.UnmarshalPayload(&payload); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}

		log.Printf("   Task: %s", payload.Task)
		log.Printf("   Agent: %s", payload.AgentID)

		// Simulate processing
		time.Sleep(1 * time.Second)

		log.Printf("✅ Completed agent task: %s", job.ID)
		return nil
	}
}

// WorkflowStepPayload is the payload for workflow steps
type WorkflowStepPayload struct {
	WorkflowID string `json:"workflow_id"`
	StepName   string `json:"step_name"`
}

// workflowStepHandler processes workflow steps
func workflowStepHandler() queue.Handler {
	return func(ctx context.Context, job *queue.Job) error {
		log.Printf("🔄 Processing workflow step: %s", job.ID)

		var payload WorkflowStepPayload
		if err := job.UnmarshalPayload(&payload); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}

		log.Printf("   Workflow: %s, Step: %s", payload.WorkflowID, payload.StepName)

		// Process step...
		time.Sleep(500 * time.Millisecond)

		log.Printf("✅ Completed workflow step: %s", job.ID)
		return nil
	}
}

// EmailPayload is the payload for email jobs
type EmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// sendEmailHandler processes email jobs
func sendEmailHandler() queue.Handler {
	return func(ctx context.Context, job *queue.Job) error {
		log.Printf("📧 Sending email: %s", job.ID)

		var payload EmailPayload
		if err := job.UnmarshalPayload(&payload); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}

		log.Printf("   To: %s, Subject: %s", payload.To, payload.Subject)

		// Send email...
		time.Sleep(200 * time.Millisecond)

		log.Printf("✅ Email sent: %s", job.ID)
		return nil
	}
}

// WebhookPayload is the payload for webhook deliveries
type WebhookPayload struct {
	URL     string          `json:"url"`
	Method  string          `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage `json:"body"`
}

// webhookHandler processes webhook deliveries
func webhookHandler() queue.Handler {
	return func(ctx context.Context, job *queue.Job) error {
		log.Printf("🔗 Delivering webhook: %s", job.ID)

		var payload WebhookPayload
		if err := job.UnmarshalPayload(&payload); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}

		log.Printf("   URL: %s", payload.URL)

		// Deliver webhook...
		time.Sleep(300 * time.Millisecond)

		log.Printf("✅ Webhook delivered: %s", job.ID)
		return nil
	}
}

// StubLLM placeholder
type StubLLM struct{}

func (s *StubLLM) Generate(ctx context.Context, prompt string, opts ...core.Option) (string, error) {
	return `{"action": "final_answer", "action_input": "Stub response"}`, nil
}

func (s *StubLLM) GenerateChat(ctx context.Context, messages []core.Message, opts ...core.Option) (string, error) {
	return `{"action": "final_answer", "action_input": "Stub response"}`, nil
}

func (s *StubLLM) Stream(ctx context.Context, prompt string, opts ...core.Option) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- `{"action": "final_answer", "action_input": "Stub"}`
	close(ch)
	return ch, nil
}

func (s *StubLLM) StreamChat(ctx context.Context, messages []core.Message, opts ...core.Option) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- `{"action": "final_answer", "action_input": "Stub"}`
	close(ch)
	return ch, nil
}

func (s *StubLLM) CountTokens(ctx context.Context, text string) (int, error) {
	return len(text) / 4, nil
}
