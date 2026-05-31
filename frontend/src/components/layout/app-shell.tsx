import { Navigate, Outlet } from "react-router-dom"
import { useAuth } from "@/lib/auth"
import { Sidebar } from "./sidebar"
import { Breadcrumbs } from "./breadcrumbs"
import { SettingsPanel } from "./settings-panel"
import { AccountMenu } from "./account-menu"

export function AppShell() {
  const { user, loading } = useAuth()

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center text-sm text-muted-foreground">
        Loading...
      </div>
    )
  }

  if (!user) {
    return <Navigate to="/login" replace />
  }

  return (
    <div className="flex h-screen">
      <Sidebar />
      <div className="flex flex-1 flex-col overflow-hidden">
        <header className="flex h-18 items-center gap-4 border-b px-6">
          <Breadcrumbs />
          <AccountMenu className="ml-auto" />
        </header>
        <main className="flex-1 overflow-auto p-6">
          <Outlet />
        </main>
      </div>
      <SettingsPanel />
    </div>
  )
}
