import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Activity, ArrowUpRight, CheckCircle, Clock, Users, XCircle, Zap } from 'lucide-react'

const stats = [
  { label: 'Active Jobs', value: '42', icon: Activity, change: '+12%', color: 'text-primary' },
  { label: 'Completed', value: '1.2k', icon: CheckCircle, change: '+5%', color: 'text-green-500' },
  { label: 'Failed', value: '12', icon: XCircle, change: '-2%', color: 'text-red-500' },
  { label: 'Avg Latency', value: '245ms', icon: Clock, change: '~', color: 'text-yellow-500' },
  { label: 'Active Workers', value: '10', icon: Users, change: '+2', color: 'text-indigo-500' },
  { label: 'Throughput', value: '52/s', icon: Zap, change: '+8%', color: 'text-blue-500' },
]

const recentJobs = [
  { id: 'job-001', type: 'send_email', status: 'completed', duration: '123ms', time: '2s ago' },
  { id: 'job-002', type: 'process_data', status: 'running', duration: '-', time: '5s ago' },
  { id: 'job-003', type: 'webhook', status: 'completed', duration: '89ms', time: '12s ago' },
  { id: 'job-004', type: 'send_email', status: 'failed', duration: '2.3s', time: '45s ago' },
  { id: 'job-005', type: 'sync_data', status: 'completed', duration: '1.2s', time: '1m ago' },
]

const recentWorkflows = [
  { id: 'wf-001', name: 'order-process', status: 'running', state: 'validate', progress: 40 },
  { id: 'wf-002', name: 'data-pipeline', status: 'completed', state: 'done', progress: 100 },
  { id: 'wf-003', name: 'sync-workflow', status: 'paused', state: 'approval', progress: 60 },
]

