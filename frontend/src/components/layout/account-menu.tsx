import { useState } from "react"
import { Link } from "react-router-dom"
import { ChevronDown, LogOut, ShieldCheck, UserRound } from "lucide-react"
import { useAuth } from "@/lib/auth"
import { cn } from "@/lib/utils"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

function initialsOf(name: string): string {
  const trimmed = name.trim()
  if (!trimmed) return "?"
  const parts = trimmed.split(/\s+/).filter(Boolean)
  if (parts.length === 1) {
    return parts[0].slice(0, 2).toUpperCase()
  }
  return (parts[0][0] + parts.at(-1)[0]).toUpperCase()
}

export function AccountMenu({ className }: Readonly<{ className?: string }>) {
  const { user, logout } = useAuth()
  const [signOutOpen, setSignOutOpen] = useState(false)

  if (!user) return null

  const displayName = user.display_name || user.username
  const initials = initialsOf(displayName)

  return (
    <div className={className}>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="sm"
            className="h-10 gap-2 pl-1 pr-2"
            aria-label="Open account menu"
          >
            <span
              aria-hidden="true"
              className={cn(
                "flex size-8 items-center justify-center rounded-full",
                "bg-primary/10 text-xs font-semibold text-primary",
                "ring-1 ring-inset ring-primary/15",
              )}
            >
              {initials}
            </span>
            <span className="hidden max-w-40 truncate text-sm font-medium sm:inline">
              {displayName}
            </span>
            <ChevronDown
              className="h-4 w-4 shrink-0 text-muted-foreground"
              aria-hidden="true"
            />
          </Button>
        </DropdownMenuTrigger>

        <DropdownMenuContent align="end" className="w-64">
          <div className="px-2 py-2">
            <div className="flex items-center gap-2">
              <span className="truncate text-sm font-medium">
                {displayName}
              </span>
              {user.superuser && (
                <Badge variant="secondary" className="gap-1">
                  <ShieldCheck className="h-3 w-3" aria-hidden="true" />
                  Superuser
                </Badge>
              )}
            </div>
            {user.display_name && (
              <p className="truncate text-xs text-muted-foreground">
                @{user.username}
              </p>
            )}
          </div>

          <DropdownMenuSeparator />

          <DropdownMenuItem asChild>
            <Link to="/account" className="cursor-pointer">
              <UserRound className="h-4 w-4" aria-hidden="true" />
              Account
            </Link>
          </DropdownMenuItem>

          <DropdownMenuSeparator />

          <DropdownMenuItem
            variant="destructive"
            onSelect={(event) => {
              event.preventDefault()
              setSignOutOpen(true)
            }}
          >
            <LogOut className="h-4 w-4" aria-hidden="true" />
            Sign out
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Dialog open={signOutOpen} onOpenChange={setSignOutOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Sign out?</DialogTitle>
            <DialogDescription>
              You will need to sign in again to continue managing
              configuration.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setSignOutOpen(false)}>
              Cancel
            </Button>
            <Button onClick={logout}>Sign out</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
