---
title: Initialize swarm
---

title: Scaling
---


Scale GoFlow to handle millions of requests.

## Architecture for Scale

```
┌─────────────┐     ┌─────────────────────────────┐
│  Load       │────▶│  API Servers (N replicas)   │
│  Balancer   │     └─────────────────────────────┘
└─────────────┘                   │
                    ┌─────────────┴─────────────┐
                    ▼                           ▼
          ┌─────────────────┐         ┌─────────────────┐
          │  DragonflyDB    │         │  Workers (N)    │
          │  (Sharded)      │         │  (Scale these)  │
          └─────────────────┘         └─────────────────┘
```

## Docker Swarm Deployment

```bash
# Initialize swarm
docker swarm init

# Deploy stack
docker stack deploy -c docker-stack.yml goflow

# Scale workers
docker service scale goflow_worker=20
```

## Queue Sharding

Distribute load across multiple queue instances:

```go
queue := queue.NewShardedQueue(queue.ShardedConfig{
    Shards: []queue.Config{
        {Address: "redis1:6379"},
        {Address: "redis2:6379"},
        {Address: "redis3:6379"},
        {Address: "redis4:6379"},
    },
    Strategy: queue.LeastLoadedShard,
})
```

Sharding strategies:
- `HashShard` - Consistent hashing on job ID
- `RoundRobinShard` - Even distribution
- `LeastLoadedShard` - Route to least busy shard

## Partitioned Workers

Workers claim specific partitions:

```go
// Worker 1 handles shards 0-1
worker1 := queue.NewPartitionedWorker(shardedQueue, 0)
worker1.Start(ctx, 5)

// Worker 2 handles shards 2-3
worker2 := queue.NewPartitionedWorker(shardedQueue, 2)
worker2.Start(ctx, 5)
```

## Distributed Locking

Prevent duplicate processing:

```go
locker := queue.NewDistributedLock(redisClient)

lock, err := locker.TryAcquire(ctx, "job:"+jobID, 
    30*time.Second,  // TTL
    5*time.Second,   // Max wait
)
if err != nil {
    return // Lock held by another worker
}
defer lock.Release(ctx)
```

## Semaphores for Rate Limiting

Control concurrency:

```go
sem := queue.NewSemaphore(redisClient, "api_calls", 100)

slot, err := sem.Acquire(ctx, 30*time.Second)
if err != nil {
    return errors.New("rate limited")
}
defer sem.Release(ctx, slot)

callExternalAPI()
```

## Metrics & Monitoring

```go
import "github.com/nuulab/goflow/pkg/metrics"

// Expose Prometheus endpoint
http.Handle("/metrics", metrics.DefaultMetrics.Handler())

// Custom metrics
metrics.JobCompleted()
metrics.JobFailed()
metrics.ObserveJobDuration(start)
metrics.SetQueueDepth(depth)
```

Key metrics:
- `goflow_jobs_completed_total`
- `goflow_jobs_failed_total`
- `goflow_queue_depth`
- `goflow_job_duration_seconds`
- `goflow_workers_active`

## Event Sourcing

Track all events for debugging:

```go
store := queue.NewEventStore(redisClient)

// All events are automatically recorded
// Query job history
events, _ := store.GetJobEvents(ctx, jobID)

// Real-time streaming
store.Subscribe(ctx, func(e queue.Event) {
    log.Printf("%s: %s", e.Type, e.JobID)
})
```

## Alerting

Get notified on failures:

```go
dlq := queue.NewDLQ(redisClient, "main")

// Slack alerts
dlq.AddAlerter(queue.NewSlackAlerter(webhookURL, "#alerts"))

// Webhook alerts
dlq.AddAlerter(queue.NewWebhookAlerter("https://api.pagerduty.com/..."))

// Custom alerts
dlq.AddAlerter(&queue.CallbackAlerter{
    Callback: func(entry queue.DLQEntry) {
        sendEmail("Job failed: " + entry.Job.ID)
    },
})
```

## Performance Tips

1. **Use sharding** - Single Redis is the bottleneck
2. **Batch operations** - Process jobs in batches when possible
3. **Tune concurrency** - Match worker count to workload
4. **Monitor queue depth** - Alert if it grows too fast
5. **Use DLQ** - Don't retry forever, move to DLQ
6. **Event sourcing** - Debug without guessing
