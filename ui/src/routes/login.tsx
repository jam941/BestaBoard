import { createFileRoute, Navigate, useNavigate } from '@tanstack/react-router'
import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { api } from '#/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '#/components/ui/card'

export const Route = createFileRoute('/login')({ component: LoginPage })

function LoginPage() {
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')

  if (typeof window !== 'undefined' && localStorage.getItem('auth_token')) {
    return <Navigate to="/" replace />
  }

  const login = useMutation({
    mutationFn: () => api.login(username, password),
    onSuccess: ({ token }) => {
      localStorage.setItem('auth_token', token)
      navigate({ to: '/' })
    },
  })

  return (
    <main className="mx-auto max-w-sm px-4 py-20">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Sign in</CardTitle>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              login.mutate()
            }}
            className="space-y-3"
          >
            <input
              type="text"
              placeholder="Username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              required
              className="w-full rounded-md border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
            />
            <input
              type="password"
              placeholder="Password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              required
              className="w-full rounded-md border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
            />
            {login.isError && (
              <p className="text-sm text-destructive">Invalid credentials</p>
            )}
            <button
              type="submit"
              disabled={login.isPending}
              className="w-full rounded-md border px-3 py-2 text-sm font-medium disabled:opacity-40 hover:bg-muted transition-colors"
            >
              {login.isPending ? 'Signing in…' : 'Sign in'}
            </button>
          </form>
        </CardContent>
      </Card>
    </main>
  )
}
