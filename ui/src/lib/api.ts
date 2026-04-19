const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'
const AUTH_TOKEN = import.meta.env.VITE_AUTH_TOKEN ?? ''

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(AUTH_TOKEN ? { Authorization: `Bearer ${AUTH_TOKEN}` } : {}),
  }

  const res = await fetch(`${API_URL}${path}`, { ...options, headers })

  if (!res.ok) {
    throw new Error(`${res.status} ${res.statusText}`)
  }

  return res.json() as Promise<T>
}

export type Status = {
  current_mode: string
  paused: boolean
  pinned: boolean
  mode_ids: string[]
}

export const api = {
  status: () => request<Status>('/status'),
  pause: () => request<void>('/pause', { method: 'POST' }),
  resume: () => request<void>('/resume', { method: 'POST' }),
  skip: () => request<void>('/skip', { method: 'POST' }),
  force: (modeID: string) =>
    request<void>(`/force/${modeID}`, { method: 'POST' }),
  unpin: () => request<void>('/unpin', { method: 'POST' }),
}
