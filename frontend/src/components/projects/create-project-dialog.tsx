import { useState } from "react"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"
import { useCreateProject } from "@/hooks/use-projects"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Plus } from "lucide-react"

export function CreateProjectDialog() {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [approvalCondition, setApprovalCondition] = useState("1 x project_admin")
  const navigate = useNavigate()
  const createProject = useCreateProject()

  function reset() {
    setName("")
    setDescription("")
    setApprovalCondition("1 x project_admin")
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim()) return
    createProject.mutate(
      {
        name: name.trim(),
        description: description.trim() || undefined,
        approval_condition: approvalCondition.trim() || undefined,
      },
      {
        onSuccess: (project) => {
          toast.success(`Project "${project.name}" created`)
          setOpen(false)
          reset()
          navigate(`/projects/${project.name}`)
        },
        onError: (err) => {
          toast.error("Failed to create project", {
            description: (err as Error).message,
          })
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>
          <Plus className="mr-2 h-4 w-4" />
          New Project
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create Project</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="proj-name">Name</Label>
            <Input
              id="proj-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="my-service"
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="proj-desc">Description</Label>
            <Textarea
              id="proj-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional description"
              rows={2}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="proj-approval">Approval Condition</Label>
            <Input
              id="proj-approval"
              value={approvalCondition}
              onChange={(e) => setApprovalCondition(e.target.value)}
              placeholder="1 x project_admin"
            />
          </div>
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => setOpen(false)}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={createProject.isPending}>
              {createProject.isPending ? "Creating..." : "Create"}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
