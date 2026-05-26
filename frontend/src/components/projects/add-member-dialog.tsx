import { useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { Input } from "@/components/ui/input"
import { UserPlus } from "lucide-react"
import { useAddProjectMember } from "@/hooks/use-projects"
import { getApiErrorMessage } from "@/lib/utils"

export function AddMemberDialog({ projectName }: { projectName: string }) {
  const [open, setOpen] = useState(false)
  const [userId, setUserId] = useState("")
  const addMember = useAddProjectMember(projectName)

  const parsedId = Number(userId)
  const valid = Number.isInteger(parsedId) && parsedId > 0

  function handleAdd() {
    if (!valid) return
    addMember.mutate(parsedId, {
      onSuccess: () => {
        setOpen(false)
        setUserId("")
        toast.success("Member added")
      },
      onError: (err) => {
        toast.error("Failed to add member", {
          description: getApiErrorMessage(err),
        })
      },
    })
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button size="sm">
          <UserPlus className="mr-2 h-4 w-4" />
          Add Member
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add Member</DialogTitle>
          <DialogDescription>
            Grant a user read access to this project. They will see the project
            but not its templates or values until granted further permissions.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>User ID</Label>
            <Input
              type="number"
              value={userId}
              onChange={(e) => setUserId(e.target.value)}
              placeholder="e.g. 42"
              onKeyDown={(e) => {
                if (e.key === "Enter") handleAdd()
              }}
            />
          </div>
          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleAdd} disabled={!valid || addMember.isPending}>
              {addMember.isPending ? "Adding..." : "Add Member"}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
