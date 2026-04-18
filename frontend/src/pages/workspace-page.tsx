import { useNavigate } from "react-router-dom"
import { useState } from "react"
import { usePullRequests } from "@/hooks/use-pull-requests"
import { useAuth } from "@/lib/auth"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { formatRelativeTime } from "@/lib/utils"
import { ChevronRight } from "lucide-react"
import { useProjects } from "@/hooks/use-projects"

function statusVariant(status: string) {
  switch (status) {
    case "draft":
      return "secondary" as const
    case "open":
      return "default" as const
    case "approved":
      return "default" as const
    default:
      return "outline" as const
  }
}

export default function WorkspacePage() {
  const navigate = useNavigate()
  const { user } = useAuth()
  const { data: prs } = usePullRequests()
  const { data: projects } = useProjects()
  const [selectedProject, setSelectedProject] = useState("")

  const myActivePRs =
    prs?.items.filter(
      (pr) =>
        pr.author_id === user?.id &&
        ["draft", "open", "approved"].includes(pr.status) &&
        pr.project_id != null,
    ) ?? []

  // Projects that already have an active PR
  const activeProjectIds = new Set(myActivePRs.map((pr) => pr.project_id))
  const availableProjects =
    projects?.items.filter((p) => !activeProjectIds.has(p.id)) ?? []

  // We need project names for the PR cards - build a lookup
  const projectLookup = new Map(
    projects?.items.map((p) => [p.id, p.name]) ?? [],
  )

  function handleStart() {
    if (!selectedProject) return
    navigate(`/workspace/${selectedProject}`)
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Workspace</h1>

      {myActivePRs.length === 0 && (
        <p className="text-muted-foreground">
          No active workspaces. Start one below.
        </p>
      )}

      <div className="space-y-2">
        {myActivePRs.map((pr) => {
          const projectName = projectLookup.get(pr.project_id!) ?? `Project #${pr.project_id}`
          const changeCount = pr.changes?.length ?? 0

          return (
            <div
              key={pr.id}
              onClick={() => navigate(`/workspace/${projectName}`)}
              className="flex cursor-pointer items-center justify-between rounded-lg border px-4 py-3 transition-colors hover:bg-accent/50"
            >
              <div className="space-y-1">
                <div className="flex items-center gap-3">
                  <span className="font-medium">{projectName}</span>
                  <Badge variant={statusVariant(pr.status)}>{pr.status}</Badge>
                </div>
                {pr.title && (
                  <p className="text-sm text-muted-foreground">
                    PR #{pr.id}: {pr.title}
                  </p>
                )}
                <p className="text-xs text-muted-foreground">
                  {changeCount} change{changeCount !== 1 ? "s" : ""} · updated{" "}
                  {formatRelativeTime(pr.updated_at)}
                </p>
              </div>
              <ChevronRight className="h-4 w-4 text-muted-foreground" />
            </div>
          )
        })}
      </div>

      <div className="flex items-end gap-3 border-t pt-4">
        <div className="space-y-1">
          <span className="text-sm font-medium">Start new workspace</span>
          <Select value={selectedProject} onValueChange={setSelectedProject}>
            <SelectTrigger className="w-64">
              <SelectValue placeholder="Select a project" />
            </SelectTrigger>
            <SelectContent>
              {availableProjects.map((p) => (
                <SelectItem key={p.id} value={p.name}>
                  {p.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <Button onClick={handleStart} disabled={!selectedProject}>
          Start
        </Button>
      </div>
    </div>
  )
}
