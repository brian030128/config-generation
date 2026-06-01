import { useState } from "react"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"
import type { Project } from "@/api/types"
import { useDeleteProject } from "@/hooks/use-projects"
import { formatRelativeTime } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
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
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { MoreHorizontal, Trash2 } from "lucide-react"

export function ProjectCard({ project }: Readonly<{ project: Project }>) {
  const [deleteOpen, setDeleteOpen] = useState(false)
  const navigate = useNavigate()
  const deleteProject = useDeleteProject()

  function openProject() {
    navigate(`/projects/${project.name}`)
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLDivElement>) {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault()
      openProject()
    }
  }

  function handleDelete() {
    deleteProject.mutate(project.name, {
      onSuccess: () => {
        toast.success(`Project "${project.name}" deleted`)
        setDeleteOpen(false)
      },
      onError: (err) => {
        toast.error("Failed to delete project", {
          description: (err as Error).message,
        })
      },
    })
  }

  return (
    <>
      <Card
        role="link"
        tabIndex={0}
        onClick={openProject}
        onKeyDown={handleKeyDown}
        className="group cursor-pointer gap-5 rounded-lg transition-all hover:border-foreground/15 hover:bg-accent/40 hover:shadow-md focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
      >
        <CardHeader className="pb-0">
          <CardTitle className="line-clamp-1 text-base">
            {project.name}
          </CardTitle>
          <CardDescription className="line-clamp-2 min-h-10">
            {project.description || "No description provided"}
          </CardDescription>
          <CardAction>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className="opacity-70 transition-opacity hover:opacity-100 group-hover:opacity-100"
                  aria-label={`Open actions for ${project.name}`}
                  onClick={(e) => e.stopPropagation()}
                >
                  <MoreHorizontal />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent
                align="end"
                onClick={(e) => e.stopPropagation()}
              >
                <DropdownMenuItem
                  variant="destructive"
                  onSelect={(e) => {
                    e.preventDefault()
                    setDeleteOpen(true)
                  }}
                >
                  <Trash2 />
                  Delete project
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </CardAction>
        </CardHeader>
        <CardContent>
          <p className="shrink-0 text-xs text-muted-foreground">
            Updated {formatRelativeTime(project.updated_at)}
          </p>
        </CardContent>
      </Card>

      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete project "{project.name}"?</DialogTitle>
            <DialogDescription>
              This removes the project and its related templates, environments,
              roles, and values. This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setDeleteOpen(false)}
              disabled={deleteProject.isPending}
            >
              Cancel
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={handleDelete}
              disabled={deleteProject.isPending}
            >
              {deleteProject.isPending ? "Deleting..." : "Delete project"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
