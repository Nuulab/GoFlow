---
title: Server settings
---

title: Microservice Architecture
---


This guide walks you through setting up GoFlow as a production-ready microservice.

## Architecture Overview

GoFlow is designed to run as three separate services:

```
┌─────────────────────────────────────────────────────────────┐
│                      Load Balancer                           │
└─────────────────────────┬───────────────────────────────────┘
                          │
         ┌────────────────┼────────────────┐
         ▼                ▼                ▼
┌─────────────┐   ┌─────────────┐   ┌─────────────┐
│  API Server │   │  API Server │   │  API Server │
│  (Replica)  │   │  (Replica)  │   │  (Replica)  │
└──────┬──────┘   └──────┬──────┘   └──────┬──────┘
       │                 │                 │
       └─────────────────┼─────────────────┘
                         ▼
              ┌────────────────────┐
              │   DragonflyDB      │
              │   (State + Queue)  │
              └─────────┬──────────┘
                        │
       ┌────────────────┼────────────────┐
       ▼                ▼                ▼
┌─────────────┐   ┌─────────────┐   ┌─────────────┐
│   Worker    │   │   Worker    │   │   Worker    │
│  (Process)  │   │  (Process)  │   │  (Process)  │
└─────────────┘   └─────────────┘   └─────────────┘
       │
       │
┌─────────────┐
│  Scheduler  │  (Single instance)
└─────────────┘
```

**Three distinct services:**
1. **API Server** - Handles HTTP/WebSocket requests (scale horizontally)
2. **Worker** - Processes queued jobs (scale to match load)
3. **Scheduler** - Runs cron jobs (single instance to prevent duplicates)

## Step 1: Project Structure

Create your project with this structure:

```
myapp/
├── cmd/
│   ├── api/
│   │   └── main.go          # API server entry point
│   ├── worker/
│   │   └── main.go          # Worker entry point
│   └── scheduler/
│       └── main.go          # Scheduler entry point
├── internal/
│   ├── handlers/            # Job handlers
│   ├── workflows/           # Workflow definitions
│   └── config/              # Configuration
├── config.yaml              # Configuration file
├── Dockerfile               # Multi-stage build
├── docker-compose.yml       # Local development
└── docker-stack.yml         # Production deployment
```

This separation allows each service to scale independently and be deployed separately if needed.

## Step 2: Configuration

Create `config.yaml`:

```yaml
# Server settings
server:
  port: 8080
  cors:
    allowed_origins:
      - "*"

# Database (DragonflyDB/Redis)
database:
  address: localhost:6379
  password: ""
  database: 0

# Queue settings
queue:
  name: jobs
  max_retries: 3

# Worker settings
worker:
  concurrency: 10

# LLM settings (for agents)
llm:
  provider: openai
  api_key: ${OPENAI_API_KEY}

# Webhook settings
webhooks:
  secret: ${WEBHOOK_SECRET}
```

Environment variables (prefixed with `${}`) are substituted at runtime, keeping secrets out of your config file.

## Step 3: API Server

Create `cmd/api/main.go`:

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/nuulab/goflow/pkg/api"
    "github.com/nuulab/goflow/pkg/cache"
    "github.com/nuulab/goflow/pkg/queue"
    "github.com/nuulab/goflow/pkg/webhook"
)

func main() {
    // Load configuration
    cfg := loadConfig()

    // Connect to DragonflyDB
    db, err := cache.New(cache.Config{
        Address: cfg.Database.Address,
    })
    if err != nil {
        log.Fatal("Failed to connect to database:", err)
    }

    // Create queue
    q, err := queue.NewDragonflyQueue(queue.Config{
        Address:   cfg.Database.Address,
        QueueName: cfg.Queue.Name,
    })
    if err != nil {
        log.Fatal("Failed to create queue:", err)
    }

    // Create API server
    server := api.NewServer(api.ServerConfig{
        Port:         cfg.Server.Port,
        AllowOrigins: cfg.Server.CORS.AllowedOrigins,
        Queue:        q,
        Cache:        db,
    })

    // Setup webhooks
    webhookHandler := webhook.NewWebhookHandler(q, nil)
    webhookHandler.SetGlobalSecret(cfg.Webhooks.Secret)
    
    // Register your webhooks
    webhookHandler.RegisterJobWebhook("/github", "github_event")
    webhookHandler.RegisterJobWebhook("/stripe", "stripe_event")

    // Mount webhook handler
    http.Handle("/webhooks/", webhookHandler.Handler())

    // Start server
    go func() {
        log.Printf("API server starting on port %d", cfg.Server.Port)
        if err := server.Start(); err != nil {
            log.Fatal(err)
        }
    }()

    // Graceful shutdown
    waitForShutdown()
}

