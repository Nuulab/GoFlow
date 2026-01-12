import { beforeEach, describe, expect, it, vi } from 'vitest';

// Mock WebSocket
const MockWebSocket = vi.fn(() => ({
  readyState: 1, // OPEN
  send: vi.fn(),
  close: vi.fn(),
  onopen: null as any,
  onclose: null as any,
  onerror: null as any,
  onmessage: null as any,
}));

// @ts-ignore
global.WebSocket = MockWebSocket;

// Mock fetch
global.fetch = vi.fn();

import GoFlowClient, {
    AgentController,
    ChannelController,
    QueueController,
    SettingsController,
    WorkflowController,
} from '../index';

describe('GoFlowClient', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Configuration', () => {
    it('should accept string config', () => {
      const client = new GoFlowClient('http://localhost:8080');
      expect(client['config'].baseUrl).toBe('http://localhost:8080');
    });

    it('should accept object config', () => {
      const client = new GoFlowClient({
        baseUrl: 'http://localhost:8080',
        autoConnect: false,
        reconnect: false,
      });
      expect(client['config'].baseUrl).toBe('http://localhost:8080');
      expect(client['config'].autoConnect).toBe(false);
      expect(client['config'].reconnect).toBe(false);
    });

    it('should strip trailing slash from baseUrl', () => {
      const client = new GoFlowClient('http://localhost:8080/');
      expect(client['config'].baseUrl).toBe('http://localhost:8080');
    });

    it('should generate wsUrl from baseUrl', () => {
      const client = new GoFlowClient({
        baseUrl: 'http://localhost:8080',
        autoConnect: false,
      });
      expect(client['config'].wsUrl).toBe('ws://localhost:8080/ws');
    });
  });

  describe('Event Handling', () => {
    it('should register and call event handlers', () => {
      const client = new GoFlowClient({ baseUrl: 'http://localhost:8080', autoConnect: false });
      const handler = vi.fn();
      
      client.on('test', handler);
      client['emit']('test', { data: 'value' });
      
      expect(handler).toHaveBeenCalledWith({ data: 'value' });
    });

    it('should unsubscribe from events', () => {
      const client = new GoFlowClient({ baseUrl: 'http://localhost:8080', autoConnect: false });
      const handler = vi.fn();
      
      const unsubscribe = client.on('test', handler);
      unsubscribe();
      client['emit']('test', { data: 'value' });
      
      expect(handler).not.toHaveBeenCalled();
    });

    it('should handle once() for one-time events', () => {
      const client = new GoFlowClient({ baseUrl: 'http://localhost:8080', autoConnect: false });
      const handler = vi.fn();
      
      client.once('test', handler);
      client['emit']('test', { data: 'first' });
      client['emit']('test', { data: 'second' });
      
      expect(handler).toHaveBeenCalledTimes(1);
      expect(handler).toHaveBeenCalledWith({ data: 'first' });
    });
  });

  describe('Controllers', () => {
    it('should return AgentController', () => {
      const client = new GoFlowClient({ baseUrl: 'http://localhost:8080', autoConnect: false });
      const agent = client.agent('test-agent');
      expect(agent).toBeInstanceOf(AgentController);
      expect(agent.id).toBe('test-agent');
    });

    it('should return WorkflowController', () => {
      const client = new GoFlowClient({ baseUrl: 'http://localhost:8080', autoConnect: false });
      const workflow = client.workflow('test-workflow');
      expect(workflow).toBeInstanceOf(WorkflowController);
      expect(workflow.id).toBe('test-workflow');
    });

    it('should return QueueController', () => {
      const client = new GoFlowClient({ baseUrl: 'http://localhost:8080', autoConnect: false });
      const queue = client.queue;
      expect(queue).toBeInstanceOf(QueueController);
    });

    it('should return SettingsController', () => {
      const client = new GoFlowClient({ baseUrl: 'http://localhost:8080', autoConnect: false });
      const settings = client.settings;
      expect(settings).toBeInstanceOf(SettingsController);
    });

    it('should return ChannelController', () => {
      const client = new GoFlowClient({ baseUrl: 'http://localhost:8080', autoConnect: false });
      const channel = client.channel('test-channel');
      expect(channel).toBeInstanceOf(ChannelController);
      expect(channel.name).toBe('test-channel');
    });
  });
});

describe('AgentController', () => {
  let client: GoFlowClient;

  beforeEach(() => {
    vi.clearAllMocks();
    client = new GoFlowClient({ baseUrl: 'http://localhost:8080', autoConnect: false });
  });

  it('should run a task', async () => {
    const mockResponse = { output: 'Hello!', iterations: 2, success: true };
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    });

    const agent = client.agent('agent-1');
    const result = await agent.run('Say hello');

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/agents/agent-1/run',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ task: 'Say hello', timeout: undefined }),
      })
    );
    expect(result).toEqual(mockResponse);
  });

  it('should get agent status', async () => {
    const mockStatus = { id: 'agent-1', status: 'idle', created_at: '2024-01-01' };
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockStatus),
    });

    const agent = client.agent('agent-1');
    const status = await agent.status();

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/agents/agent-1',
      expect.any(Object)
    );
    expect(status).toEqual(mockStatus);
  });

  it('should stop agent', async () => {
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({}),
    });

    const agent = client.agent('agent-1');
    await agent.stop();

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/agents/agent-1/stop',
      expect.objectContaining({ method: 'POST' })
    );
  });

  it('should reset agent', async () => {
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({}),
    });

    const agent = client.agent('agent-1');
    await agent.reset();

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/agents/agent-1/reset',
      expect.objectContaining({ method: 'POST' })
    );
  });

  it('should delete agent', async () => {
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({}),
    });

    const agent = client.agent('agent-1');
    await agent.delete();

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/agents/agent-1',
      expect.objectContaining({ method: 'DELETE' })
    );
  });
});

