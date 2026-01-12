# JavaScript SDK

TypeScript/JavaScript client for GoFlow.

## Installation

```bash
npm install @goflow/client
```

## Usage

```typescript
import { GoFlowClient } from '@goflow/client'

const client = new GoFlowClient('http://localhost:8080')

// Run an agent
const result = await client.agent('my-agent').run('What is 2+2?')
console.log(result.output)
```

## GoFlowClient

```typescript
class GoFlowClient {
  constructor(baseUrl: string, options?: ClientOptions)
  
  // Controllers
  agent(name: string): AgentController
  workflow(id: string): WorkflowController
  get queue(): QueueController
  
  // Connection
  connect(): void
  disconnect(): void
  on(event: string, callback: (data: any) => void): void
}
```

## AgentController

```typescript
interface AgentController {
  // Run an agent task and get the full result
  run(task: string): Promise<RunResult>
  
  // Stream agent output token by token
  runStream(task: string): AsyncGenerator<StreamEvent>
}

interface RunResult {
  output: string
  steps: Step[]
  tool_calls: ToolCall[]
  duration: number
}

interface StreamEvent {
  type: 'token' | 'step' | 'tool_start' | 'tool_end' | 'done' | 'error'
  content?: string
  step?: Step
  toolCall?: ToolCall
  error?: string
}
```

## WorkflowController

Manage workflow execution.

```typescript
interface WorkflowController {
  start(input: any): Promise<{ id: string }>
  status(): Promise<WorkflowStatus>
  pause(): Promise<void>
  resume(): Promise<void>
  signal(signalName: string, data: any): Promise<void>
}
```

## QueueController

Manage background job queues.

```typescript
interface QueueController {
  enqueue(jobName: string, data: any, options?: JobOptions): Promise<Job>
  get(jobId: string): Promise<Job>
  list(options?: { status?: string, limit?: number }): Promise<Job[]>
  retry(jobId: string): Promise<void>
}
```

## Events

```typescript
client.on('job.completed', (event) => {
  console.log('Job completed:', event.job_id)
})

client.on('workflow.step', (event) => {
  console.log('Workflow stepped:', event.state)
})
```

