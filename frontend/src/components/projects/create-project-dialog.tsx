import { useState } from "react"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"
import { useCreateProject } from "@/hooks/use-projects"
import { validateApprovalCondition } from "@/lib/approval-condition"
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
  const [approvalCondition, setApprovalCondition] = useState("")
  const navigate = useNavigate()
  const createProject = useCreateProject()

  // The project's admin role "<name>_project_admin" is auto-created; it's the
  // only role that exists at creation. Left blank, the condition defaults to
  // "1 x <name>_project_admin"; a custom condition may only reference that role
  // (other approver roles are created on the Roles page later, then referenced).
  const adminRole = `${name.trim()}_project_admin`
  const approvalValidation = validateApprovalCondition(
    approvalCondition.trim() || `1 x ${adminRole}`,
    [],
    [adminRole],
  )

  function reset() {
    setName("")
    setDescription("")
    setApprovalCondition("")
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim() || !approvalValidation.valid) return
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
              placeholder={name.trim() ? `1 x ${adminRole}` : "1 x <name>_project_admin"}
            />
            {approvalValidation.valid ? (
              <p className="text-xs text-muted-foreground">
                Defaults to <code>1 x {adminRole}</code>. Add other approver
                roles on the Roles page later, then update this condition.
              </p>
            ) : (
              <ul className="space-y-0.5 text-xs text-destructive">
                {approvalValidation.errors.map((e) => (
                  <li key={e}>{e}</li>
                ))}
              </ul>
            )}
          </div>
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => setOpen(false)}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={createProject.isPending || !approvalValidation.valid}
            >
              {createProject.isPending ? "Creating..." : "Create"}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
