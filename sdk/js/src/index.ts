/**
 * GoFlow JavaScript SDK
 * Real-time communication with GoFlow AI agents
 */

// Types
export interface GoFlowConfig {
  baseUrl: string;
  wsUrl?: string;
  autoConnect?: boolean;
  reconnect?: boolean;
  reconnectInterval?: number;
}

export interface AgentInfo {
  id: string;
  status: 'idle' | 'running' | 'stopped';
  created_at: string;
  last_run_at?: string;
}

export interface RunResult {
  output: string;
  iterations: number;
  success: boolean;
  error?: string;
}

export interface StreamEvent {
  type: string;
  content?: string;
  error?: string;
}

export interface Job {
  id: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  result?: any;
  error?: string;
  created_at: string;
}

export interface Workflow {
  id: string;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'paused';
  result?: any;
}

export interface StepEvent {
  action: string;
  observation: string;
  is_final: boolean;
}

export interface Settings {
  max_iterations: number;
  default_timeout: number;
  verbose_logging: boolean;
  allowed_origins: string[];
}

export interface WSEvent {
  type: string;
  agent_id?: string;
  data?: any;
  timestamp: string;
}

type EventHandler<T = any> = (data: T) => void;

/**
 * GoFlow Client - Main SDK entry point
 */
export class GoFlowClient {
  private config: Required<GoFlowConfig>;
  private ws: WebSocket | null = null;
  private eventHandlers: Map<string, Set<EventHandler>> = new Map();
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private connected = false;

  constructor(config: string | GoFlowConfig) {
    const baseConfig = typeof config === 'string' ? { baseUrl: config } : config;
    
    this.config = {
      baseUrl: baseConfig.baseUrl.replace(/\/$/, ''),
      wsUrl: baseConfig.wsUrl || baseConfig.baseUrl.replace(/^http/, 'ws') + '/ws',
      autoConnect: baseConfig.autoConnect ?? true,
      reconnect: baseConfig.reconnect ?? true,
      reconnectInterval: baseConfig.reconnectInterval ?? 3000,
    };

    if (this.config.autoConnect) {
      this.connect();
    }
  }

  // ===== Connection Management =====

