const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'

function getToken(): string {
  if (typeof window === 'undefined') return ''
  return localStorage.getItem('auth_token') ?? ''
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const token = getToken()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  }

  const res = await fetch(`${API_URL}${path}`, { ...options, headers })

  if (!res.ok) {
    if (res.status === 401 && typeof window !== 'undefined') {
      localStorage.removeItem('auth_token')
      window.location.href = '/login'
    }
    throw new Error(`${res.status} ${res.statusText}`)
  }

  return res.json() as Promise<T>
}

export type ModeInfo = {
  id: string
  enabled: boolean
}

export type BoardNote = {
  id: number
  text: string
  created_at: string
  expires_at: string
  dismissed_at: string | null
  active: boolean
}

export type Status = {
  current_mode: string
  paused: boolean
  pinned: boolean
  modes: ModeInfo[]
}

export const api = {
  login: (username: string, password: string) =>
    request<{ token: string }>('/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  logout: async () => {
    await request<void>('/logout', { method: 'POST' }).catch(() => {})
    if (typeof window !== 'undefined') localStorage.removeItem('auth_token')
  },
  status: () => request<Status>('/status'),
  pause: () => request<void>('/pause', { method: 'POST' }),
  resume: () => request<void>('/resume', { method: 'POST' }),
  skip: () => request<void>('/skip', { method: 'POST' }),
  force: (modeID: string) =>
    request<void>(`/force/${modeID}`, { method: 'POST' }),
  unpin: () => request<void>('/unpin', { method: 'POST' }),
  enableMode: (modeID: string) =>
    request<void>(`/modes/${modeID}/enable`, { method: 'POST' }),
  disableMode: (modeID: string) =>
    request<void>(`/modes/${modeID}/disable`, { method: 'POST' }),
  previewMode: (modeID: string) =>
    request<{ id: string; text: string }>(`/modes/${modeID}/preview`),
  listNotes: () => request<BoardNote[]>('/notes'),
  createNote: (text: string, durationMinutes: number) =>
    request<BoardNote>('/notes', {
      method: 'POST',
      body: JSON.stringify({ text, duration_minutes: durationMinutes }),
    }),
  dismissNote: (id: number) =>
    request<void>(`/notes/${id}`, { method: 'DELETE' }),
  createUser: (username: string, password: string) =>
    request<{ username: string }>('/users', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  getPreferences: () => request<Preferences>('/preferences'),
  updatePreferences: (prefs: Preferences) =>
    request<Preferences>('/preferences', {
      method: 'PATCH',
      body: JSON.stringify(prefs),
    }),
}

export type Preferences = {
  rotation_interval: string
  static_text: string
  weather_latitude: number
  weather_longitude: number
  weather_timezone: string
  weather_units: string
  note_duration: string
}
