import { NavLink, useLocation } from "react-router-dom"
import { useState } from "react"
import {
  ChevronsLeft,
  FolderOpen,
  GitPullRequest,
  Globe,
  LogOut,
  Pencil,
  Rocket,
  Shield,
} from "lucide-react"
import { useAuth } from "@/lib/auth"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Logo } from "@/components/brand/logo"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Separator } from "@/components/ui/separator"

const navItems = [
  { to: "/projects", label: "Projects", icon: FolderOpen },
  { to: "/global-values", label: "Global Values", icon: Globe },
  { to: "/workspace", label: "Workspace", icon: Pencil },
  { to: "/pull-requests", label: "Pull Requests", icon: GitPullRequest },
  { to: "/deploy", label: "Deploy", icon: Rocket },
  { to: "/roles", label: "Roles", icon: Shield },
]

export function Sidebar() {
  const { user, logout } = useAuth()
  const location = useLocation()
  const [signOutOpen, setSignOutOpen] = useState(false)
  const [collapsed, setCollapsed] = useState(false)
  const activeIndex = navItems.findIndex((item) =>
    location.pathname.startsWith(item.to),
  )

  return (
    <aside
      className={cn(
        "flex h-screen flex-col overflow-hidden border-r bg-sidebar text-sidebar-foreground transition-[width] duration-200",
        collapsed ? "w-16" : "w-60",
      )}
    >
      <div
        className={cn(
          "relative flex h-18 items-center gap-2 border-b px-4",
          collapsed && "justify-center px-2",
        )}
      >
        {collapsed ? (
          <button
            type="button"
            onClick={() => setCollapsed(false)}
            title="Expand sidebar"
            className="flex rounded-md transition-opacity hover:opacity-80 focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
          >
            <Logo variant="icon" className="h-9 w-9" />
          </button>
        ) : (
          <Logo variant="wordmark" className="h-18 w-auto" />
        )}
        {!collapsed && (
          <Button
            variant="ghost"
            size="icon"
            className="ml-auto size-8 border border-border bg-background/80 shadow-xs hover:bg-accent dark:border-white/15 dark:bg-white/5 dark:hover:bg-white/10"
            onClick={() => setCollapsed(true)}
            title="Collapse sidebar"
          >
            <ChevronsLeft className="h-5 w-5" />
          </Button>
        )}
      </div>
      <nav className={cn("relative flex-1 space-y-1 p-2", collapsed && "px-2")}>
        {activeIndex >= 0 && (
          <div
            aria-hidden="true"
            className="absolute left-2 top-4 z-10 h-5 w-0.5 rounded-full bg-[#2563EB] transition-transform duration-300 ease-out"
            style={{ transform: `translateY(${activeIndex * 40}px)` }}
          />
        )}
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            title={collapsed ? item.label : undefined}
            className={({ isActive }) =>
              cn(
                "relative flex h-9 items-center rounded-md px-3 text-sm transition-colors",
                isActive
                  ? "bg-sidebar-accent font-medium text-[#2563EB]"
                  : "text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-accent-foreground",
              )
            }
          >
            <item.icon className="h-4 w-4 shrink-0" />
            <span
              className={cn(
                "overflow-hidden whitespace-nowrap transition-[max-width,opacity,margin] duration-200",
                collapsed
                  ? "max-w-0 opacity-0 ml-0"
                  : "max-w-40 opacity-100 ml-3 delay-100",
              )}
            >
              {item.label}
            </span>
          </NavLink>
        ))}
      </nav>
      <Separator />
      <div
        className={cn(
          "flex items-center justify-between p-3",
          collapsed && "justify-center px-2",
        )}
      >
        <span
          className={cn(
            "truncate text-sm text-muted-foreground",
            collapsed && "sr-only",
          )}
        >
          {user?.username ?? "Unknown"}
        </span>
        <Button
          variant="ghost"
          size="icon"
          onClick={() => setSignOutOpen(true)}
          title="Sign out"
        >
          <LogOut className="h-4 w-4" />
        </Button>
      </div>
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
    </aside>
  )
}
