---
title: CLI Reference
description: The GoFlow command-line interface for development and operations.
icon: Terminal
---

import { Callout } from 'fumadocs-ui/components/callout';
import { Tabs, Tab } from 'fumadocs-ui/components/tabs';

<Callout type="info">
  **Binary Name:** `goflow`  
  **Install:** `go install github.com/nuulab/goflow/cmd/cli@latest`
</Callout>

## Installation

```bash title="Install the CLI globally"
go install github.com/nuulab/goflow/cmd/cli@latest
```

## Service Commands

Start GoFlow services for development or production.

<Tabs items={['API Server', 'Worker', 'Scheduler']}>
  <Tab value="API Server">
```bash title="Start the API server"
# Start on default port 8080
goflow server

# Custom port
goflow server -p 3000

# With verbose logging
goflow server -p 8080 -v
```
  </Tab>
  <Tab value="Worker">
```bash title="Start a job worker"
# Start with 10 concurrent workers
goflow worker -c 10

# Custom queue name
goflow worker -q high-priority -c 5
```
  </Tab>
  <Tab value="Scheduler">
```bash title="Start the scheduler"
# Start the cron scheduler
goflow scheduler
```
  </Tab>
</Tabs>

## Agent Commands

Run AI agents from the command line.

```bash title="Run an agent task"
# Single task
goflow agent "What is the weather in Tokyo?"

# Interactive chat mode
goflow agent -i

# With specific tools enabled
goflow agent -t calculator,web_search "Calculate 25% of 400"

# Custom model
goflow agent --provider anthropic --model claude-3-opus "Write a poem"
```

### Agent Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-i, --interactive` | Interactive chat mode | `false` |
| `-t, --tools` | Comma-separated tool list | all |
| `--provider` | LLM provider | `openai` |
| `--model` | Model name | provider default |
| `-m, --max-iterations` | Max reasoning steps | `10` |

## Job Commands

Manage background jobs in the queue.

```bash title="Job operations"
# List all jobs
goflow job list

# Filter by status
goflow job list -s pending
goflow job list -s failed

# Enqueue a new job
goflow job enqueue -t send_email -d '{"to":"user@example.com"}'

# Get job details
goflow job status job-abc123

# Retry a failed job
goflow job retry job-abc123
```

## Workflow Commands

Control workflow executions.

```bash title="Workflow operations"
# List all workflows
goflow workflow list

# Start a workflow with input
goflow workflow start order-process -i '{"order_id":123}'

# Check workflow status
goflow workflow status wf-abc123

# Pause a running workflow
goflow workflow pause wf-abc123

# Resume a paused workflow
goflow workflow resume wf-abc123

# Send a signal
goflow workflow signal wf-abc123 approval '{"approved":true}'
```

## Queue Commands

Monitor queue statistics and events.

```bash title="Queue monitoring"
# Show queue statistics
goflow queue stats

# Watch events in real-time
goflow events -f

# Filter events by job
goflow events -j job-abc123
```

## Dead Letter Queue

Manage failed jobs in the DLQ.

```bash title="DLQ operations"
# List DLQ entries
goflow dlq list

# Retry a specific job
goflow dlq retry job-abc123

# Retry all DLQ jobs
goflow dlq retry-all

# Purge the DLQ
goflow dlq purge
```

## Global Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--config` | Config file path | `./goflow.yaml` |
| `--redis` | Redis address | `localhost:6379` |
| `-v, --verbose` | Verbose output | `false` |

## Configuration File

Create a `goflow.yaml` file for persistent configuration:

```yaml title="goflow.yaml"
# API Server
server:
  port: 8080
  host: "0.0.0.0"

# Redis/DragonflyDB
redis: localhost:6379

# Agent defaults
agent:
  max_iterations: 10
  default_provider: openai

# LLM Configuration
llm:
  provider: openai
  model: gpt-4o
  api_key: ${OPENAI_API_KEY}  # Use env var
```
