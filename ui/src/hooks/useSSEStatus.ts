import { useEffect, useState } from 'react'
import { type Status } from '#/lib/api'
export type { Status }

const BASE_URL = import.meta.env.VITE_API_URL ?? ''

export function useSSEStatus() {
  const [data, setData] = useState<Status | null>(null)
  const [connected, setConnected] = useState(false)
  const [error, setError] = useState(false)

  useEffect(() => {
    const url = `${BASE_URL}/events`
    const es = new EventSource(url)
    let active = true

    es.onopen = () => {
      if (!active) return
      setConnected(true)
      setError(false)
    }

    es.onmessage = (e) => {
      if (!active) return
      try {
        setData(JSON.parse(e.data) as Status)
        setError(false)
      } catch {
      }
    }

    es.onerror = () => {
      if (!active) return
      setConnected(false)
      setError(true)
    }

    return () => {
      active = false
      es.close()
    }
  }, [])

  return { data, connected, error }
}
