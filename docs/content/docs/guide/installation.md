---
title: Installation
description: Comprehensive guide to installing and configuring GoFlow
---

# Installation Guide

GoFlow is designed to be flexible. You can install it as a library in your existing Go application, run it as a standalone server, or deploy it as a microservice in a containerized environment.

## Go Module

The most common way to use GoFlow is as a library within your Go application. This gives you full access to the Agents, Workflows, and Tools SDKs directly in your code.

Add GoFlow to your project:

```bash
go get github.com/nuulab/goflow
```

This downloads the package and adds it to your `go.mod` file. You can now import packages like `github.com/nuulab/goflow/pkg/agent` and `github.com/nuulab/goflow/pkg/workflow` in your code.

---

## From Source

If you want to contribute to GoFlow or run the latest unreleased version, you can build from source. This is also useful if you want to run the provided examples or the standalone server binary manually.

1. **Clone the repository:**
   ```bash
   git clone https://github.com/nuulab/goflow
   cd goflow
   ```

2. **Build the binaries:**
   ```bash
   go build ./...
   ```
   This command compiles all packages in the repository, ensuring you have the necessary dependencies.

---

## Docker

For running the GoFlow API server and Worker nodes without writing Go code, Docker is the easiest method. This is perfect for deploying GoFlow as an orchestration layer for other services.

### Using Docker Compose

The repository includes a `docker-compose.yml` file that sets up the GoFlow server along with a DragonflyDB instance (a high-performance Redis alternative) for the job queue and storage.

```bash
docker-compose up
```

This starts:
- **GoFlow Server**: Accessible at `http://localhost:8080`
- **DragonflyDB**: Accessible at `localhost:6379`

### Using Docker Image

You can also pull the pre-built image directly if you already have a Redis instance running:

```bash
docker pull goflow/goflow:latest
docker run -p 8080:8080 -e GOFLOW_REDIS="host.docker.internal:6379" goflow/goflow
```

---

## Docker Swarm (Production)

For production deployments requiring high availability and horizontal scaling, Docker Swarm or Kubernetes is recommended.

**Initialize Swarm:**
```bash
docker swarm init
```
This initializes the current node as a Swarm manager.

**Deploy the Stack:**
```bash
docker stack deploy -c docker-stack.yml goflow
```
This deploys the services defined in your stack file, creating a scalable service mesh.

**Scale Workers:**
When your workload increases, you can instantly scale the number of worker nodes to handle more concurrent agents and workflows.
```bash
docker service scale goflow_worker=10
```

---

## Dependencies

### Required
- **Go 1.21+**: GoFlow leverages modern Go features like Generics (introduced in 1.18) and the latest standard library improvements. Ensure your environment matches this version.

### Optional
- **DragonflyDB/Redis**: While GoFlow can run agents in-memory, a persistent backend like Redis or DragonflyDB is required for:
  - **Durable Workflows**: Saving state so workflows can resume after restarts.
  - **Job Queues**: Distributing tasks across multiple worker nodes.
  - **Distributed Caching**: Sharing resources between agents.
- **LLM API Keys**: To use AI capabilities, you'll need API keys for providers like OpenAI (`OPENAI_API_KEY`) or Anthropic (`ANTHROPIC_API_KEY`).

---

## Configuration

GoFlow follows the [12-Factor App](https://12factor.net/) methodology, allowing configuration via environment variables or a YAML config file.

### Environment Variables

Environment variables are the preferred method for containerized deployments.

| Variable | Description |
|----------|-------------|
| `GOFLOW_PORT` | The HTTP port for the API server (default: 8080) |
| `GOFLOW_REDIS` | Address of the Redis/DragonflyDB instance |
| `OPENAI_API_KEY` | Key for OpenAI models |
| `ANTHROPIC_API_KEY` | Key for Anthropic Claude models |
| `GOOGLE_API_KEY` | Key for Gemini models |

### Config File

For local development or complex setups, you can use a `goflow.yaml` file in your working directory.

```yaml
server:
  port: 8080 # Port to listen on

cache:
  address: localhost:6379 # Redis connection string

agent:
  max_iterations: 10 # Prevent infinite loops in autonomous agents
  default_timeout: 5m # Global timeout for agent execution
```

---

## Verifying Installation

After installation, verify everything is working correctly:

1. **Check Go build:**
   ```bash
   go build ./...
   ```
   Should complete without errors.

2. **Run tests:**
   ```bash
   go test ./...
   ```
   Runs the comprehensive test suite to ensure stability.

3. **Start the server:**
   ```bash
   go run ./cmd/server -port 8080
   ```
   
4. **Check health endpoint:**
   ```bash
   curl http://localhost:8080/health
   ```
   You should receive a `200 OK` response with status info.

---

## CLI Installation

The GoFlow CLI tool helps you manage workflows, inspect queues, and run agents from the command line.

**Build and install:**
```bash
go install github.com/nuulab/goflow/cmd/cli@latest
```

**Verify CLI:**
```bash
goflow --version
```
This confirms the CLI is in your system PATH and ready to use.
