# Installation

## Go Module

Add GoFlow to your Go project:

```bash
go get github.com/goflow/goflow
```

## From Source

Clone and build:

```bash
git clone https://github.com/goflow/goflow
cd goflow
go build ./...
```

## Docker

Run with Docker Compose (includes DragonflyDB):

```bash
docker-compose up
```

Or pull the image:

```bash
docker pull goflow/goflow:latest
docker run -p 8080:8080 goflow/goflow
```

## Docker Swarm (Production)

Deploy to a Swarm cluster:

```bash
docker swarm init
docker stack deploy -c docker-stack.yml goflow
```

Scale workers:

```bash
docker service scale goflow_worker=10
```

## Dependencies

### Required
- **Go 1.21+** - GoFlow uses generics and modern Go features

### Optional
- **DragonflyDB/Redis** - For persistence, queues, and caching
- **OpenAI/Anthropic API key** - For LLM-powered agents

## Configuration

### Environment Variables

```bash
# Server
export GOFLOW_PORT=8080
export GOFLOW_REDIS=localhost:6379

# LLM
export OPENAI_API_KEY=sk-...
# or
export ANTHROPIC_API_KEY=sk-ant-...
```

### Config File

Create `goflow.yaml`:

```yaml
server:
  port: 8080

cache:
  address: localhost:6379

agent:
  max_iterations: 10
  default_timeout: 5m
```

## Verifying Installation

```bash
# Check Go build
go build ./...

# Run tests
go test ./...

# Start the server
go run ./cmd/server -port 8080

# Check health
curl http://localhost:8080/health
```

## CLI Installation

Build the CLI:

```bash
go build -o goflow ./cmd/cli
./goflow --version
```

Or install globally:

```bash
go install github.com/goflow/goflow/cmd/cli@latest
```
