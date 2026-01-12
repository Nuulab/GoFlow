# Queues

GoFlow provides distributed job queues for async processing.

## Basic Usage

```go
import "github.com/goflow/goflow/pkg/queue"

// Connect to queue
q, _ := queue.NewDragonflyQueue(queue.Config{
    Address:   "localhost:6379",
    QueueName: "my-jobs",
})

// Create a job
job, _ := queue.NewJob("send_email", EmailPayload{
    To:      "user@example.com",
    Subject: "Hello",
})

// Enqueue
q.Enqueue(ctx, job)
```

This creates a connection to DragonflyDB (or Redis), creates a typed job, and adds it to the queue. Jobs are persisted until a worker picks them up.

## Worker

```go
worker := queue.NewWorker(q)

// Register handlers
worker.Handle("send_email", func(ctx context.Context, job *queue.Job) error {
    var payload EmailPayload
    job.UnmarshalPayload(&payload)
    return sendEmail(payload)
})

// Start processing (10 concurrent workers)
worker.Start(ctx, 10)
```

Workers are long-running processes that pull jobs from the queue and execute them. The concurrency parameter (10 here) controls how many jobs run in parallel. Return an error to trigger a retry.

## Job Options

```go
job, _ := queue.NewJob("process", payload)

job.WithPriority(10)              // Higher = processed sooner
job.WithMaxRetries(5)             // Retry on failure
job.WithMetadata("user_id", "123") // Custom metadata
```

Job options let you customize processing behavior:
- **Priority** - High-priority jobs jump ahead in the queue
- **MaxRetries** - Automatic retry on failure with backoff
- **Metadata** - Attach tracking info for debugging

## Queue Sharding

Scale horizontally with sharded queues:

```go
shardedQueue, _ := queue.NewShardedQueue(queue.ShardedConfig{
    Shards: []queue.Config{
        {Address: "redis1:6379", QueueName: "jobs"},
        {Address: "redis2:6379", QueueName: "jobs"},
        {Address: "redis3:6379", QueueName: "jobs"},
    },
    Strategy: queue.LeastLoadedShard, // or HashShard, RoundRobinShard
})
```

Sharding distributes jobs across multiple Redis instances. This prevents any single instance from becoming a bottleneck. The strategy determines how jobs are assigned:
- **HashShard** - Consistent hashing on job ID (same job always goes to same shard)
- **RoundRobinShard** - Even distribution across shards
- **LeastLoadedShard** - Route to the shard with fewest pending jobs

## Distributed Locking

Prevent duplicate job processing:

```go
locker := queue.NewDistributedLock(redisClient)

lock, err := locker.Acquire(ctx, "job:"+jobID, 30*time.Second)
if err != nil {
    return // Another worker has it
}
defer lock.Release(ctx)

// Process safely
```

In distributed systems, multiple workers might try to process the same job. Distributed locks ensure only one worker succeeds. The TTL (30s here) auto-releases the lock if the worker crashes.

## Dead Letter Queue

Handle permanently failed jobs:

```go
dlq := queue.NewDLQ(redisClient, "my-jobs")

// Add alerting
dlq.AddAlerter(queue.NewSlackAlerter(webhookURL, "#alerts"))

// Retry all DLQ entries
dlq.RetryAll(ctx, mainQueue)
```

Jobs that fail after max retries go to the Dead Letter Queue instead of being lost. You can inspect them, fix the issue, and retry. Alerters notify your team immediately when jobs fail permanently.

## Event Sourcing

Track all job events:

```go
store := queue.NewEventStore(redisClient)

// Record events
store.Append(ctx, queue.Event{
    Type:  queue.EventJobCompleted,
    JobID: job.ID,
})

// Query history
events, _ := store.GetJobEvents(ctx, jobID)

// Real-time subscription
store.Subscribe(ctx, func(event queue.Event) {
    log.Println("Event:", event.Type, event.JobID)
})
```

Event sourcing provides a complete audit log of every job's lifecycle. You can see exactly what happened, when, and debug issues even after the fact. The subscription feature enables real-time dashboards.

## Webhook Integration

Trigger jobs from external webhooks:

```go
webhookHandler := webhook.NewWebhookHandler(queue, engine)
webhookHandler.RegisterJobWebhook("/stripe", "handle_stripe_event")

http.Handle("/webhooks/", webhookHandler.Handler())
```

This creates an HTTP endpoint that external services can call. When Stripe sends an event to `/webhooks/stripe`, it automatically becomes a `handle_stripe_event` job in your queue.

## Metrics

```go
import "github.com/goflow/goflow/pkg/metrics"

// Instrument handlers
worker.Handle("process", func(ctx context.Context, job *queue.Job) error {
    start := time.Now()
    defer metrics.ObserveJobDuration(start)
    
    err := process(job)
    if err != nil {
        metrics.JobFailed()
        return err
    }
    metrics.JobCompleted()
    return nil
})

// Expose metrics endpoint
http.Handle("/metrics", metrics.DefaultMetrics.Handler())
```

Metrics are essential for production monitoring. GoFlow provides Prometheus-compatible metrics for queue depth, job duration, success/failure rates, and more. Connect to Grafana for beautiful dashboards.

## Delayed Jobs

```go
import "github.com/goflow/goflow/pkg/queue"

scheduler := queue.NewScheduler(redisClient, mainQueue)

// Schedule for later
scheduler.ScheduleAt(ctx, job, time.Now().Add(1*time.Hour))

// Schedule with delay
scheduler.ScheduleIn(ctx, job, 30*time.Minute)

scheduler.Start(ctx)
```

Delayed jobs let you schedule work for the future. Common uses include reminder emails, subscription renewals, or any time-based business logic. The scheduler checks periodically and enqueues jobs when their time comes.