  /**
   * Connect to GoFlow WebSocket server
   */
  async connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        resolve();
        return;
      }

      this.ws = new WebSocket(this.config.wsUrl);

      this.ws.onopen = () => {
        this.connected = true;
        this.emit('connected', {});
        resolve();
      };

      this.ws.onclose = () => {
        this.connected = false;
        this.emit('disconnected', {});
        
        if (this.config.reconnect) {
          this.scheduleReconnect();
        }
      };

      this.ws.onerror = (error) => {
        this.emit('error', error);
        reject(error);
      };

      this.ws.onmessage = (event) => {
        try {
          const data: WSEvent = JSON.parse(event.data);
          this.handleEvent(data);
        } catch (e) {
          console.error('Failed to parse WebSocket message:', e);
        }
      };
    });
  }

  /**
   * Disconnect from WebSocket server
   */
  disconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    
    this.config.reconnect = false;
    this.ws?.close();
    this.ws = null;
    this.connected = false;
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer) return;
    
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect().catch(() => {});
    }, this.config.reconnectInterval);
  }

  private handleEvent(event: WSEvent): void {
    // Emit to specific event type listeners
    this.emit(event.type, event);

    // Emit to agent-specific listeners
    if (event.agent_id) {
      this.emit(`agent:${event.agent_id}`, event);
    }

    // Emit to wildcard listeners
    this.emit('*', event);
  }

  // ===== Event Handling =====

  /**
   * Subscribe to events
   */
  on<T = any>(event: string, handler: EventHandler<T>): () => void {
    if (!this.eventHandlers.has(event)) {
      this.eventHandlers.set(event, new Set());
    }
    this.eventHandlers.get(event)!.add(handler);

    // Return unsubscribe function
    return () => this.off(event, handler);
  }

  /**
   * Unsubscribe from events
   */
  off(event: string, handler: EventHandler): void {
    this.eventHandlers.get(event)?.delete(handler);
  }

  /**
   * Subscribe to events (one-time)
   */
  once<T = any>(event: string, handler: EventHandler<T>): void {
    const wrapper: EventHandler<T> = (data) => {
      this.off(event, wrapper);
      handler(data);
    };
    this.on(event, wrapper);
  }

  private emit(event: string, data: any): void {
    this.eventHandlers.get(event)?.forEach(handler => {
      try {
        handler(data);
      } catch (e) {
        console.error('Event handler error:', e);
      }
    });
  }

  // ===== WebSocket Commands =====

  /**
   * Subscribe to server-side topics
   */
  subscribe(...topics: string[]): void {
    this.send({ type: 'subscribe', payload: { topics } });
  }

  /**
   * Unsubscribe from server-side topics
   */
  unsubscribe(...topics: string[]): void {
    this.send({ type: 'unsubscribe', payload: { topics } });
  }

  /**
   * Publish to a channel
   */
  publish(channel: string, topic: string, data: any): void {
    this.send({ type: 'publish', payload: { channel, topic, data } });
  }

  private send(message: any): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message));
    }
  }

  // ===== HTTP API =====

  private async fetch<T>(path: string, options: RequestInit = {}): Promise<T> {
    const response = await fetch(`${this.config.baseUrl}${path}`, {
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
      ...options,
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: 'Unknown error' }));
      throw new Error(error.error || `HTTP ${response.status}`);
    }

    return response.json();
  }

  // ===== Agent Operations =====

  /**
   * Get an agent controller
   */
  agent(id: string): AgentController {
    return new AgentController(this, id);
  }

  /**
   * List all agents
   */
  async listAgents(): Promise<AgentInfo[]> {
    const result = await this.fetch<{ agents: AgentInfo[] }>('/api/agents');
    return result.agents;
  }

  /**
   * Create a new agent
   */
  async createAgent(id?: string): Promise<AgentInfo> {
    return this.fetch<AgentInfo>('/api/agents', {
      method: 'POST',
      body: JSON.stringify({ id }),
    });
  }

  // ===== Settings =====

  /**
   * Get settings controller
   */
  get settings(): SettingsController {
    return new SettingsController(this);
  }

  // ===== Channel Operations =====

  /**
   * Get a channel controller
   */
  channel(name: string): ChannelController {
    return new ChannelController(this, name);
  }

  // ===== Workflow Operations =====

  /**
   * Get a workflow controller
   */
  workflow(id: string): WorkflowController {
    return new WorkflowController(this, id);
  }

  // ===== Queue Operations =====

  /**
   * Get queue controller
   */
  get queue(): QueueController {
    return new QueueController(this);
  }

  // ===== Internal API for controllers =====

  /** @internal */
  _fetch<T>(path: string, options?: RequestInit): Promise<T> {
    return this.fetch(path, options);
  }

  /** @internal */
  _on<T>(event: string, handler: EventHandler<T>): () => void {
    return this.on(event, handler);
  }

  /** @internal */
  _send(message: any): void {
    return this.send(message);
  }

  /** @internal */
  get isConnected(): boolean {
    return this.connected;
  }
}

/**
 * Agent Controller - Control a specific agent
 */
export class AgentController {
  constructor(
    private client: GoFlowClient,
    public readonly id: string
  ) {}

  /**
   * Run a task on this agent
   */
  async run(task: string, timeout?: number): Promise<RunResult> {
    return this.client._fetch<RunResult>(`/api/agents/${this.id}/run`, {
      method: 'POST',
      body: JSON.stringify({ task, timeout }),
    });
  }

  /**
   * Get agent status
   */
  async status(): Promise<AgentInfo> {
    return this.client._fetch<AgentInfo>(`/api/agents/${this.id}`);
  }

  /**
   * Stop the agent
   */
  async stop(): Promise<void> {
    await this.client._fetch(`/api/agents/${this.id}/stop`, { method: 'POST' });
  }

  /**
   * Reset agent state
   */
  async reset(): Promise<void> {
    await this.client._fetch(`/api/agents/${this.id}/reset`, { method: 'POST' });
  }

  /**
   * Delete the agent
   */
  async delete(): Promise<void> {
    await this.client._fetch(`/api/agents/${this.id}`, { method: 'DELETE' });
  }

  /**
   * Subscribe to agent events
   */
  onStep(handler: EventHandler<StepEvent>): () => void {
    this.client.subscribe(`agent:${this.id}`);
    return this.client._on(`agent:${this.id}`, (event: WSEvent) => {
      if (event.type === 'agent.step') {
        handler(event.data as StepEvent);
      }
    });
  }

  /**
   * Subscribe to tool calls
   */
  onToolCall(handler: EventHandler<{ tool: string; input: string }>): () => void {
    this.client.subscribe(`agent:${this.id}`);
    return this.client._on(`agent:${this.id}`, (event: WSEvent) => {
      if (event.type === 'agent.tool_call') {
        handler(event.data);
      }
    });
  }

