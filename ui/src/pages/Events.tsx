import { Button } from '@/components/ui/button'
import { Card, CardHeader } from '@/components/ui/card'
import { Pause, Play } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

const mockEvents = [
  { id: 1, time: '13:14:55', type: 'job.completed', jobId: 'job-001', details: 'duration=123ms' },
  { id: 2, time: '13:14:54', type: 'job.started', jobId: 'job-002', details: 'worker=worker-3' },
  { id: 3, time: '13:14:52', type: 'job.queued', jobId: 'job-003', details: 'type=send_email' },
  { id: 4, time: '13:14:50', type: 'job.failed', jobId: 'job-004', details: 'error=timeout' },
  { id: 5, time: '13:14:48', type: 'job.retried', jobId: 'job-004', details: 'attempt=2' },
]

const typeColors: Record<string, string> = {
  'job.completed': 'text-green-400',
  'job.started': 'text-blue-400',
  'job.queued': 'text-yellow-400',
  'job.failed': 'text-red-400',
  'job.retried': 'text-orange-400',
  'workflow.started': 'text-purple-400',
  'workflow.step': 'text-cyan-400',
}

export default function Events() {
  const [isPaused, setIsPaused] = useState(false)
  const [events, setEvents] = useState(mockEvents)
  const [filter, setFilter] = useState<string>('all')
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (isPaused) return

    const interval = setInterval(() => {
      const types = ['job.completed', 'job.started', 'job.queued', 'job.failed']
      const newEvent = {
        id: Date.now(),
        time: new Date().toLocaleTimeString('en-US', { hour12: false }),
        type: types[Math.floor(Math.random() * types.length)],
        jobId: `job-${Math.floor(Math.random() * 1000).toString().padStart(3, '0')}`,
        details: `worker=worker-${Math.floor(Math.random() * 5) + 1}`,
      }
      setEvents(prev => [newEvent, ...prev.slice(0, 99)])
    }, 1500)

    return () => clearInterval(interval)
  }, [isPaused])

  const filteredEvents = events.filter(e => 
    filter === 'all' || e.type.startsWith(filter)
  )

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-medium">Events</h1>
          <p className="text-muted-foreground text-xs">Real-time event stream</p>
        </div>
        <Button 
          variant="outline" 
          size="sm"
          onClick={() => setIsPaused(!isPaused)}
        >
          {isPaused ? <Play className="w-3 h-3 mr-1.5" /> : <Pause className="w-3 h-3 mr-1.5" />}
          {isPaused ? 'Resume' : 'Pause'}
        </Button>
      </div>

      <div className="flex items-center gap-1">
        {['all', 'job', 'workflow'].map((f) => (
          <button
            key={f}
            onClick={() => setFilter(f)}
            className={`px-2.5 py-1 rounded-md text-xs transition-colors ${
              filter === f
                ? 'bg-foreground text-background'
                : 'text-muted-foreground hover:text-foreground hover:bg-muted'
            }`}
          >
            {f === 'all' ? 'All' : f.charAt(0).toUpperCase() + f.slice(1) + 's'}
          </button>
        ))}
      </div>

      <Card>
        <CardHeader className="py-2 px-4 border-b border-border">
          <div className="flex items-center gap-2">
            <div className={`w-1.5 h-1.5 rounded-full ${isPaused ? 'bg-yellow-400' : 'bg-green-400 animate-pulse'}`} />
            <span className="text-xs text-muted-foreground">{isPaused ? 'Paused' : 'Live'}</span>
          </div>
        </CardHeader>
        <div ref={containerRef} className="h-[calc(100vh-220px)] overflow-auto font-mono text-[11px]">
          {filteredEvents.map((event, i) => (
            <div 
              key={event.id}
              className={`flex items-center gap-3 px-4 py-1.5 border-b border-border/50 hover:bg-muted/30 ${
                i === 0 && !isPaused ? 'bg-muted/20' : ''
              }`}
            >
              <span className="text-muted-foreground w-16">{event.time}</span>
              <span className={`w-24 ${typeColors[event.type] || 'text-foreground'}`}>
                {event.type}
              </span>
              <span className="text-foreground w-16">{event.jobId}</span>
              <span className="text-muted-foreground flex-1">{event.details}</span>
            </div>
          ))}
        </div>
      </Card>
    </div>
  )
}
