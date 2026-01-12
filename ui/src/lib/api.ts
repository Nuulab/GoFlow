// API client for GoFlow backend
const API_BASE = '/api'

export interface Job {
  id: string
  type: string
  payload: Record<string, unknown>
  priority: number
  created_at: string
  attempts: number
  max_retries: number
  metadata: Record<string, string>
  status?: 'pending' | 'running' | 'completed' | 'failed'
  duration?: number
}

export interface Workflow {
  id: string
  name: string
  status: 'running' | 'completed' | 'paused' | 'failed'
  current_state: string
  states: string[]
  started_at: string
  completed_at?: string
  input: Record<string, unknown>
  error?: string
}

export interface Event {
  id: string
  type: string
  job_id: string
  job_type: string
  timestamp: string
  data?: Record<string, unknown>
  worker_id?: string
  error?: string
  duration?: number
}

export interface DLQEntry {
  job: Job
  error: string
  failed_at: string
  attempts: number
  worker_id?: string
}

export interface Stats {
  pending: number
  running: number
  completed: number
  failed: number
  dlq_size: number
  workers_active: number
  throughput: number
}

// API functions
export async function fetchStats(): Promise<Stats> {
  const res = await fetch(`${API_BASE}/stats`)
  if (!res.ok) throw new Error('Failed to fetch stats')
  return res.json()
}

export async function fetchJobs(status?: string): Promise<Job[]> {
  const url = status ? `${API_BASE}/jobs?status=${status}` : `${API_BASE}/jobs`
  const res = await fetch(url)
  if (!res.ok) throw new Error('Failed to fetch jobs')
  return res.json()
}

export async function fetchJob(id: string): Promise<Job> {
  const res = await fetch(`${API_BASE}/jobs/${id}`)
  if (!res.ok) throw new Error('Failed to fetch job')
  return res.json()
}

export async function retryJob(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/jobs/${id}/retry`, { method: 'POST' })
  if (!res.ok) throw new Error('Failed to retry job')
}

export async function fetchWorkflows(): Promise<Workflow[]> {
  const res = await fetch(`${API_BASE}/workflows`)
  if (!res.ok) throw new Error('Failed to fetch workflows')
  return res.json()
}

export async function fetchWorkflow(id: string): Promise<Workflow> {
  const res = await fetch(`${API_BASE}/workflows/${id}`)
  if (!res.ok) throw new Error('Failed to fetch workflow')
  return res.json()
}

export async function pauseWorkflow(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/workflows/${id}/pause`, { method: 'POST' })
  if (!res.ok) throw new Error('Failed to pause workflow')
}

export async function resumeWorkflow(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/workflows/${id}/resume`, { method: 'POST' })
  if (!res.ok) throw new Error('Failed to resume workflow')
}

export async function fetchEvents(limit = 50): Promise<Event[]> {
  const res = await fetch(`${API_BASE}/events?limit=${limit}`)
  if (!res.ok) throw new Error('Failed to fetch events')
  return res.json()
}

export async function fetchDLQ(): Promise<DLQEntry[]> {
  const res = await fetch(`${API_BASE}/dlq`)
  if (!res.ok) throw new Error('Failed to fetch DLQ')
  return res.json()
}

export async function retryDLQEntry(jobId: string): Promise<void> {
  const res = await fetch(`${API_BASE}/dlq/${jobId}/retry`, { method: 'POST' })
  if (!res.ok) throw new Error('Failed to retry DLQ entry')
}

export async function purgeDLQ(): Promise<void> {
  const res = await fetch(`${API_BASE}/dlq`, { method: 'DELETE' })
  if (!res.ok) throw new Error('Failed to purge DLQ')
}

// WebSocket for real-time events
export function connectWebSocket(onEvent: (event: Event) => void): WebSocket {
  const ws = new WebSocket(`ws://${window.location.host}/ws`)
  
  ws.onopen = () => {
    ws.send(JSON.stringify({ type: 'subscribe', topic: 'events' }))
  }
  
  ws.onmessage = (msg) => {
    try {
      const data = JSON.parse(msg.data)
      if (data.type === 'event') {
        onEvent(data.payload)
      }
    } catch (e) {
      console.error('WebSocket parse error:', e)
    }
  }
  
  return ws
}
