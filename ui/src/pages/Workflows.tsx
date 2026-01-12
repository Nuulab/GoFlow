import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Modal } from '@/components/ui/modal'
import type { Workflow } from '@/lib/api'
import { Eye, Pause, Play, RotateCcw } from 'lucide-react'
import { useState } from 'react'

const mockWorkflows: Workflow[] = [
  { 
    id: 'wf-001', 
    name: 'order-process', 
    status: 'running', 
    current_state: 'validate',
    states: ['init', 'validate', 'process', 'ship', 'complete'],
    started_at: '2025-01-09T13:28:00Z',
    input: { order_id: 12345, customer: 'john@example.com' }
  },
  { 
    id: 'wf-002', 
    name: 'data-pipeline', 
    status: 'completed', 
    current_state: 'complete',
    states: ['fetch', 'transform', 'load', 'complete'],
    started_at: '2025-01-09T13:20:00Z',
    completed_at: '2025-01-09T13:22:00Z',
    input: { source: 'api', batch_size: 100 }
  },
  { 
    id: 'wf-003', 
    name: 'sync-workflow', 
    status: 'paused', 
    current_state: 'approval',
    states: ['validate', 'approval', 'execute', 'notify'],
    started_at: '2025-01-09T13:25:00Z',
    input: { sync_id: 'abc123' }
  },
  { 
    id: 'wf-004', 
    name: 'report-generator', 
    status: 'failed', 
    current_state: 'generate',
    states: ['gather', 'generate', 'send'],
    started_at: '2025-01-09T13:15:00Z',
    error: 'Template not found: monthly_report.html',
    input: { type: 'monthly', recipients: ['team@example.com'] }
  },
]

export default function Workflows() {
  const [workflows, setWorkflows] = useState(mockWorkflows)
  const [selectedWorkflow, setSelectedWorkflow] = useState<Workflow | null>(null)

  const getStateIndex = (wf: Workflow) => wf.states.indexOf(wf.current_state)

  const handlePause = (id: string) => {
    setWorkflows(workflows.map(w => w.id === id ? { ...w, status: 'paused' } : w))
  }

  const handleResume = (id: string) => {
    setWorkflows(workflows.map(w => w.id === id ? { ...w, status: 'running' } : w))
  }

  const handleRetry = (id: string) => {
    setWorkflows(workflows.map(w => w.id === id ? { ...w, status: 'running', error: undefined } : w))
  }

  return (
    <div className="p-6 space-y-6">
      <div>
        <h1 className="text-lg font-medium">Workflows</h1>
        <p className="text-muted-foreground text-xs">Monitor and control workflow executions</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {workflows.map((wf) => {
          const currentIdx = getStateIndex(wf)
          
          return (
            <Card key={wf.id}>
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between">
                  <div>
                    <CardTitle className="text-sm flex items-center gap-2">
                      {wf.name}
                      <Badge variant={
                        wf.status === 'completed' ? 'success' :
                        wf.status === 'running' ? 'default' :
                        wf.status === 'failed' ? 'destructive' :
                        'warning'
                      }>
                        {wf.status}
                      </Badge>
                    </CardTitle>
                    <p className="text-[10px] text-muted-foreground mt-0.5 font-mono">{wf.id}</p>
                  </div>
                  <div className="flex gap-0.5">
                    {wf.status === 'running' && (
                      <Button variant="ghost" size="sm" onClick={() => handlePause(wf.id)}>
                        <Pause className="w-3 h-3" />
                      </Button>
                    )}
                    {wf.status === 'paused' && (
                      <Button variant="ghost" size="sm" onClick={() => handleResume(wf.id)}>
                        <Play className="w-3 h-3" />
                      </Button>
                    )}
                    {wf.status === 'failed' && (
                      <Button variant="ghost" size="sm" onClick={() => handleRetry(wf.id)}>
                        <RotateCcw className="w-3 h-3" />
                      </Button>
                    )}
                    <Button variant="ghost" size="sm" onClick={() => setSelectedWorkflow(wf)}>
                      <Eye className="w-3 h-3" />
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="pt-0">
                {/* State visualization */}
                <div className="flex items-center mb-3">
                  {wf.states.map((state, i) => {
                    const isCompleted = i < currentIdx
                    const isCurrent = i === currentIdx
                    const isFailed = wf.status === 'failed' && isCurrent
                    
                    return (
                      <div key={state} className="flex items-center flex-1">
                        <div className="flex flex-col items-center flex-1">
                          <div className={`w-6 h-6 rounded-full flex items-center justify-center text-[10px] font-medium border ${
                            isCompleted ? 'bg-foreground text-background border-foreground' :
                            isFailed ? 'bg-destructive text-destructive-foreground border-destructive' :
                            isCurrent ? 'bg-muted text-foreground border-foreground' :
                            'bg-transparent text-muted-foreground border-muted'
                          }`}>
                            {isCompleted ? '✓' : i + 1}
                          </div>
                          <span className={`text-[9px] mt-1 ${isCurrent ? 'text-foreground' : 'text-muted-foreground'}`}>
                            {state}
                          </span>
                        </div>
                        {i < wf.states.length - 1 && (
                          <div className={`h-px flex-1 mx-1 ${isCompleted ? 'bg-foreground' : 'bg-muted'}`} />
                        )}
                      </div>
                    )
                  })}
                </div>

                {wf.error && (
                  <p className="text-[10px] text-destructive truncate">{wf.error}</p>
                )}
              </CardContent>
            </Card>
          )
        })}
      </div>

      {/* Workflow Detail Modal */}
      <Modal open={!!selectedWorkflow} onClose={() => setSelectedWorkflow(null)} title="Workflow Details">
        {selectedWorkflow && (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-3 text-xs">
              <div>
                <span className="text-muted-foreground">ID</span>
                <p className="font-mono">{selectedWorkflow.id}</p>
              </div>
              <div>
                <span className="text-muted-foreground">Name</span>
                <p>{selectedWorkflow.name}</p>
              </div>
              <div>
                <span className="text-muted-foreground">Status</span>
                <p><Badge variant={selectedWorkflow.status === 'completed' ? 'success' : selectedWorkflow.status === 'failed' ? 'destructive' : 'default'}>{selectedWorkflow.status}</Badge></p>
              </div>
              <div>
                <span className="text-muted-foreground">Current State</span>
                <p>{selectedWorkflow.current_state}</p>
              </div>
            </div>

            <div>
              <span className="text-xs text-muted-foreground">States</span>
              <div className="mt-1 flex flex-wrap gap-1">
                {selectedWorkflow.states.map((state, i) => (
                  <Badge key={state} variant={state === selectedWorkflow.current_state ? 'default' : 'outline'}>
                    {i + 1}. {state}
                  </Badge>
                ))}
              </div>
            </div>
            
            <div>
              <span className="text-xs text-muted-foreground">Input</span>
              <pre className="mt-1 p-3 bg-muted rounded text-xs overflow-auto">
                {JSON.stringify(selectedWorkflow.input, null, 2)}
              </pre>
            </div>

            {selectedWorkflow.error && (
              <div>
                <span className="text-xs text-muted-foreground">Error</span>
                <p className="mt-1 p-3 bg-destructive/10 border border-destructive/20 rounded text-xs text-destructive">
                  {selectedWorkflow.error}
                </p>
              </div>
            )}
          </div>
        )}
      </Modal>
    </div>
  )
}
