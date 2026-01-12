# CLI

The GoFlow CLI for development and operations.

## Installation

```bash
go install github.com/goflow/goflow/cmd/cli@latest
```

## Commands

### Services

```bash
# Start API server
goflow server -p 8080

# Start worker
goflow worker -c 10

# Start scheduler
goflow scheduler
```

### Jobs

```bash
# List jobs
goflow job list
goflow job list -s pending

# Enqueue a job
goflow job enqueue -t send_email -d '{"to":"user@example.com"}'

# Get job status
goflow job status job-123

# Retry a failed job
goflow job retry job-123
```

### Workflows

```bash
# List workflows
goflow workflow list

# Start a workflow
goflow workflow start order-process -i '{"order_id":123}'

# Check status
goflow workflow status wf-123

# Control
goflow workflow pause wf-123
goflow workflow resume wf-123
```

### Agent

```bash
# Run a task
goflow agent "What is 2+2?"

# Interactive mode
goflow agent -i

# With specific tools
goflow agent -t calculator,web "Search and calculate"
```

### Queue Operations

```bash
# Queue stats
goflow queue stats

# View events
goflow events
goflow events -f          # Follow
goflow events -j job-123  # Filter by job
```

### Dead Letter Queue

```bash
# List DLQ
goflow dlq list

# Retry
goflow dlq retry job-123
goflow dlq retry-all

# Purge
goflow dlq purge
```

## Global Flags

```bash
--config   Config file (default: ./goflow.yaml)
--redis    Redis address (default: localhost:6379)
-v         Verbose output
```

## Configuration

Create `goflow.yaml`:

```yaml
server:
  port: 8080

redis: localhost:6379

agent:
  max_iterations: 10
```
