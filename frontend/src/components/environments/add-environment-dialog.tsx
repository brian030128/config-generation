import { useState } from "react"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { Input } from "@/components/ui/input"
import { Plus } from "lucide-react"
import { useStageChange } from "@/hooks/use-pull-requests"

export function AddEnvironmentDialog({
  projectName,
}: {
  projectName: string
}) {
  const [open, setOpen] = useState(false)
  const [envName, setEnvName] = useState("")
  const navigate = useNavigate()
  const stageChange = useStageChange(projectName)

  const trimmed = envName.trim()

  function handleCreate() {
    if (!trimmed) return
    stageChange.mutate(
      {
        object_type: "environment",
        proposed_payload: JSON.stringify({ name: trimmed }),
      },
      {
        onSuccess: () => {
          setOpen(false)
          setEnvName("")
          navigate(`/workspace/${projectName}/env/${trimmed}`)
        },
        onError: (err) => {
          toast.error("Failed to stage environment", {
            description: (err as Error).message,
          })
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button size="sm">
          <Plus className="mr-2 h-4 w-4" />
          Add Environment
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add Environment</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>Environment Name</Label>
            <Input
              value={envName}
              onChange={(e) => setEnvName(e.target.value)}
              placeholder="e.g. staging, production"
              onKeyDown={(e) => {
                if (e.key === "Enter") handleCreate()
              }}
            />
          </div>
          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleCreate} disabled={!trimmed || stageChange.isPending}>
              {stageChange.isPending ? "Creating..." : "Create Environment"}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
