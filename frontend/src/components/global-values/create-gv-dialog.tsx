import { useState } from "react"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"
import { useCreateGlobalValues } from "@/hooks/use-global-values"
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
import { Plus } from "lucide-react"

export function CreateGlobalValuesDialog() {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [approvalCondition, setApprovalCondition] = useState("")
  const createGV = useCreateGlobalValues()
  const navigate = useNavigate()

  // The entry's admin role "<name>_gv_group_admin" is auto-created and is the
  // only role that exists at creation. Blank defaults to "1 x <name>_gv_group_admin".
  const adminRole = `${name.trim()}_gv_group_admin`
  const approvalValidation = validateApprovalCondition(
    approvalCondition.trim() || `1 x ${adminRole}`,
    [],
    [adminRole],
  )

  function reset() {
    setName("")
    setApprovalCondition("")
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim() || !approvalValidation.valid) return
    createGV.mutate(
      {
        name: name.trim(),
        payload: {},
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
              placeholder={name.trim() ? `1 x ${adminRole}` : "1 x <name>_gv_group_admin"}
            />
            {approvalValidation.valid ? (
              <p className="text-xs text-muted-foreground">
                Defaults to <code>1 x {adminRole}</code>. Add other roles on the
                Roles page later, then update this condition.
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
            <Button variant="outline" type="button" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={createGV.isPending || !approvalValidation.valid}
            >
              {createGV.isPending ? "Creating..." : "Create"}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
