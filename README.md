<div align="center">
  <img src="docs/public/goflow-logo.png" alt="GoFlow Logo" width="120" />
  <h1>GoFlow</h1>
</div>

**The High-Performance AI Orchestration Framework for Go.**  
Build intelligent agents, durable workflows, and background jobs with ease.

![GoFlow Dashboard](docs/assets/dashboard-preview.png)

> **Created by Nuulab**  
> Released under the **MIT License** (Open Source)

---

## 🚀 Features

### 🤖 Intelligent Agents
- **Local & Cloud LLMs**: Support for OpenAI, Anthropic, Gemini, and local models.
- **Tool Use**: Agents can use tools to interact with your system and the web.
- **Streaming**: Real-time token-by-token feedback.
- **Memory**: Context-aware conversations.

### 🔄 Durable Workflows
- **Code-First**: Define workflows in pure Go code.
- **Fault Tolerant**: Automatic retries, pause/resume, and state persistence.
- **Signals**: Asynchronously signal running workflows.

### ⚡ Background Jobs
- **High Throughput**: Redis/DragonflyDB-backed job queues.
- **Dead Letter Queue**: Robust error handling and manual retry management.
- **Priority Queues**: Critical tasks run first.

### 🖥️ Modern Dashboard (UI)
- **Real-Time Monitoring**: Live visualization of agents, jobs, and workflows.
- **Dark Mode**: Sleek "2026" aesthetic with glassmorphism.
- **Interactive**: Manage your system directly from the UI.

### 🛠️ Developer CLI
- **`goflow agent`**: Run and chat with agents directly in your terminal.
- **`goflow queue`**: Inspect queues and retry jobs.
- **`goflow workflow`**: Start and manage workflow executions.

---

## 📦 Components

| Package | Description |
|---------|-------------|
| `pkg/agent` | Core agent runtime with LLM and tool integration |
| `pkg/workflow` | Temporal-like durable execution engine |
| `pkg/queue` | Distributed background job processing |
| `pkg/llm` | Providers for OpenAI, Anthropic, Gemini |
| `sdk/js` | TypeScript/JavaScript client SDK |
| `cmd/cli` | The `goflow` developer CLI tool |
| `ui` | React/Tailwind modern dashboard |

---

## ⚡ Quick Start

### 1. Installation

```bash
# Clone the repository
git clone https://github.com/nuulab/goflow
cd goflow

# Install the CLI
go install ./cmd/cli
```

### 2. Run the Stack

```bash
# Start the GoFlow server (API + Worker)
go run ./cmd/server -port 8080

# Start the Dashboard (in a separate terminal)
cd ui && npm run dev
```

### 3. Use the CLI

```bash
# Chat with an agent
goflow agent -i

# check queue stats
goflow queue stats
```

### 4. Use the SDK (JavaScript/TypeScript)

```typescript
import { GoFlowClient } from '@goflow/client'

const client = new GoFlowClient('http://localhost:8080')

// Run an intelligent agent
const result = await client.agent('assistant').run('Analyze my data')

// Start a workflow
await client.workflow('order-process').start({ orderId: 123 })
```

---

## 🏗️ Architecture

GoFlow is built as a modular monolith that can scale into microservices. It uses **Redis** (or DragonflyDB) for high-performance state management and task queues.

- **API Server**: Handles REST and WebSocket requests.
- **Worker Pool**: Executes system jobs and workflow tasks.
- **Scheduler**: Manages timed events and crons.

---

## 📄 License

**MIT License**

Copyright (c) 2026 **Nuulab**

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
