# @goflow/client

JavaScript/TypeScript SDK for GoFlow AI orchestration framework.

## Installation

### npm
```bash
npm install @goflow/client
```

### bun
```bash
bun add @goflow/client
```

### pnpm
```bash
pnpm add @goflow/client
```

### yarn
```bash
yarn add @goflow/client
```

## Quick Start

```typescript
import { GoFlowClient } from '@goflow/client';

// Create a client
const client = new GoFlowClient('http://localhost:8080');

// Connect to WebSocket for real-time updates
await client.connect();

// Run an AI agent
const agent = client.agent('my-agent');
const result = await agent.run('What is the weather in NYC?');
console.log(result.output);

// Stream agent output
for await (const chunk of agent.runStream('Write a poem about AI')) {
  process.stdout.write(chunk);
}

// Subscribe to agent events
agent.onStep((event) => {
  console.log(`Action: ${event.action}`);
  console.log(`Observation: ${event.observation}`);
});

// Disconnect when done
client.disconnect();
```

## API Reference

### GoFlowClient

```typescript
const client = new GoFlowClient({
  baseUrl: 'http://localhost:8080',
  wsUrl: 'ws://localhost:8080/ws',  // Optional
  autoConnect: true,                 // Auto-connect WebSocket
  reconnect: true,                   // Auto-reconnect on disconnect
  reconnectInterval: 3000,           // Reconnect delay (ms)
});
```

#### Methods

- `connect()` - Connect to WebSocket server
- `disconnect()` - Disconnect from WebSocket server
- `agent(id)` - Get an agent controller
- `workflow(id)` - Get a workflow controller
- `queue()` - Get the queue controller
- `settings()` - Get the settings controller
- `channel(name)` - Get a channel controller
- `listAgents()` - List all agents
- `createAgent(id?)` - Create a new agent

### AgentController

```typescript
const agent = client.agent('my-agent');

// Run a task
const result = await agent.run('Your task here', timeout?);

// Stream output
for await (const chunk of agent.runStream('Your task here')) {
  console.log(chunk);
}

// Agent status
const status = await agent.status();

// Control
await agent.stop();
await agent.reset();
await agent.delete();

// Event subscriptions
agent.onStep((e) => console.log(e.action, e.observation));
agent.onToolCall((e) => console.log(e.tool, e.input));
agent.onComplete((e) => console.log(e.output, e.iterations));
```

### WorkflowController

```typescript
const workflow = client.workflow('my-workflow');

// Start workflow
const result = await workflow.start({ input: 'data' });

// Workflow status
const status = await workflow.status();

// Control
await workflow.pause();
await workflow.resume();
await workflow.signal('signal-name', { data: 'value' });
```

### QueueController

```typescript
const queue = client.queue();

// Enqueue a job
const job = await queue.enqueue('job-type', { data: 'value' }, {
  delay: 5000,    // ms delay
  priority: 10,   // higher = sooner
});

// Get job status
const status = await queue.get(job.id);

// Retry a failed job
await queue.retry(job.id);

// List jobs
const jobs = await queue.list('pending');
```

### SettingsController

```typescript
const settings = client.settings();

// Get settings
const current = await settings.get();

// Update settings
await settings.update({
  max_iterations: 20,
  verbose_logging: true,
});
```

### ChannelController

```typescript
const channel = client.channel('my-channel');

// Subscribe to a topic
const unsubscribe = channel.subscribe('topic', (data) => {
  console.log('Received:', data);
});

// Publish to a topic
channel.publish('topic', { message: 'Hello!' });

// Unsubscribe
unsubscribe();
```

## Events

```typescript
// Global events
client.on('connected', () => console.log('Connected'));
client.on('disconnected', () => console.log('Disconnected'));
client.on('error', (err) => console.error(err));

// Agent-specific events
client.on('agent:step', (event) => console.log(event));
client.on('agent:complete', (event) => console.log(event));
```

## TypeScript Support

Full TypeScript support with exported types:

```typescript
import type {
  GoFlowConfig,
  AgentInfo,
  RunResult,
  StreamEvent,
  Job,
  Workflow,
  StepEvent,
  Settings,
} from '@goflow/client';
```

## Browser Support

The SDK works in both Node.js and browser environments. In browsers, it uses the native `WebSocket` API.

```html
<script type="module">
  import { GoFlowClient } from 'https://esm.sh/@goflow/client'; // GitHub: https://github.com/nuulab/goflow
  
  const client = new GoFlowClient('http://localhost:8080');
  // ...
</script>
```

## License

MIT © Nuulab
