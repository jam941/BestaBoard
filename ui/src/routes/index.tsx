import { createFileRoute } from '@tanstack/react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
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

const DURATION_OPTIONS = [
  { label: '5 min', value: 5 },
  { label: '15 min', value: 15 },
  { label: '30 min', value: 30 },
  { label: '1 hour', value: 60 },
]

function StatusPage() {
  const { data, error } = useSSEStatus()
  const [previewText, setPreviewText] = useState<Record<string, string>>({})
  const [noteText, setNoteText] = useState('')
  const [noteDuration, setNoteDuration] = useState(15)
  const queryClient = useQueryClient()

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
  const enableMode = useMutation({
    mutationFn: (modeID: string) => api.enableMode(modeID),
    onSuccess: (_data, modeID) => toast.success(`${modeID} enabled`),
    onError: () => toast.error('Failed to enable mode'),
  })
  const disableMode = useMutation({
    mutationFn: (modeID: string) => api.disableMode(modeID),
    onSuccess: (_data, modeID) => toast.success(`${modeID} disabled`),
    onError: () => toast.error('Failed to disable mode'),
  })
  const { data: notes } = useQuery({
    queryKey: ['notes'],
    queryFn: api.listNotes,
    refetchInterval: 15_000,
  })

  const createNote = useMutation({
    mutationFn: ({ text, duration }: { text: string; duration: number }) =>
      api.createNote(text, duration),
    onSuccess: () => {
      toast.success('Note sent to board')
      setNoteText('')
      queryClient.invalidateQueries({ queryKey: ['notes'] })
    },
    onError: () => toast.error('Failed to send note'),
  })

  const dismissNote = useMutation({
    mutationFn: (id: number) => api.dismissNote(id),
    onSuccess: () => {
      toast.success('Note dismissed')
      queryClient.invalidateQueries({ queryKey: ['notes'] })
    },
    onError: () => toast.error('Failed to dismiss note'),
  })

  const previewMode = useMutation({
    mutationFn: (modeID: string) => api.previewMode(modeID),
    onSuccess: (result, modeID) =>
      setPreviewText((prev) =>
        prev[modeID] === result.text
          ? { ...prev, [modeID]: '' } // toggle off if same
          : { ...prev, [modeID]: result.text },
      ),
    onError: () => toast.error('Preview failed'),
  })

  const busy =
    pause.isPending ||
    resume.isPending ||
    skip.isPending ||
    unpin.isPending ||
    force.isPending ||
    enableMode.isPending ||
    disableMode.isPending ||
    previewMode.isPending

  return (
    <main className="mx-auto max-w-lg px-4 py-10 space-y-4">
      <h1 className="text-2xl font-bold">Bestaboard</h1>

      {!data && !error && (
        <p className="text-muted-foreground text-sm">Connecting…</p>
      )}

      {error && (
        <div className="rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">
          Lost connection to board service — reconnecting…
        </div>
      )}

      {data && (
        <>
          {/* Status */}
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

          {/* Controls */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Controls</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
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

              <button
                onClick={() => skip.mutate()}
                disabled={busy || data.paused}
                className="w-full rounded-md border px-3 py-2 text-sm font-medium disabled:opacity-40 hover:bg-muted transition-colors"
              >
                Skip to next
              </button>

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

          {/* Modes */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Modes</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              {data.modes.map((m) => {
                const isCurrent = data.current_mode === m.id
                const isPinned = data.pinned && isCurrent
                return (
                  <div key={m.id}>
                  <div
                    className="flex items-center justify-between gap-2 rounded-md border px-3 py-2"
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <span className="text-sm font-medium truncate">{m.id}</span>
                      {isCurrent && (
                        <Badge variant="outline" className="shrink-0 text-xs">
                          {data.pinned ? 'pinned' : 'active'}
                        </Badge>
                      )}
                      {!m.enabled && (
                        <Badge variant="secondary" className="shrink-0 text-xs">
                          off
                        </Badge>
                      )}
                    </div>
                    <div className="flex gap-1.5 shrink-0">
                      <button
                        onClick={() => previewMode.mutate(m.id)}
                        disabled={busy}
                        className="rounded border px-2.5 py-1 text-xs font-medium disabled:opacity-40 hover:bg-muted transition-colors"
                      >
                        {previewText[m.id] ? 'Hide' : 'Preview'}
                      </button>
                      <button
                        onClick={() => force.mutate(m.id)}
                        disabled={busy || isPinned || !m.enabled}
                        className="rounded border px-2.5 py-1 text-xs font-medium disabled:opacity-40 hover:bg-muted transition-colors"
                      >
                        Force
                      </button>
                      {m.enabled ? (
                        <button
                          onClick={() => disableMode.mutate(m.id)}
                          disabled={busy}
                          className="rounded border px-2.5 py-1 text-xs font-medium disabled:opacity-40 hover:bg-muted transition-colors"
                        >
                          Disable
                        </button>
                      ) : (
                        <button
                          onClick={() => enableMode.mutate(m.id)}
                          disabled={busy}
                          className="rounded border px-2.5 py-1 text-xs font-medium disabled:opacity-40 hover:bg-muted transition-colors"
                        >
                          Enable
                        </button>
                      )}
                    </div>
                  </div>
                  {previewText[m.id] && (
                    <pre className="mt-1 rounded bg-muted px-3 py-2 text-xs font-mono leading-relaxed whitespace-pre">
                      {previewText[m.id]}
                    </pre>
                  )}
                  </div>
                )
              })}
            </CardContent>
          </Card>
        </>
      )}

      {/* Notes — always visible, independent of SSE connection */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Send a Note</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <textarea
            value={noteText}
            onChange={(e) => setNoteText(e.target.value)}
            placeholder="Type a message…"
            rows={2}
            maxLength={135}
            className="w-full rounded-md border bg-background px-3 py-2 text-sm resize-none focus:outline-none focus:ring-1 focus:ring-ring"
          />
          <div className="flex items-center gap-2">
            <div className="flex gap-1 flex-1">
              {DURATION_OPTIONS.map((opt) => (
                <button
                  key={opt.value}
                  onClick={() => setNoteDuration(opt.value)}
                  className={`flex-1 rounded border px-2 py-1 text-xs font-medium transition-colors ${
                    noteDuration === opt.value
                      ? 'bg-primary text-primary-foreground border-primary'
                      : 'hover:bg-muted'
                  }`}
                >
                  {opt.label}
                </button>
              ))}
            </div>
            <button
              onClick={() => createNote.mutate({ text: noteText, duration: noteDuration })}
              disabled={createNote.isPending || !noteText.trim()}
              className="rounded-md border px-4 py-1.5 text-sm font-medium disabled:opacity-40 hover:bg-muted transition-colors"
            >
              Send
            </button>
          </div>

          {notes && notes.length > 0 && (
            <div className="space-y-1.5 pt-1">
              <p className="text-xs text-muted-foreground">Recent</p>
              {notes.map((n) => (
                <div
                  key={n.id}
                  className="flex items-start justify-between gap-2 rounded border px-3 py-2"
                >
                  <div className="min-w-0">
                    <p className="text-sm truncate">{n.text}</p>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      {n.active
                        ? `Active · expires ${new Date(n.expires_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`
                        : n.dismissed_at
                          ? 'Dismissed'
                          : 'Expired'}
                    </p>
                  </div>
                  {n.active && (
                    <button
                      onClick={() => dismissNote.mutate(n.id)}
                      disabled={dismissNote.isPending}
                      className="shrink-0 rounded border px-2 py-1 text-xs font-medium disabled:opacity-40 hover:bg-muted transition-colors"
                    >
                      Dismiss
                    </button>
                  )}
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </main>
  )
}
