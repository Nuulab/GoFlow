# Queue API

The `queue` package provides distributed job queues.

## Queue Interface

```go
type Queue interface {
    Enqueue(ctx context.Context, job *Job) error
    Dequeue(ctx context.Context, timeout time.Duration) (*Job, error)
    Peek(ctx context.Context) (*Job, error)
    Len(ctx context.Context) (int64, error)
    Close() error
}

func NewDragonflyQueue(cfg Config) (Queue, error)
```

## Job

```go
type Job struct {
    ID         string
    Type       string
    Payload    json.RawMessage
    Priority   int
    CreatedAt  time.Time
    Attempts   int
    MaxRetries int
    Metadata   map[string]string
}

func NewJob[T any](jobType string, payload T) (*Job, error)
func (j *Job) UnmarshalPayload(v any) error
func (j *Job) WithPriority(p int) *Job
func (j *Job) WithMaxRetries(n int) *Job
func (j *Job) WithMetadata(key, value string) *Job
```

## Config

```go
type Config struct {
    Address    string
    Password   string
    Database   int
    QueueName  string
    MaxRetries int
}
```

## Worker

```go
type Worker struct{}
type Handler func(ctx context.Context, job *Job) error

func NewWorker(queue Queue) *Worker
func (w *Worker) Handle(jobType string, handler Handler)
func (w *Worker) Start(ctx context.Context, concurrency int)
func (w *Worker) Stop()
```

## ShardedQueue

```go
type ShardedQueue struct{}

type ShardedConfig struct {
    Shards   []Config
    Strategy ShardStrategy
}

type ShardStrategy int
const (
    HashShard ShardStrategy = iota
    RoundRobinShard
    LeastLoadedShard
)

func NewShardedQueue(cfg ShardedConfig) (*ShardedQueue, error)
func (sq *ShardedQueue) NumShards() int
func (sq *ShardedQueue) LenPerShard(ctx context.Context) ([]int64, error)
```

## DistributedLock

```go
type DistributedLock struct{}
type Lock struct{}

func NewDistributedLock(client *redis.Client) *DistributedLock
func (dl *DistributedLock) Acquire(ctx context.Context, key string, ttl time.Duration) (*Lock, error)
func (dl *DistributedLock) TryAcquire(ctx context.Context, key string, ttl, maxWait time.Duration) (*Lock, error)
func (dl *DistributedLock) WithLock(ctx context.Context, key string, ttl time.Duration, fn func() error) error

func (l *Lock) Release(ctx context.Context) error
func (l *Lock) Extend(ctx context.Context, ttl time.Duration) error
func (l *Lock) IsHeld(ctx context.Context) (bool, error)
```

## EventStore

```go
type EventStore struct{}
type Event struct {
    ID        string
    Type      EventType
    JobID     string
    JobType   string
    Timestamp time.Time
    Data      map[string]any
    WorkerID  string
    Error     string
    Duration  time.Duration
}

type EventType string
const (
    EventJobCreated   EventType = "job.created"
    EventJobQueued    EventType = "job.queued"
    EventJobStarted   EventType = "job.started"
    EventJobCompleted EventType = "job.completed"
    EventJobFailed    EventType = "job.failed"
    EventJobRetried   EventType = "job.retried"
    EventJobDLQ       EventType = "job.dlq"
)

func NewEventStore(client *redis.Client) *EventStore
func (es *EventStore) Append(ctx context.Context, event Event) error
func (es *EventStore) GetJobEvents(ctx context.Context, jobID string) ([]Event, error)
func (es *EventStore) GetRecentEvents(ctx context.Context, count int64) ([]Event, error)
func (es *EventStore) Subscribe(ctx context.Context, handler func(Event)) error
```

## DLQ

```go
type DLQ struct{}
type DLQEntry struct {
    Job        *Job
    Error      string
    FailedAt   time.Time
    Attempts   int
    WorkerID   string
    StackTrace string
}

type Alerter interface {
    Alert(ctx context.Context, entry DLQEntry) error
}

func NewDLQ(client *redis.Client, name string) *DLQ
func (d *DLQ) AddAlerter(alerter Alerter)
func (d *DLQ) Add(ctx context.Context, job *Job, err error, workerID string) error
func (d *DLQ) Get(ctx context.Context, start, stop int64) ([]DLQEntry, error)
func (d *DLQ) Len(ctx context.Context) (int64, error)
func (d *DLQ) Retry(ctx context.Context, queue Queue, index int64) error
func (d *DLQ) RetryAll(ctx context.Context, queue Queue) (int, error)
func (d *DLQ) Purge(ctx context.Context) error

func NewWebhookAlerter(url string) *WebhookAlerter
func NewSlackAlerter(webhookURL, channel string) *SlackAlerter
func NewLogAlerter(logger func(string, ...any)) *LogAlerter
```

## Semaphore

```go
type Semaphore struct{}

func NewSemaphore(client *redis.Client, key string, limit int) *Semaphore
func (s *Semaphore) Acquire(ctx context.Context, ttl time.Duration) (string, error)
func (s *Semaphore) Release(ctx context.Context, id string) error
func (s *Semaphore) Available(ctx context.Context) (int, error)
```
