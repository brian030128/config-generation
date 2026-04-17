import { Link } from "react-router-dom"
import type { Project } from "@/api/types"
import { formatRelativeTime } from "@/lib/utils"
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card"

export function ProjectCard({ project }: { project: Project }) {
  return (
    <Link to={`/projects/${project.name}`}>
      <Card className="transition-colors hover:bg-accent/50">
        <CardHeader className="pb-2">
          <CardTitle className="text-base">{project.name}</CardTitle>
          <CardDescription>
            {project.description || "(no description)"}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-xs text-muted-foreground">
            Updated {formatRelativeTime(project.updated_at)}
          </p>
        </CardContent>
      </Card>
    </Link>
  )
}
