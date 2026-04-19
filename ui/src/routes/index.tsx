import { createFileRoute } from '@tanstack/react-router'
import { useMutation } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '#/lib/api'
import { useSSEStatus } from '#/hooks/useSSEStatus'
import { Badge } from '#/components/ui/badge'
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '#/components/ui/card'

export const Route = createFileRoute('/')({ component: StatusPage })

function StatusPage() {
  const { data, connected, error } = useSSEStatus()

  const pause = useMutation({
    mutationFn: api.pause,
    onSuccess: () => toast.success('Rotation paused'),
    onError: () => toast.error('Failed to pause'),
  })
  const resume = useMutation({
    mutationFn: api.resume,
    onSuccess: () => toast.success('Rotation resumed'),
    onError: () => toast.error('Failed to resume'),
  })
  const skip = useMutation({
    mutationFn: api.skip,
    onSuccess: () => toast.success('Skipped to next mode'),
    onError: () => toast.error('Failed to skip'),
  })
  const unpin = useMutation({
    mutationFn: api.unpin,
    onSuccess: () => toast.success('Unpinned — rotation resumed'),
    onError: () => toast.error('Failed to unpin'),
  })
  const force = useMutation({
    mutationFn: (modeID: string) => api.force(modeID),
    onSuccess: (_data, modeID) => toast.success(`Pinned to ${modeID}`),
    onError: () => toast.error('Failed to force mode'),
  })

  const busy =
    pause.isPending ||
    resume.isPending ||
    skip.isPending ||
    unpin.isPending ||
    force.isPending

  return (
    <main className="mx-auto max-w-lg px-4 py-10 space-y-4">
      <h1 className="text-2xl font-bold">Bestaboard</h1>

      {!connected && !error && !data && (
        <p className="text-muted-foreground text-sm">Connecting…</p>
      )}

      {error && (
        <div className="rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">
          Lost connection to board service — reconnecting…
        </div>
      )}

      {data && (
        <>
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Status</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Current mode</span>
                <Badge variant="outline">{data.current_mode}</Badge>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Rotation</span>
                <Badge variant={data.paused ? 'destructive' : 'default'}>
                  {data.paused ? 'paused' : 'running'}
                </Badge>
              </div>
              {data.pinned && (
                <div className="flex items-center justify-between">
                  <span className="text-sm text-muted-foreground">Pinned</span>
                  <Badge variant="secondary">yes</Badge>
                </div>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Controls</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              {/* Pause / Resume */}
              <div className="flex gap-2">
                <button
                  onClick={() => pause.mutate()}
                  disabled={busy || data.paused}
                  className="flex-1 rounded-md border px-3 py-2 text-sm font-medium disabled:opacity-40 hover:bg-muted transition-colors"
                >
                  Pause
                </button>
                <button
                  onClick={() => resume.mutate()}
                  disabled={busy || !data.paused}
                  className="flex-1 rounded-md border px-3 py-2 text-sm font-medium disabled:opacity-40 hover:bg-muted transition-colors"
                >
                  Resume
                </button>
              </div>

              {/* Skip */}
              <button
                onClick={() => skip.mutate()}
                disabled={busy || data.paused}
                className="w-full rounded-md border px-3 py-2 text-sm font-medium disabled:opacity-40 hover:bg-muted transition-colors"
              >
                Skip to next
              </button>

              {/* Force mode */}
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">Force a mode</p>
                <div className="flex flex-wrap gap-2">
                  {data.mode_ids.map((id) => (
                    <button
                      key={id}
                      onClick={() => force.mutate(id)}
                      disabled={busy || (data.pinned && data.current_mode === id)}
                      className="rounded-md border px-3 py-1.5 text-sm font-medium disabled:opacity-40 hover:bg-muted transition-colors"
                    >
                      {id}
                    </button>
                  ))}
                </div>
              </div>

              {/* Unpin */}
              {data.pinned && (
                <button
                  onClick={() => unpin.mutate()}
                  disabled={busy}
                  className="w-full rounded-md border px-3 py-2 text-sm font-medium disabled:opacity-40 hover:bg-muted transition-colors"
                >
                  Unpin (resume rotation)
                </button>
              )}
            </CardContent>
          </Card>
        </>
      )}
    </main>
  )
}
