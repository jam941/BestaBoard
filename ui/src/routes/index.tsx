import { createFileRoute } from '@tanstack/react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '#/lib/api'
import { Badge } from '#/components/ui/badge'
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '#/components/ui/card'

export const Route = createFileRoute('/')({ component: StatusPage })

function StatusPage() {
  const queryClient = useQueryClient()

  const { data, isError, isPending } = useQuery({
    queryKey: ['status'],
    queryFn: api.status,
    refetchInterval: (query) => (query.state.status === 'error' ? false : 10_000),
    refetchIntervalInBackground: false,
    retry: 1,
  })

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['status'] })

  const pause = useMutation({ mutationFn: api.pause, onSuccess: invalidate })
  const resume = useMutation({ mutationFn: api.resume, onSuccess: invalidate })
  const skip = useMutation({ mutationFn: api.skip, onSuccess: invalidate })
  const unpin = useMutation({ mutationFn: api.unpin, onSuccess: invalidate })
  const force = useMutation({
    mutationFn: (modeID: string) => api.force(modeID),
    onSuccess: invalidate,
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

      {isPending && <p className="text-muted-foreground text-sm">Loading…</p>}

      {isError && (
        <div className="rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">
          Could not reach the board service. Is the backend running?
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
                      disabled={busy || data.pinned && data.current_mode === id}
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