describe('WorkflowController', () => {
  let client: GoFlowClient;

  beforeEach(() => {
    vi.clearAllMocks();
    client = new GoFlowClient({ baseUrl: 'http://localhost:8080', autoConnect: false });
  });

  it('should start a workflow', async () => {
    const mockResponse = { id: 'wf-1', status: 'running' };
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    });

    const workflow = client.workflow('order-processing');
    const result = await workflow.start({ orderId: '123' });

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/workflows/order-processing/start',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ input: { orderId: '123' } }),
      })
    );
    expect(result).toEqual(mockResponse);
  });

  it('should pause a workflow', async () => {
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({}),
    });

    const workflow = client.workflow('wf-1');
    await workflow.pause();

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/workflows/wf-1/pause',
      expect.objectContaining({ method: 'POST' })
    );
  });

  it('should resume a workflow', async () => {
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({}),
    });

    const workflow = client.workflow('wf-1');
    await workflow.resume();

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/workflows/wf-1/resume',
      expect.objectContaining({ method: 'POST' })
    );
  });

  it('should send a signal to workflow', async () => {
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({}),
    });

    const workflow = client.workflow('wf-1');
    await workflow.signal('payment_received', { amount: 100 });

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/workflows/wf-1/signal',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ signal: 'payment_received', data: { amount: 100 } }),
      })
    );
  });
});

describe('QueueController', () => {
  let client: GoFlowClient;

  beforeEach(() => {
    vi.clearAllMocks();
    client = new GoFlowClient({ baseUrl: 'http://localhost:8080', autoConnect: false });
  });

  it('should enqueue a job', async () => {
    const mockJob = { id: 'job-1', status: 'pending' };
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockJob),
    });

    const job = await client.queue.enqueue('email', { to: 'user@example.com' });

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/queue/enqueue',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ name: 'email', data: { to: 'user@example.com' } }),
      })
    );
    expect(job).toEqual(mockJob);
  });

  it('should get a job', async () => {
    const mockJob = { id: 'job-1', status: 'completed', result: { sent: true } };
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockJob),
    });

    const job = await client.queue.get('job-1');

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/queue/jobs/job-1',
      expect.any(Object)
    );
    expect(job).toEqual(mockJob);
  });

  it('should list jobs', async () => {
    const mockJobs = [
      { id: 'job-1', status: 'completed' },
      { id: 'job-2', status: 'pending' },
    ];
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockJobs),
    });

    const jobs = await client.queue.list();

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/queue/jobs',
      expect.any(Object)
    );
    expect(jobs).toEqual(mockJobs);
  });

  it('should retry a job', async () => {
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({}),
    });

    await client.queue.retry('job-1');

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/queue/jobs/job-1/retry',
      expect.objectContaining({ method: 'POST' })
    );
  });
});

describe('SettingsController', () => {
  let client: GoFlowClient;

  beforeEach(() => {
    vi.clearAllMocks();
    client = new GoFlowClient({ baseUrl: 'http://localhost:8080', autoConnect: false });
  });

  it('should get settings', async () => {
    const mockSettings = {
      max_iterations: 10,
      default_timeout: 300,
      verbose_logging: false,
      allowed_origins: ['*'],
    };
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockSettings),
    });

    const settings = await client.settings.get();

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/settings',
      expect.any(Object)
    );
    expect(settings).toEqual(mockSettings);
  });

  it('should update settings', async () => {
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({}),
    });

    await client.settings.update({ max_iterations: 20 });

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/settings',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify({ max_iterations: 20 }),
      })
    );
  });
});

describe('Error Handling', () => {
  let client: GoFlowClient;

  beforeEach(() => {
    vi.clearAllMocks();
    client = new GoFlowClient({ baseUrl: 'http://localhost:8080', autoConnect: false });
  });

  it('should throw error on failed request', async () => {
    (global.fetch as any).mockResolvedValueOnce({
      ok: false,
      status: 404,
      json: () => Promise.resolve({ error: 'Agent not found' }),
    });

    const agent = client.agent('nonexistent');
    
    await expect(agent.status()).rejects.toThrow('Agent not found');
  });

  it('should handle network errors', async () => {
    (global.fetch as any).mockRejectedValueOnce(new Error('Network error'));

    const agent = client.agent('agent-1');
    
    await expect(agent.status()).rejects.toThrow('Network error');
  });
});