export default function Dashboard() {
  return (
    <div className="p-8 space-y-8 animate-in fade-in duration-500">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Dashboard</h1>
          <p className="text-muted-foreground mt-1">Real-time system overview</p>
        </div>
        <div className="flex items-center gap-2">
            <span className="relative flex h-3 w-3">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span>
              <span className="relative inline-flex rounded-full h-3 w-3 bg-green-500"></span>
            </span>
            <span className="text-sm font-medium text-muted-foreground">System Operational</span>
        </div>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
        {stats.map((stat, i) => (
          <Card key={stat.label} className="group hover:border-sidebar-primary/50 transition-colors" style={{ animationDelay: `${i * 50}ms` }}>
            <CardContent className="p-4">
              <div className="flex justify-between items-start mb-2">
                <div className={`p-2 rounded-lg bg-background/50 ring-1 ring-inset ring-border group-hover:bg-background transition-colors`}>
                    <stat.icon className={`w-4 h-4 ${stat.color}`} />
                </div>
                {stat.change && (
                    <span className="text-[10px] bg-green-500/10 text-green-600 px-1.5 py-0.5 rounded-full font-medium">
                        {stat.change}
                    </span>
                )}
              </div>
              <div className="text-2xl font-bold tracking-tight">{stat.value}</div>
              <div className="text-xs text-muted-foreground mt-1 font-medium">{stat.label}</div>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Recent Activity */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Recent Jobs */}
        <Card className="h-full">
          <CardHeader className="py-4 px-6 border-b border-border/40 bg-muted/20">
            <div className="flex items-center justify-between">
              <div className="flex gap-2 items-center">
                <ListTodo className="w-4 h-4 text-muted-foreground" />
                <CardTitle>Recent Jobs</CardTitle>
              </div>
              <a href="/jobs" className="text-xs font-medium text-primary hover:text-primary/80 flex items-center gap-1 transition-colors">
                View all <ArrowUpRight className="w-3 h-3" />
              </a>
            </div>
          </CardHeader>
          <CardContent className="p-0">
            <div className="divide-y divide-border/40">
              {recentJobs.map((job) => (
                <div key={job.id} className="flex items-center justify-between px-6 py-3.5 hover:bg-muted/30 transition-colors group">
                  <div className="flex items-center gap-3">
                    <StatusDot status={job.status} />
                    <div>
                      <div className="text-sm font-medium group-hover:text-primary transition-colors">{job.type}</div>
                      <div className="text-xs text-muted-foreground font-mono mt-0.5">{job.id}</div>
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="text-xs font-medium">{job.duration}</div>
                    <div className="text-[10px] text-muted-foreground mt-0.5">{job.time}</div>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        {/* Recent Workflows */}
        <Card className="h-full">
          <CardHeader className="py-4 px-6 border-b border-border/40 bg-muted/20">
            <div className="flex items-center justify-between">
              <div className="flex gap-2 items-center">
                <GitBranch className="w-4 h-4 text-muted-foreground" />
                <CardTitle>Active Workflows</CardTitle>
              </div>
              <a href="/workflows" className="text-xs font-medium text-primary hover:text-primary/80 flex items-center gap-1 transition-colors">
                View all <ArrowUpRight className="w-3 h-3" />
              </a>
            </div>
          </CardHeader>
          <CardContent className="p-0">
            <div className="divide-y divide-border/40">
              {recentWorkflows.map((wf) => (
                <div key={wf.id} className="px-6 py-4 hover:bg-muted/30 transition-colors">
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2.5">
                      <span className="text-sm font-medium">{wf.name}</span>
                      <Badge variant={
                        wf.status === 'completed' ? 'success' :
                        wf.status === 'running' ? 'default' :
                        'warning'
                      } className="uppercase text-[10px] py-0 px-1.5 h-4">
                        {wf.status}
                      </Badge>
                    </div>
                    <span className="text-xs text-muted-foreground font-mono">{wf.state}</span>
                  </div>
                  <div className="h-1.5 bg-muted rounded-full overflow-hidden w-full">
                    <div 
                      className={`h-full transition-all duration-500 rounded-full ${
                        wf.status === 'completed' ? 'bg-green-500' :
                        wf.status === 'paused' ? 'bg-yellow-500' : 
                        'bg-primary'
                      }`}
                      style={{ width: `${wf.progress}%` }}
                    />
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Quick Terminal/Logs Snippet (Visual Flair) */}
      <Card className="bg-black/90 border-sidebar-border shadow-2xl overflow-hidden group">
        <CardHeader className="py-2.5 px-4 border-b border-white/10 bg-white/5 flex flex-row items-center gap-2">
            <div className="flex gap-1.5">
                <div className="w-2.5 h-2.5 rounded-full bg-red-500/80"></div>
                <div className="w-2.5 h-2.5 rounded-full bg-yellow-500/80"></div>
                <div className="w-2.5 h-2.5 rounded-full bg-green-500/80"></div>
            </div>
            <div className="ml-2 text-[10px] font-mono text-white/40">system.log</div>
        </CardHeader>
        <CardContent className="p-4 font-mono text-xs text-green-400/90 h-32 overflow-hidden relative">
            <div className="absolute inset-0 bg-gradient-to-t from-black/90 to-transparent pointer-events-none" />
            <div className="space-y-1 opacity-80">
                <div>[INFO] System initialized. Ready to process.</div>
                <div>[INFO] Connected to Redis at localhost:6379</div>
                <div>[INFO] Worker-01 started listening on queue: default</div>
                <div className="text-blue-400">[DEBUG] Handshake completed in 3ms</div>
                <div>[INFO] Workflow engine online</div>
                <div>[WARN] High memory usage detected (mock)</div>
                <div>[INFO] Processing job-002...</div>
            </div>
        </CardContent>
      </Card>
    </div>
  )
}

function StatusDot({ status }: { status: string }) {
  const colors = {
    completed: 'bg-green-500 shadow-[0_0_8px_rgba(34,197,94,0.4)]',
    running: 'bg-blue-500 shadow-[0_0_8px_rgba(59,130,246,0.4)] animate-pulse',
    failed: 'bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.4)]',
    pending: 'bg-yellow-500 shadow-[0_0_8px_rgba(234,179,8,0.4)]',
  }
  return (
    <div className={`w-2 h-2 rounded-full ring-2 ring-background transition-all ${colors[status as keyof typeof colors] || 'bg-gray-400'}`} />
  )
}

import { GitBranch, ListTodo } from 'lucide-react'

