import { useState } from "react"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"
import { useCreateGlobalValues } from "@/hooks/use-global-values"
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
import { Plus } from "lucide-react"

export function CreateGlobalValuesDialog() {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [initialKey, setInitialKey] = useState("")
  const [initialValue, setInitialValue] = useState("")
  const [approvalCondition, setApprovalCondition] = useState("1 x gv_group_admin")
  const createGV = useCreateGlobalValues()
  const navigate = useNavigate()

  function reset() {
    setName("")
    setInitialKey("")
    setInitialValue("")
    setApprovalCondition("1 x gv_group_admin")
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim() || !initialKey.trim() || !initialValue.trim()) return
    createGV.mutate(
      {
        name: name.trim(),
        payload: { [initialKey.trim()]: initialValue.trim() },
        commit_message: "Initial creation",
        approval_condition: approvalCondition.trim() || undefined,
      },
      {
        onSuccess: (gv) => {
          toast.success(`Global values "${gv.name}" created`)
          setOpen(false)
          reset()
          navigate(`/global-values/${gv.name}`)
        },
        onError: (err) => {
          toast.error("Failed to create global values", {
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
          New Entry
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create Global Values Entry</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="gv-name">Name</Label>
            <Input
              id="gv-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. prod_db_values"
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="gv-approval">Approval Condition</Label>
            <Input
              id="gv-approval"
              value={approvalCondition}
              onChange={(e) => setApprovalCondition(e.target.value)}
              placeholder="e.g. 1 x gv_group_admin"
            />
          </div>
          <div className="space-y-2">
            <Label>Initial Key-Value Pair</Label>
            <div className="flex gap-2">
              <Input
                value={initialKey}
                onChange={(e) => setInitialKey(e.target.value)}
                placeholder="Key"
                required
              />
              <Input
                value={initialValue}
                onChange={(e) => setInitialValue(e.target.value)}
                placeholder="Value"
                required
              />
            </div>
          </div>
          <div className="flex justify-end gap-2">
            <Button variant="outline" type="button" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={createGV.isPending}>
              {createGV.isPending ? "Creating..." : "Create"}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
