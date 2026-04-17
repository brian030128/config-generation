import { Navigate, Outlet } from "react-router-dom"
import { useAuth } from "@/lib/auth"
import { Sidebar } from "./sidebar"
import { Breadcrumbs } from "./breadcrumbs"

export function AppShell() {
  const { token } = useAuth()

  if (!token) {
    return <Navigate to="/login" replace />
  }

  return (
    <div className="flex h-screen">
      <Sidebar />
      <div className="flex flex-1 flex-col overflow-hidden">
        <header className="flex h-14 items-center border-b px-6">
          <Breadcrumbs />
        </header>
        <main className="flex-1 overflow-auto p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
