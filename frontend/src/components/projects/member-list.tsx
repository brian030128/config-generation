import { useState } from "react"
import { toast } from "sonner"
import type { ProjectMember } from "@/api/types"
import { useProjectMembers, useRemoveProjectMember } from "@/hooks/use-projects"
import { useAuth } from "@/lib/auth"
import { formatRelativeTime, getApiErrorMessage } from "@/lib/utils"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { AddMemberDialog } from "./add-member-dialog"
import { Trash2, Users } from "lucide-react"

export function MemberList({ projectName }: { projectName: string }) {
  const { data, isLoading, error } = useProjectMembers(projectName)
  const { user } = useAuth()
  const members = data?.items ?? []

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium">Members</h3>
        <AddMemberDialog projectName={projectName} />
      </div>

      {isLoading && (
        <p className="text-sm text-muted-foreground">Loading members...</p>
      )}

      {error && (
        <p className="text-sm text-destructive">
          Failed to load members: {getApiErrorMessage(error)}
        </p>
      )}

      {!isLoading && !error && members.length === 0 && (
        <div className="flex min-h-56 items-center justify-center rounded-lg border border-dashed bg-card/40 p-8 text-center">
          <div className="mx-auto flex max-w-sm flex-col items-center gap-3">
            <div className="flex size-11 items-center justify-center rounded-lg border bg-background text-muted-foreground">
              <Users className="h-5 w-5" />
            </div>
            <div className="space-y-1">
              <h4 className="text-base font-semibold">No members yet</h4>
              <p className="text-sm text-muted-foreground">
                Add a member to grant them access to this project.
              </p>
            </div>
          </div>
        </div>
      )}

      <div className="space-y-2">
        {members.map((member) => (
          <div
            key={member.user_id}
            className="flex items-center justify-between rounded-lg border px-4 py-3"
          >
            <div className="flex items-center gap-3">
              <span className="font-medium">
                {member.display_name || member.username}
              </span>
              <span className="text-xs text-muted-foreground">
                @{member.username}
              </span>
              {user?.id === member.user_id && (
                <Badge variant="secondary">You</Badge>
              )}
            </div>
            <div className="flex items-center gap-3">
              <span className="text-xs text-muted-foreground">
                added {formatRelativeTime(member.added_at)}
              </span>
              <RemoveMemberButton projectName={projectName} member={member} />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

function RemoveMemberButton({
  projectName,
  member,
}: {
  projectName: string
  member: ProjectMember
}) {
  const [open, setOpen] = useState(false)
  const removeMember = useRemoveProjectMember(projectName)
  const displayName = member.display_name || member.username

  function handleRemove() {
    removeMember.mutate(member.user_id, {
      onSuccess: () => {
        setOpen(false)
        toast.success(`Removed ${displayName}`)
      },
      onError: (err) => {
        toast.error("Failed to remove member", {
          description: getApiErrorMessage(err),
        })
      },
    })
  }

  return (
    <>
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label={`Remove ${displayName}`}
        onClick={() => setOpen(true)}
      >
        <Trash2 />
      </Button>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Remove {displayName}?</DialogTitle>
            <DialogDescription>
              They lose access to this project. Any project-scoped roles they
              hold are revoked too. You can add them back later.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setOpen(false)}
              disabled={removeMember.isPending}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleRemove}
              disabled={removeMember.isPending}
            >
              {removeMember.isPending ? "Removing..." : "Remove member"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
