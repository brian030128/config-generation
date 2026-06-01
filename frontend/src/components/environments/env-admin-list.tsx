import { useState } from "react"
import { toast } from "sonner"
import { ShieldCheck, Trash2 } from "lucide-react"
import type { EnvAdmin } from "@/api/types"
import { useEnvAdmins, useRemoveEnvAdmin } from "@/hooks/use-environments"
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
import { AddEnvAdminDialog } from "./add-env-admin-dialog"

export function EnvAdminList({
  projectName,
  envName,
}: Readonly<{
  projectName: string
  envName: string
}>) {
  const { data, isLoading, error } = useEnvAdmins(projectName, envName)
  const { user } = useAuth()
  const admins = data?.items ?? []

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <ShieldCheck className="h-4 w-4 text-muted-foreground" />
          <h3 className="text-lg font-medium">Environment admins</h3>
        </div>
        <AddEnvAdminDialog
          projectName={projectName}
          envName={envName}
          existingAdminIds={admins.map((a) => a.user_id)}
        />
      </div>

      {isLoading && (
        <p className="text-sm text-muted-foreground">Loading admins...</p>
      )}

      {error && (
        <p className="text-sm text-destructive">
          Failed to load admins: {getApiErrorMessage(error)}
        </p>
      )}

      {!isLoading && !error && admins.length === 0 && (
        <p className="text-sm text-muted-foreground">
          No env admins yet. Project admins can always manage this environment.
        </p>
      )}

      <div className="space-y-2">
        {admins.map((admin) => (
          <div
            key={admin.user_id}
            className="flex items-center justify-between rounded-lg border px-4 py-3"
          >
            <div className="flex items-center gap-3">
              <span className="font-medium">
                {admin.display_name || admin.username}
              </span>
              <span className="text-xs text-muted-foreground">
                @{admin.username}
              </span>
              {user?.id === admin.user_id && (
                <Badge variant="secondary">You</Badge>
              )}
            </div>
            <div className="flex items-center gap-3">
              <span className="text-xs text-muted-foreground">
                granted {formatRelativeTime(admin.granted_at)}
              </span>
              <RemoveEnvAdminButton
                projectName={projectName}
                envName={envName}
                admin={admin}
              />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

function RemoveEnvAdminButton({
  projectName,
  envName,
  admin,
}: Readonly<{
  projectName: string
  envName: string
  admin: EnvAdmin
}>) {
  const [open, setOpen] = useState(false)
  const removeAdmin = useRemoveEnvAdmin(projectName, envName)
  const displayName = admin.display_name || admin.username

  function handleRemove() {
    removeAdmin.mutate(admin.user_id, {
      onSuccess: () => {
        setOpen(false)
        toast.success(`Removed ${displayName}`)
      },
      onError: (err) => {
        toast.error("Failed to remove env admin", {
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
            <DialogTitle>Remove {displayName} as env admin?</DialogTitle>
            <DialogDescription>
              They lose admin control of {envName}. You can grant it again later.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setOpen(false)}
              disabled={removeAdmin.isPending}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleRemove}
              disabled={removeAdmin.isPending}
            >
              {removeAdmin.isPending ? "Removing..." : "Remove admin"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