func waitForShutdown() {
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    log.Println("Shutting down...")
}
```

The API server is stateless - it reads from and writes to DragonflyDB. This means you can run as many replicas as needed behind a load balancer.

## Step 4: Worker Service

Create `cmd/worker/main.go`:

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/nuulab/goflow/pkg/queue"
    "myapp/internal/handlers"
)

func main() {
    cfg := loadConfig()

    // Connect to queue
    q, err := queue.NewDragonflyQueue(queue.Config{
        Address:   cfg.Database.Address,
        QueueName: cfg.Queue.Name,
    })
    if err != nil {
        log.Fatal("Failed to connect to queue:", err)
    }

    // Create worker
    worker := queue.NewWorker(q)

    // Register all job handlers
    worker.Handle("github_event", handlers.HandleGitHubEvent)
    worker.Handle("stripe_event", handlers.HandleStripeEvent)
    worker.Handle("send_email", handlers.HandleSendEmail)
    worker.Handle("process_upload", handlers.HandleProcessUpload)

    // Start processing
    ctx, cancel := context.WithCancel(context.Background())
    
    log.Printf("Worker starting with concurrency %d", cfg.Worker.Concurrency)
    go worker.Start(ctx, cfg.Worker.Concurrency)

    // Graceful shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    log.Println("Worker shutting down...")
    cancel()
    worker.Stop()
}
```

Workers are the workhorses - scale them based on your queue depth. If jobs are piling up, add more workers.

## Step 5: Job Handlers

Create `internal/handlers/handlers.go`:

```go
package handlers

import (
    "context"
    "encoding/json"
    "log"

    "github.com/nuulab/goflow/pkg/queue"
)

// HandleGitHubEvent processes GitHub webhook events
func HandleGitHubEvent(ctx context.Context, job *queue.Job) error {
    var payload map[string]any
    if err := job.UnmarshalPayload(&payload); err != nil {
        return err
    }

    event := payload["event"].(string)
    log.Printf("Processing GitHub event: %s", event)

    switch event {
    case "push":
        return handlePush(ctx, payload)
    case "pull_request":
        return handlePR(ctx, payload)
    default:
        log.Printf("Unknown GitHub event: %s", event)
    }

    return nil
}

// HandleStripeEvent processes Stripe webhook events
func HandleStripeEvent(ctx context.Context, job *queue.Job) error {
    var payload map[string]any
    if err := job.UnmarshalPayload(&payload); err != nil {
        return err
    }

    eventType := payload["type"].(string)
    log.Printf("Processing Stripe event: %s", eventType)

    // Handle different Stripe events
    switch eventType {
    case "payment_intent.succeeded":
        return handlePaymentSuccess(ctx, payload)
    case "customer.subscription.updated":
        return handleSubscriptionUpdate(ctx, payload)
    }

    return nil
}

// HandleSendEmail sends an email
func HandleSendEmail(ctx context.Context, job *queue.Job) error {
    type EmailPayload struct {
        To      string `json:"to"`
        Subject string `json:"subject"`
        Body    string `json:"body"`
    }

    var payload EmailPayload
    if err := job.UnmarshalPayload(&payload); err != nil {
        return err
    }

    log.Printf("Sending email to %s: %s", payload.To, payload.Subject)
    // Actual email sending logic here
    return nil
}
```

Each handler is a simple function that receives a job and returns an error. If you return an error, the job will be retried (up to max_retries).

## Step 6: Scheduler Service

Create `cmd/scheduler/main.go`:

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/nuulab/goflow/pkg/queue"
    "github.com/nuulab/goflow/pkg/workflow"
)

