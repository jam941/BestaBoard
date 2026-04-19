import { Link } from '@tanstack/react-router'

export default function Header() {
  return (
    <header className="sticky top-0 z-50 border-b bg-background/80 px-4 backdrop-blur-lg">
      <nav className="mx-auto flex max-w-lg items-center py-3">
        <Link to="/" className="text-sm font-semibold no-underline">
          Bestaboard
        </Link>
      </nav>
    </header>
  )
}
