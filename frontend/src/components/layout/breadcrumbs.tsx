import { Link, useLocation } from "react-router-dom"
import { ChevronRight } from "lucide-react"
import { Fragment } from "react"

const labelMap: Record<string, string> = {
  projects: "Projects",
  "global-values": "Global Values",
  env: "Environments",
}

export function Breadcrumbs() {
  const { pathname } = useLocation()
  const segments = pathname.split("/").filter(Boolean)

  const crumbs: { label: string; to: string }[] = []
  let path = ""

  for (let i = 0; i < segments.length; i++) {
    const seg = segments[i]
    path += `/${seg}`

    // Skip "env" as a standalone breadcrumb segment — use the next segment as label
    if (seg === "env") continue

    const label = labelMap[seg] ?? decodeURIComponent(seg)
    crumbs.push({ label, to: path })
  }

  if (crumbs.length <= 1) return null

  return (
    <nav className="flex items-center gap-1 text-sm text-muted-foreground">
      {crumbs.map((crumb, i) => (
        <Fragment key={crumb.to}>
          {i > 0 && <ChevronRight className="h-3 w-3" />}
          {i === crumbs.length - 1 ? (
            <span className="font-medium text-foreground">{crumb.label}</span>
          ) : (
            <Link to={crumb.to} className="hover:text-foreground transition-colors">
              {crumb.label}
            </Link>
          )}
        </Fragment>
      ))}
    </nav>
  )
}