func main() {
    cfg := loadConfig()

    // Connect to queue
    q, err := queue.NewDragonflyQueue(queue.Config{
        Address:   cfg.Database.Address,
        QueueName: cfg.Queue.Name,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Create workflow engine
    engine := workflow.NewEngine(nil)
    
    // Register workflows
    engine.Register(buildDailyReportWorkflow())
    engine.Register(buildCleanupWorkflow())

    // Create cron scheduler
    cron := workflow.NewCron(engine)
    
    // Schedule jobs
    cron.Add("daily-report", "daily-report", "0 9 * * *", nil)
    cron.Add("cleanup", "cleanup", "0 2 * * *", nil)
    cron.Add("health-check", "health-check", "@every 5m", nil)

    // Start scheduler
    ctx, cancel := context.WithCancel(context.Background())
    cron.Start(ctx)
    log.Println("Scheduler started")

    // Wait for shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Println("Scheduler shutting down...")
    cancel()
    cron.Stop()
}
```

**Important:** Only run ONE scheduler instance. Multiple schedulers would trigger duplicate cron jobs. Docker Swarm handles this with `replicas: 1`.

## Step 7: Dockerfile

Create a multi-stage `Dockerfile`:

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /api ./cmd/api
RUN CGO_ENABLED=0 go build -o /worker ./cmd/worker
RUN CGO_ENABLED=0 go build -o /scheduler ./cmd/scheduler

# Runtime stage
FROM alpine:3.18

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

COPY --from=builder /api /app/api
COPY --from=builder /worker /app/worker
COPY --from=builder /scheduler /app/scheduler
COPY config.yaml /app/config.yaml

# Default to API server
CMD ["/app/api"]
```

The multi-stage build keeps the final image small (~20MB). All three binaries are included - the entrypoint determines which runs.

## Step 8: Docker Compose (Development)

Create `docker-compose.yml` for local development:

```yaml
version: "3.8"

services:
  api:
    build: .
    command: ["/app/api"]
    ports:
      - "8080:8080"
    environment:
      - DATABASE_ADDRESS=dragonfly:6379
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    depends_on:
      - dragonfly

  worker:
    build: .
    command: ["/app/worker"]
    environment:
      - DATABASE_ADDRESS=dragonfly:6379
      - WORKER_CONCURRENCY=5
    depends_on:
      - dragonfly
    deploy:
      replicas: 2

  scheduler:
    build: .
    command: ["/app/scheduler"]
    environment:
      - DATABASE_ADDRESS=dragonfly:6379
    depends_on:
      - dragonfly

  dragonfly:
    image: docker.dragonflydb.io/dragonflydb/dragonfly
    ports:
      - "6379:6379"
    volumes:
      - dragonfly_data:/data

volumes:
  dragonfly_data:
```

Start everything with:

```bash
docker-compose up
```

This gives you a complete local environment with API, workers, scheduler, and database.

## Step 9: Docker Swarm (Production)

Create `docker-stack.yml` for production:

```yaml
version: "3.8"

services:
  api:
    image: myapp:latest
    command: ["/app/api"]
    ports:
      - "8080:8080"
    environment:
      - DATABASE_ADDRESS=dragonfly:6379
    deploy:
      replicas: 3
      update_config:
        parallelism: 1
        delay: 10s
      restart_policy:
        condition: on-failure
    networks:
      - goflow

  worker:
    image: myapp:latest
    command: ["/app/worker"]
    environment:
      - DATABASE_ADDRESS=dragonfly:6379
      - WORKER_CONCURRENCY=10
    deploy:
      replicas: 5
      restart_policy:
        condition: on-failure
    networks:
      - goflow

  scheduler:
    image: myapp:latest
    command: ["/app/scheduler"]
    environment:
      - DATABASE_ADDRESS=dragonfly:6379
    deploy:
      replicas: 1  # IMPORTANT: Only one scheduler!
      restart_policy:
        condition: on-failure
    networks:
      - goflow

  dragonfly:
    image: docker.dragonflydb.io/dragonflydb/dragonfly
    volumes:
      - dragonfly_data:/data
    deploy:
      replicas: 1
    networks:
      - goflow

networks:
  goflow:

volumes:
  dragonfly_data:
```

Deploy to Swarm:

```bash
# Initialize swarm (if not already)
docker swarm init

# Deploy
docker stack deploy -c docker-stack.yml myapp

# Scale workers based on load
docker service scale myapp_worker=10
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_ADDRESS` | DragonflyDB/Redis address | localhost:6379 |
| `SERVER_PORT` | API server port | 8080 |
| `WORKER_CONCURRENCY` | Jobs per worker | 10 |
| `OPENAI_API_KEY` | OpenAI API key | - |
| `WEBHOOK_SECRET` | Webhook signature secret | - |

## Monitoring

Add Prometheus metrics:

```go
import "github.com/nuulab/goflow/pkg/metrics"

// In your API server
http.Handle("/metrics", metrics.DefaultMetrics.Handler())
```

Key metrics to watch:
- `goflow_queue_depth` - Jobs waiting
- `goflow_jobs_completed_total` - Throughput
- `goflow_jobs_failed_total` - Error rate
- `goflow_job_duration_seconds` - Processing time

## Health Checks

Add health endpoints to your API:

```go
http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
})

http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
    // Check database connection
    if err := db.Ping(r.Context()); err != nil {
        w.WriteHeader(http.StatusServiceUnavailable)
        return
    }
    w.WriteHeader(http.StatusOK)
})
```

Use these in Docker/Kubernetes for health checks and load balancer routing.

## Next Steps

- [Scaling](/docs/guide/scaling) - Advanced scaling strategies
- [Deployment](/docs/guide/deployment) - Production deployment options
- [Webhooks](/docs/guide/webhooks) - External integrations
