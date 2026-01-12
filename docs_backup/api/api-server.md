# API Server

The `api` package provides REST and WebSocket endpoints.

## Server

```go
type Server struct{}

func NewServer(cfg ServerConfig) *Server
func (s *Server) Start(addr string) error
func (s *Server) Shutdown(ctx context.Context) error
func (s *Server) RegisterAgent(name string, agent *agent.Agent)
```

## ServerConfig

```go
type ServerConfig struct {
    Port         int
    AllowOrigins []string
    Agent        *agent.Agent
    Queue        queue.Queue
    Cache        cache.Cache
}
```

## REST Endpoints

### Agents
```
GET    /api/agents           List registered agents
POST   /api/agents/:name/run Run an agent with a task
GET    /api/agents/:name     Get agent info
```

### Settings
```
GET    /api/settings         Get server settings
PUT    /api/settings         Update settings
```

### Jobs
```
GET    /api/jobs             List jobs
POST   /api/jobs             Create a job
GET    /api/jobs/:id         Get job details
POST   /api/jobs/:id/retry   Retry a job
```

### Workflows
```
GET    /api/workflows        List workflows
POST   /api/workflows/:name  Start a workflow
GET    /api/workflows/:id    Get workflow status
POST   /api/workflows/:id/pause   Pause
POST   /api/workflows/:id/resume  Resume
POST   /api/workflows/:id/signal  Send signal
```

### Events
```
GET    /api/events           Get recent events
```

### DLQ
```
GET    /api/dlq              List DLQ entries
POST   /api/dlq/:id/retry    Retry entry
DELETE /api/dlq              Purge DLQ
```

### Health
```
GET    /health               Health check
GET    /ready                Ready check
```

## WebSocket

Connect to `/ws` for real-time events:

```json
// Subscribe to events
{"type": "subscribe", "topic": "events"}

// Receive events
{"type": "event", "payload": {"type": "job.completed", "job_id": "..."}}
```

## WebSocket Hub

```go
type Hub struct{}

func NewHub() *Hub
func (h *Hub) Register(conn *Conn)
func (h *Hub) Unregister(conn *Conn)
func (h *Hub) Broadcast(topic string, message any)
func (h *Hub) Run()
```
