import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Modal } from '@/components/ui/modal'
import type { Job } from '@/lib/api'
import { Eye, RefreshCw, RotateCcw, Search } from 'lucide-react'
import { useState } from 'react'

// Mock data - replace with API calls when backend is running
const mockJobs: Job[] = [
  { id: 'job-1736445000001', type: 'send_email', status: 'completed', priority: 0, attempts: 1, created_at: '2025-01-09T13:00:00Z', max_retries: 3, payload: { to: 'user@example.com', subject: 'Welcome!' }, metadata: {} },
  { id: 'job-1736445000002', type: 'process_data', status: 'running', priority: 1, attempts: 1, created_at: '2025-01-09T12:55:00Z', max_retries: 3, payload: { file_id: 'abc123' }, metadata: {} },
  { id: 'job-1736445000003', type: 'webhook', status: 'completed', priority: 0, attempts: 1, created_at: '2025-01-09T12:48:00Z', max_retries: 3, payload: { url: 'https://api.example.com/hook' }, metadata: {} },
  { id: 'job-1736445000004', type: 'send_email', status: 'failed', priority: 0, attempts: 3, created_at: '2025-01-09T12:15:00Z', max_retries: 3, payload: { to: 'bad@example.com' }, metadata: { error: 'SMTP timeout' } },
  { id: 'job-1736445000005', type: 'sync_data', status: 'pending', priority: 2, attempts: 0, created_at: '2025-01-09T11:30:00Z', max_retries: 3, payload: { source: 'api' }, metadata: {} },
]

export default function Jobs() {
  const [jobs, setJobs] = useState<Job[]>(mockJobs)
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [selectedJob, setSelectedJob] = useState<Job | null>(null)
  const [loading, setLoading] = useState(false)

  const filteredJobs = jobs.filter(job => {
    if (statusFilter !== 'all' && job.status !== statusFilter) return false
    if (search && !job.id.includes(search) && !job.type.includes(search)) return false
    return true
  })

  const formatTime = (dateStr: string) => {
    const date = new Date(dateStr)
    const now = new Date()
    const diff = now.getTime() - date.getTime()
    const mins = Math.floor(diff / 60000)
    if (mins < 60) return `${mins}m ago`
    const hours = Math.floor(mins / 60)
    if (hours < 24) return `${hours}h ago`
    return `${Math.floor(hours / 24)}d ago`
  }

  const handleRetry = async (jobId: string) => {
    // In real implementation, call retryJob(jobId)
    setJobs(jobs.map(j => j.id === jobId ? { ...j, status: 'pending', attempts: 0 } : j))
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-medium">Jobs</h1>
          <p className="text-muted-foreground text-xs">View and manage queued jobs</p>
        </div>
        <Button variant="outline" size="sm" onClick={() => setLoading(true)}>
          <RefreshCw className={`w-3 h-3 mr-2 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-4">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-3 h-3 text-muted-foreground" />
          <input
            type="text"
            placeholder="Search jobs..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full h-8 pl-8 pr-3 bg-muted border-0 rounded-md text-xs focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>
        
        <div className="flex items-center gap-1">
          {['all', 'pending', 'running', 'completed', 'failed'].map((status) => (
            <button
              key={status}
              onClick={() => setStatusFilter(status)}
              className={`px-2.5 py-1 rounded-md text-xs transition-colors ${
                statusFilter === status
                  ? 'bg-foreground text-background'
                  : 'text-muted-foreground hover:text-foreground hover:bg-muted'
              }`}
            >
              {status.charAt(0).toUpperCase() + status.slice(1)}
            </button>
          ))}
        </div>
      </div>

      {/* Jobs Table */}
      <Card className="overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-border text-left">
                <th className="text-xs font-medium text-muted-foreground px-4 py-2">ID</th>
                <th className="text-xs font-medium text-muted-foreground px-4 py-2">Type</th>
                <th className="text-xs font-medium text-muted-foreground px-4 py-2">Status</th>
                <th className="text-xs font-medium text-muted-foreground px-4 py-2">Priority</th>
                <th className="text-xs font-medium text-muted-foreground px-4 py-2">Attempts</th>
                <th className="text-xs font-medium text-muted-foreground px-4 py-2">Created</th>
                <th className="text-xs font-medium text-muted-foreground px-4 py-2 text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {filteredJobs.map((job) => (
                <tr key={job.id} className="border-b border-border hover:bg-muted/30 transition-colors">
                  <td className="px-4 py-2.5">
                    <span className="font-mono text-xs">{job.id.slice(-12)}</span>
                  </td>
                  <td className="px-4 py-2.5">
                    <span className="text-xs">{job.type}</span>
                  </td>
                  <td className="px-4 py-2.5">
                    <Badge variant={
                      job.status === 'completed' ? 'success' :
                      job.status === 'running' ? 'default' :
                      job.status === 'failed' ? 'destructive' :
                      'warning'
                    }>
                      {job.status}
                    </Badge>
                  </td>
                  <td className="px-4 py-2.5 text-xs text-muted-foreground">{job.priority}</td>
                  <td className="px-4 py-2.5 text-xs text-muted-foreground">{job.attempts}/{job.max_retries}</td>
                  <td className="px-4 py-2.5 text-xs text-muted-foreground">{formatTime(job.created_at)}</td>
                  <td className="px-4 py-2.5 text-right">
                    <div className="flex items-center justify-end gap-1">
                      <Button variant="ghost" size="sm" onClick={() => setSelectedJob(job)}>
                        <Eye className="w-3 h-3" />
                      </Button>
                      {job.status === 'failed' && (
                        <Button variant="ghost" size="sm" onClick={() => handleRetry(job.id)}>
                          <RotateCcw className="w-3 h-3" />
                        </Button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>

      {/* Job Detail Modal */}
      <Modal open={!!selectedJob} onClose={() => setSelectedJob(null)} title="Job Details">
        {selectedJob && (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-3 text-xs">
              <div>
                <span className="text-muted-foreground">ID</span>
                <p className="font-mono">{selectedJob.id}</p>
              </div>
              <div>
                <span className="text-muted-foreground">Type</span>
                <p>{selectedJob.type}</p>
              </div>
              <div>
                <span className="text-muted-foreground">Status</span>
                <p><Badge variant={selectedJob.status === 'completed' ? 'success' : selectedJob.status === 'failed' ? 'destructive' : 'default'}>{selectedJob.status}</Badge></p>
              </div>
              <div>
                <span className="text-muted-foreground">Attempts</span>
                <p>{selectedJob.attempts}/{selectedJob.max_retries}</p>
              </div>
            </div>
            
            <div>
              <span className="text-xs text-muted-foreground">Payload</span>
              <pre className="mt-1 p-3 bg-muted rounded text-xs overflow-auto">
                {JSON.stringify(selectedJob.payload, null, 2)}
              </pre>
            </div>

            {selectedJob.metadata?.error && (
              <div>
                <span className="text-xs text-muted-foreground">Error</span>
                <p className="mt-1 p-3 bg-destructive/10 border border-destructive/20 rounded text-xs text-destructive">
                  {selectedJob.metadata.error}
                </p>
              </div>
            )}
          </div>
        )}
      </Modal>
    </div>
  )
}