  /**
   * Subscribe to completion
   */
  onComplete(handler: EventHandler<{ output: string; iterations: number }>): () => void {
    this.client.subscribe(`agent:${this.id}`);
    return this.client._on(`agent:${this.id}`, (event: WSEvent) => {
      if (event.type === 'agent.completed') {
        handler(event.data);
      }
    });
  }

  /**
   * Run agent and stream output
   */
  async *runStream(task: string): AsyncGenerator<string, void, unknown> {
    const response = await fetch(`${this.client['_fetch']('/api/agents/' + this.id + '/stream', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ task }),
    })}`);
    
    // Note: Actual streaming implementation requires SSE or readable streams override
    // For now we simulate with the run method
    const result = await this.run(task);
    yield result.output;
  }
}

/**
 * Workflow Controller - Manage workflows
 */
export class WorkflowController {
  constructor(
    private client: GoFlowClient,
    public readonly id: string
  ) {}

  async start(input: any): Promise<Workflow> {
    return this.client._fetch<Workflow>(`/api/workflows/${this.id}/start`, {
      method: 'POST',
      body: JSON.stringify({ input }),
    });
  }

  async status(): Promise<Workflow> {
    return this.client._fetch<Workflow>(`/api/workflows/${this.id}`);
  }

  async pause(): Promise<void> {
    await this.client._fetch(`/api/workflows/${this.id}/pause`, { method: 'POST' });
  }

  async resume(): Promise<void> {
    await this.client._fetch(`/api/workflows/${this.id}/resume`, { method: 'POST' });
  }

  async signal(signal: string, data: any): Promise<void> {
    await this.client._fetch(`/api/workflows/${this.id}/signal`, {
      method: 'POST',
      body: JSON.stringify({ signal, data }),
    });
  }
}

/**
 * Queue Controller - Manage job queues
 */
export class QueueController {
  constructor(private client: GoFlowClient) {}

  async enqueue(name: string, data: any, options: { delay?: number; priority?: number } = {}): Promise<Job> {
    return this.client._fetch<Job>('/api/queue/enqueue', {
      method: 'POST',
      body: JSON.stringify({ name, data, ...options }),
    });
  }

  async get(jobId: string): Promise<Job> {
    return this.client._fetch<Job>(`/api/queue/jobs/${jobId}`);
  }

  async retry(jobId: string): Promise<void> {
    await this.client._fetch(`/api/queue/jobs/${jobId}/retry`, { method: 'POST' });
  }

  async list(status?: string): Promise<Job[]> {
    const query = status ? `?status=${status}` : '';
    return this.client._fetch<Job[]>(`/api/queue/jobs${query}`);
  }
}

/**
 * Settings Controller - Manage server settings
 */
export class SettingsController {
  constructor(private client: GoFlowClient) {}

  /**
   * Get current settings
   */
  async get(): Promise<Settings> {
    return this.client._fetch<Settings>('/api/settings');
  }

  /**
   * Update settings
   */
  async update(settings: Partial<Settings>): Promise<void> {
    await this.client._fetch('/api/settings', {
      method: 'PUT',
      body: JSON.stringify(settings),
    });
  }
}

/**
 * Channel Controller - Real-time pub/sub channels
 */
export class ChannelController {
  private handlers: Map<string, Set<EventHandler>> = new Map();

  constructor(
    private client: GoFlowClient,
    public readonly name: string
  ) {
    // Listen for channel messages
    this.client._on('channel.message', (event: WSEvent) => {
      if (event.data?.channel === this.name) {
        this.handlers.get(event.data.topic)?.forEach(h => h(event.data.data));
        this.handlers.get('*')?.forEach(h => h(event.data));
      }
    });
  }

  /**
   * Subscribe to a topic on this channel
   */
  subscribe<T = any>(topic: string, handler: EventHandler<T>): () => void {
    if (!this.handlers.has(topic)) {
      this.handlers.set(topic, new Set());
    }
    this.handlers.get(topic)!.add(handler);

    return () => {
      this.handlers.get(topic)?.delete(handler);
    };
  }

  /**
   * Publish to this channel
   */
  publish(topic: string, data: any): void {
    this.client._send({
      type: 'publish',
      payload: { channel: this.name, topic, data },
    });
  }
}

// Default export
export default GoFlowClient;
