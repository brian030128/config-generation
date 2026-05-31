import { Link } from "react-router-dom"
import {
  FolderOpen,
  Shield,
  ShieldCheck,
  UserRound,
} from "lucide-react"
import { useAuth } from "@/lib/auth"
import { useRoles } from "@/hooks/use-roles"
import { useProjects } from "@/hooks/use-projects"
import {
  gvAtomsToCapabilities,
  gvCapabilityLabels,
  inferTarget,
  projectAtomsToCapabilities,
  projectCapabilityLabels,
} from "@/lib/role-permissions"
import { cn, getApiErrorMessage } from "@/lib/utils"
import type { Role } from "@/api/types"
import { Badge } from "@/components/ui/badge"

function initialsOf(name: string): string {
  const trimmed = name.trim()
  if (!trimmed) return "?"
  const parts = trimmed.split(/\s+/).filter(Boolean)
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
}

function formatDate(iso: string): string {
  try {
    return new Intl.DateTimeFormat(undefined, {
      year: "numeric",
      month: "short",
      day: "numeric",
    }).format(new Date(iso))
  } catch {
    return iso
  }
}

function describeRole(role: Role): { target: string; labels: string[] } {
  const perms = role.permissions ?? []
  const t = inferTarget(perms)
  if (t?.kind === "project") {
    return {
      target: `Project: ${t.project}`,
      labels: projectCapabilityLabels(
        projectAtomsToCapabilities(perms, t.project).caps,
      ),
    }
  }
  if (t?.kind === "global-values") {
    return {
      target: `Global values: ${t.name}`,
      labels: gvCapabilityLabels(gvAtomsToCapabilities(perms, t.name).caps),
    }
  }
  return { target: "No scope", labels: [] }
}

function SectionCard({
  title,
  description,
  children,
}: {
  title: string
  description?: string
  children: React.ReactNode
}) {
  return (
    <section className="space-y-3 rounded-lg border bg-card px-5 py-4 shadow-xs">
      <header className="space-y-1">
        <h2 className="text-base font-semibold">{title}</h2>
        {description && (
          <p className="text-sm text-muted-foreground">{description}</p>
        )}
      </header>
      {children}
    </section>
  )
}

function EmptyState({ children }: { children: React.ReactNode }) {
  return (
    <p className="rounded-md border border-dashed bg-muted/30 px-3 py-4 text-center text-sm text-muted-foreground">
      {children}
    </p>
  )
}

export default function AccountPage() {
  const { user } = useAuth()
  const rolesQuery = useRoles(!!user)
  const projectsQuery = useProjects()

  if (!user) return null

  const displayName = user.display_name || user.username
  const initials = initialsOf(displayName)

  const myRoles = (rolesQuery.data?.items ?? []).filter((role) =>
    (role.members ?? []).some((m) => m.user_id === user.id),
  )
  const projects = projectsQuery.data?.items ?? []

  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <h1 className="text-2xl font-semibold">Account</h1>
        <p className="text-sm text-muted-foreground">
          Your profile, assigned roles, and project memberships. All data
          shown here is your own — to manage other users, visit the relevant
          project or role page.
        </p>
      </div>

      <SectionCard title="Profile">
        <div className="flex items-start gap-4">
          <div
            aria-hidden="true"
            className={cn(
              "flex size-14 shrink-0 items-center justify-center rounded-full",
              "bg-primary/10 text-base font-semibold text-primary",
              "ring-1 ring-inset ring-primary/15",
            )}
          >
            {initials}
          </div>
          <div className="min-w-0 flex-1 space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="truncate text-lg font-medium">
                {displayName}
              </span>
              {user.superuser && (
                <Badge variant="secondary" className="gap-1">
                  <ShieldCheck className="h-3 w-3" aria-hidden="true" />
                  Superuser
                </Badge>
              )}
            </div>
            <p className="text-sm text-muted-foreground">@{user.username}</p>
            <p className="text-xs text-muted-foreground">
              Member since {formatDate(user.created_at)}
            </p>
            {user.superuser && (
              <p className="pt-1 text-xs text-muted-foreground">
                Superusers bypass all permission checks and can manage global
                roles. Use this access carefully.
              </p>
            )}
          </div>
        </div>
      </SectionCard>

      <SectionCard
        title="Roles"
        description="Roles assigned to you. Each role grants a bundle of permission atoms."
      >
        {rolesQuery.isLoading && (
          <p className="text-sm text-muted-foreground">Loading roles…</p>
        )}
        {rolesQuery.error && (
          <p className="text-sm text-destructive">
            Failed to load roles: {getApiErrorMessage(rolesQuery.error)}
          </p>
        )}
        {!rolesQuery.isLoading && !rolesQuery.error && myRoles.length === 0 && (
          <EmptyState>You have no roles assigned.</EmptyState>
        )}
        {myRoles.length > 0 && (
          <ul className="space-y-2">
            {myRoles.map((role) => {
              const { target, labels } = describeRole(role)
              return (
                <li
                  key={role.id}
                  className="space-y-2 rounded-md border px-3 py-2"
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <Shield
                      className="h-4 w-4 shrink-0 text-muted-foreground"
                      aria-hidden="true"
                    />
                    <span className="font-medium">{role.name}</span>
                    {role.is_auto_created && (
                      <Badge variant="outline" className="text-xs">
                        Auto-created
                      </Badge>
                    )}
                    <span className="text-xs text-muted-foreground">
                      {target}
                    </span>
                  </div>
                  {labels.length > 0 && (
                    <div className="flex flex-wrap gap-1">
                      {labels.map((label) => (
                        <span
                          key={label}
                          className="rounded-full border bg-muted/40 px-2 py-0.5 text-xs text-muted-foreground"
                        >
                          {label}
                        </span>
                      ))}
                    </div>
                  )}
                </li>
              )
            })}
          </ul>
        )}
      </SectionCard>

      <SectionCard
        title="Projects"
        description={
          user.superuser
            ? "As a superuser, you can see every project in the system."
            : "Projects you are a member of."
        }
      >
        {projectsQuery.isLoading && (
          <p className="text-sm text-muted-foreground">Loading projects…</p>
        )}
        {projectsQuery.error && (
          <p className="text-sm text-destructive">
            Failed to load projects: {getApiErrorMessage(projectsQuery.error)}
          </p>
        )}
        {!projectsQuery.isLoading &&
          !projectsQuery.error &&
          projects.length === 0 && (
            <EmptyState>
              You are not a member of any project yet. Ask a project admin to
              add you.
            </EmptyState>
          )}
        {projects.length > 0 && (
          <ul className="divide-y rounded-md border">
            {projects.map((project) => (
              <li key={project.id}>
                <Link
                  to={`/projects/${encodeURIComponent(project.name)}`}
                  className="flex items-center gap-3 px-3 py-2 transition-colors hover:bg-accent/40"
                >
                  <FolderOpen
                    className="h-4 w-4 shrink-0 text-muted-foreground"
                    aria-hidden="true"
                  />
                  <span className="min-w-0 flex-1 truncate font-medium">
                    {project.name}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    Created {formatDate(project.created_at)}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </SectionCard>

      <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
        <UserRound className="h-3.5 w-3.5" aria-hidden="true" />
        Profile editing is not yet available. Contact an administrator to
        update your display name or password.
      </p>
    </div>
  )
}
