// GoFlow Scheduler - Runs cron jobs in Swarm mode
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nuulab/goflow/pkg/queue"
	"github.com/nuulab/goflow/pkg/workflow"
)

func main() {
	// Flags
	redisAddr := flag.String("redis", "localhost:6379", "Redis/DragonflyDB address")
	flag.Parse()

	// Environment overrides
	if envRedis := os.Getenv("GOFLOW_REDIS"); envRedis != "" {
		*redisAddr = envRedis
	}

	// Banner
	fmt.Println("⏰ GoFlow Scheduler")
	fmt.Printf("   Redis: %s\n", *redisAddr)

	// Connect to queue
	q, err := queue.NewDragonflyQueue(queue.Config{
		Address:   *redisAddr,
		QueueName: "goflow:jobs",
	})
	if err != nil {
		log.Fatalf("Failed to connect to queue: %v", err)
	}
	log.Println("✅ Connected to queue")

	// Create workflow engine
	engine := workflow.NewEngine(nil)

	// Create cron scheduler
	cron := workflow.NewCron(engine)

	// Register scheduled jobs from environment or config
	// In production, load from config file or database
	registerScheduledJobs(cron, q)

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("\n🛑 Shutting down scheduler...")
		cancel()
		cron.Stop()
	}()

	// Start scheduler
	log.Println("🚀 Starting scheduler...")
	cron.Start(ctx)

	// Keep running
	<-ctx.Done()
	log.Println("👋 Scheduler stopped")
}

// registerScheduledJobs registers cron jobs
// In production, load from config or database
func registerScheduledJobs(cron *workflow.Cron, q queue.Queue) {
	// Example scheduled jobs
	schedules := []struct {
		ID         string
		Workflow   string
		Expression string
	}{
		// {"daily-report", "generate_report", "@daily"},
		// {"hourly-sync", "sync_data", "@hourly"},
		// {"every-5m", "health_check", "*/5 * * * *"},
	}

	for _, s := range schedules {
		if err := cron.Add(s.ID, s.Workflow, s.Expression, nil); err != nil {
			log.Printf("⚠️  Failed to add schedule %s: %v", s.ID, err)
		} else {
			log.Printf("📅 Scheduled: %s (%s)", s.ID, s.Expression)
		}
	}

	log.Printf("📋 Registered %d scheduled jobs", len(schedules))
}
