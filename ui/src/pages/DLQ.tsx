import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Modal } from '@/components/ui/modal'
import { AlertTriangle, Eye, RotateCcw, Trash2 } from 'lucide-react'
import { useState } from 'react'

interface DLQEntry {
  id: string
  type: string
  error: string
  attempts: number
  failedAt: string
  payload: Record<string, unknown>
}

const mockDLQ: DLQEntry[] = [
  { 
    id: 'job-123', 
    type: 'send_email', 
    error: 'SMTP connection timeout after 30s',
    attempts: 3,
    failedAt: '30 min ago',
    payload: { to: 'user@example.com', subject: 'Welcome!' }
  },
  { 
    id: 'job-456', 
    type: 'webhook', 
    error: '404 Not Found: https://api.example.com/callback',
    attempts: 3,
    failedAt: '1 hour ago',
    payload: { url: 'https://api.example.com/callback', method: 'POST' }
  },
]

export default function DLQ() {
  const [entries, setEntries] = useState(mockDLQ)
  const [selectedEntry, setSelectedEntry] = useState<DLQEntry | null>(null)

  const handleRetry = (id: string) => {
    setEntries(entries.filter(e => e.id !== id))
  }

  const handleRetryAll = () => {
    setEntries([])
  }

  const handlePurge = () => {
    if (confirm('Permanently delete all DLQ entries?')) {
      setEntries([])
    }
  }

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-medium">Dead Letter Queue</h1>
          <p className="text-muted-foreground text-xs">Jobs that failed after max retries</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={handleRetryAll} disabled={entries.length === 0}>
            <RotateCcw className="w-3 h-3 mr-1.5" />
            Retry All
          </Button>
          <Button variant="destructive" size="sm" onClick={handlePurge} disabled={entries.length === 0}>
            <Trash2 className="w-3 h-3 mr-1.5" />
            Purge
          </Button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-3 gap-3">
        <Card className="bg-muted/30">
          <CardContent className="p-3">
            <div className="text-2xl font-semibold text-red-400">{entries.length}</div>
            <div className="text-[10px] text-muted-foreground">Failed Jobs</div>
          </CardContent>
        </Card>
        <Card className="bg-muted/30">
          <CardContent className="p-3">
            <div className="text-2xl font-semibold">3</div>
            <div className="text-[10px] text-muted-foreground">Max Retries</div>
          </CardContent>
        </Card>
        <Card className="bg-muted/30">
          <CardContent className="p-3">
            <div className="text-2xl font-semibold">1h</div>
            <div className="text-[10px] text-muted-foreground">Oldest</div>
          </CardContent>
        </Card>
      </div>

      {/* DLQ Entries */}
      {entries.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <AlertTriangle className="w-8 h-8 mx-auto mb-2 text-muted-foreground/30" />
            <p className="text-xs text-muted-foreground">No failed jobs</p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-2">
          {entries.map((entry) => (
            <Card key={entry.id}>
              <CardContent className="p-4">
                <div className="flex items-start justify-between">
                  <div className="flex items-start gap-3">
                    <AlertTriangle className="w-4 h-4 text-red-400 mt-0.5" />
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-mono">{entry.id}</span>
                        <Badge variant="outline">{entry.type}</Badge>
                      </div>
                      <p className="text-xs text-red-400 mt-1">{entry.error}</p>
                      <p className="text-[10px] text-muted-foreground mt-1">
                        {entry.attempts} attempts • Failed {entry.failedAt}
                      </p>
                    </div>
                  </div>
                  <div className="flex gap-1">
                    <Button variant="ghost" size="sm" onClick={() => setSelectedEntry(entry)}>
                      <Eye className="w-3 h-3" />
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => handleRetry(entry.id)}>
                      <RotateCcw className="w-3 h-3" />
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Detail Modal */}
      <Modal open={!!selectedEntry} onClose={() => setSelectedEntry(null)} title="Failed Job Details">
        {selectedEntry && (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-3 text-xs">
              <div>
                <span className="text-muted-foreground">ID</span>
                <p className="font-mono">{selectedEntry.id}</p>
              </div>
              <div>
                <span className="text-muted-foreground">Type</span>
                <p>{selectedEntry.type}</p>
              </div>
              <div>
                <span className="text-muted-foreground">Attempts</span>
                <p>{selectedEntry.attempts}</p>
              </div>
              <div>
                <span className="text-muted-foreground">Failed</span>
                <p>{selectedEntry.failedAt}</p>
              </div>
            </div>
            
            <div>
              <span className="text-xs text-muted-foreground">Error</span>
              <p className="mt-1 p-3 bg-red-500/10 border border-red-500/20 rounded text-xs text-red-400">
                {selectedEntry.error}
              </p>
            </div>
            
            <div>
              <span className="text-xs text-muted-foreground">Payload</span>
              <pre className="mt-1 p-3 bg-muted rounded text-xs overflow-auto">
                {JSON.stringify(selectedEntry.payload, null, 2)}
              </pre>
            </div>
          </div>
        )}
      </Modal>
    </div>
  )
}
