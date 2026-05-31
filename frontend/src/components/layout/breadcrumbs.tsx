import { Link, useLocation } from "react-router-dom"
import { ChevronRight, House } from "lucide-react"
import { Fragment } from "react"

const labelMap: Record<string, string> = {
  projects: "Projects",
  "global-values": "Global Values",
  env: "Environments",
  templates: "Templates",
  environments: "Environments",
  members: "Members",
  permissions: "Permissions",
  account: "Account",
  workspace: "Workspace",
  roles: "Roles",
  "pull-requests": "Pull Requests",
  deploy: "Deploy",
}

export function Breadcrumbs() {
  const { pathname } = useLocation()
  const segments = pathname.split("/").filter(Boolean)

  // Always start with a Home crumb
  const crumbs: { label: string; to: string; icon?: boolean }[] = [
    { label: "Home", to: "/", icon: true },
  ]
  let path = ""

  for (let i = 0; i < segments.length; i++) {
    const seg = segments[i]
    path += `/${seg}`

    // Skip "env" as a standalone breadcrumb segment — use the next segment as label
    if (seg === "env") continue

    // Skip intermediate numeric id segments (e.g. the userId in
    // /projects/:name/members/:userId/permissions); they have no standalone
    // route, so linking them would 404. The accumulated path still includes
    // them for the following (real) crumb.
    if (/^\d+$/.test(seg) && i < segments.length - 1) continue

    const label = labelMap[seg] ?? decodeURIComponent(seg)
    crumbs.push({ label, to: path })
  }

  return (
    <nav aria-label="Breadcrumb" className="flex items-center gap-1 text-sm text-muted-foreground">
      {crumbs.map((crumb, i) => (
        <Fragment key={crumb.to}>
          {i > 0 && <ChevronRight className="h-3 w-3 shrink-0" />}
          {i === crumbs.length - 1 ? (
            <span className="font-medium text-foreground">
              {crumb.icon ? <House className="h-3.5 w-3.5" aria-label="Home" /> : crumb.label}
            </span>
          ) : (
            <Link to={crumb.to} className="hover:text-foreground transition-colors flex items-center">
              {crumb.icon ? <House className="h-3.5 w-3.5" aria-label="Home" /> : crumb.label}
            </Link>
          )}
        </Fragment>
      ))}
    </nav>
  )
}
