import { Link, useNavigate } from '@tanstack/react-router'
import { useMutation } from '@tanstack/react-query'
import { api } from '#/lib/api'

export default function Header() {
  const navigate = useNavigate()
  const logout = useMutation({
    mutationFn: api.logout,
    onSuccess: () => navigate({ to: '/login' }),
  })

  const isLoggedIn =
    typeof window !== 'undefined' && !!localStorage.getItem('auth_token')

  return (
    <header className="sticky top-0 z-50 border-b bg-background/80 px-4 backdrop-blur-lg">
      <nav className="mx-auto flex max-w-lg items-center justify-between py-3">
        <Link to="/" className="text-sm font-semibold no-underline">
          Bestaboard
        </Link>
        {isLoggedIn && (
          <button
            onClick={() => logout.mutate()}
            disabled={logout.isPending}
            className="text-sm text-muted-foreground hover:text-foreground transition-colors disabled:opacity-40"
          >
            Sign out
          </button>
        )}
      </nav>
    </header>
  )
}
