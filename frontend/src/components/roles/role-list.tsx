import { useState } from "react"
import { toast } from "sonner"
import type { Role, UserRole } from "@/api/types"
import { useAuth } from "@/lib/auth"
import {
  useDeleteRole,
  useRemoveRoleMember,
  useRoles,
} from "@/hooks/use-roles"
import {
  gvAtomsToCapabilities,
  gvCapabilityLabels,
  inferTarget,
  projectAtomsToCapabilities,
  projectCapabilityLabels,
} from "@/lib/role-permissions"
import { getApiErrorMessage } from "@/lib/utils"
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
import { RoleEditorDialog } from "./role-editor-dialog"
import { AssignRoleDialog } from "./assign-role-dialog"
import { Pencil, Plus, Shield, Trash2, UserPlus, X } from "lucide-react"

function describeRole(role: Role): { target: string; labels: string[] } {
  const perms = role.permissions ?? []
  const t = inferTarget(perms)
  if (t?.kind === "project") {
    return {
      target: `Project: ${t.project}`,
      labels: projectCapabilityLabels(
        projectAtomsToCapabilities(perms, t.project).caps,
      ),
    }
  }
  if (t?.kind === "global-values") {
    return {
      target: `Global values: ${t.name}`,
      labels: gvCapabilityLabels(gvAtomsToCapabilities(perms, t.name).caps),
    }
  }
  return { target: "No scope", labels: [] }
}

export function RoleList() {
  const { data, isLoading, error } = useRoles()
  const { user } = useAuth()
  const roles = data?.items ?? []
  const canManage = data?.viewer_can_manage ?? false

  const [createOpen, setCreateOpen] = useState(false)
  const [editRole, setEditRole] = useState<Role | null>(null)
  const [assignRole, setAssignRole] = useState<Role | null>(null)

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold">Roles</h1>
          <p className="text-sm text-muted-foreground">
            Global, named bundles of permissions. Reference them by name in a
            project's or entry's approval condition.
          </p>
        </div>
        {canManage ? (
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="mr-2 h-4 w-4" />
            Create Role
          </Button>
        ) : (
          <p className="text-xs text-muted-foreground">
            Only superusers can manage roles.
          </p>
        )}
      </div>

      {isLoading && (
        <p className="text-sm text-muted-foreground">Loading roles...</p>
      )}

      {error && (
        <p className="text-sm text-destructive">
          Failed to load roles: {getApiErrorMessage(error)}
        </p>
      )}

      {!isLoading && !error && roles.length === 0 && (
        <div className="flex min-h-56 items-center justify-center rounded-lg border border-dashed bg-card/40 p-8 text-center">
          <div className="mx-auto flex max-w-sm flex-col items-center gap-3">
            <div className="flex size-11 items-center justify-center rounded-lg border bg-background text-muted-foreground">
              <Shield className="h-5 w-5" />
            </div>
            <div className="space-y-1">
              <h4 className="text-base font-semibold">No roles yet</h4>
              <p className="text-sm text-muted-foreground">
                Create a role to grant a bundle of permissions and use it in an
                approval condition.
              </p>
            </div>
          </div>
        </div>
      )}

      <div className="space-y-3">
        {roles.map((role) => {
          const { target, labels } = describeRole(role)
          const members = role.members ?? []
          return (
            <div key={role.id} className="space-y-3 rounded-lg border px-4 py-3">
              <div className="flex items-start justify-between gap-3">
                <div className="space-y-1.5">
                  <div className="flex items-center gap-2">
                    <Shield className="h-4 w-4 text-muted-foreground" />
                    <span className="font-medium">{role.name}</span>
                    {role.is_auto_created && (
                      <Badge variant="secondary">built-in</Badge>
                    )}
                    <span className="text-xs text-muted-foreground">{target}</span>
                  </div>
                  {labels.length > 0 ? (
                    <div className="flex flex-wrap gap-1">
                      {labels.map((l) => (
                        <Badge key={l} variant="outline" className="font-normal">
                          {l}
                        </Badge>
                      ))}
                    </div>
                  ) : (
                    <p className="text-xs text-muted-foreground">
                      No permissions
                    </p>
                  )}
                </div>
                {canManage && (
                  <div className="flex shrink-0 items-center gap-1">
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label={`Assign a user to ${role.name}`}
                      onClick={() => setAssignRole(role)}
                    >
                      <UserPlus />
                    </Button>
                    {!role.is_auto_created && (
                      <>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label={`Edit ${role.name}`}
                          onClick={() => setEditRole(role)}
                        >
                          <Pencil />
                        </Button>
                        <DeleteRoleButton role={role} />
                      </>
                    )}
                  </div>
                )}
              </div>

              <div className="flex flex-wrap gap-2">
                {members.length === 0 ? (
                  <span className="text-xs text-muted-foreground">
                    No members assigned
                  </span>
                ) : (
                  members.map((m) => (
                    <MemberChip
                      key={m.user_id}
                      role={role}
                      member={m}
                      canManage={canManage}
                      isYou={user?.id === m.user_id}
                    />
                  ))
                )}
              </div>
            </div>
          )
        })}
      </div>

      <RoleEditorDialog open={createOpen} onOpenChange={setCreateOpen} />
      {editRole && (
        <RoleEditorDialog
          role={editRole}
          open={!!editRole}
          onOpenChange={(o) => !o && setEditRole(null)}
        />
      )}
      {assignRole && (
        <AssignRoleDialog
          role={assignRole}
          open={!!assignRole}
          onOpenChange={(o) => !o && setAssignRole(null)}
        />
      )}
    </div>
  )
}

function MemberChip({
  role,
  member,
  canManage,
  isYou,
}: {
  role: Role
  member: UserRole
  canManage: boolean
  isYou: boolean
}) {
  const remove = useRemoveRoleMember()
  const name =
    member.display_name || member.username || `user ${member.user_id}`

  return (
    <span className="inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs">
      {name}
      {isYou && <span className="text-muted-foreground">(you)</span>}
      {canManage && (
        <button
          type="button"
          aria-label={`Remove ${name} from ${role.name}`}
          disabled={remove.isPending}
          onClick={() =>
            remove.mutate(
              { roleId: role.id, userId: member.user_id },
              {
                onSuccess: () =>
                  toast.success(`Removed ${name} from ${role.name}`),
                onError: (err) =>
                  toast.error("Failed to remove member", {
                    description: getApiErrorMessage(err),
                  }),
              },
            )
          }
          className="text-muted-foreground hover:text-foreground disabled:opacity-50"
        >
          <X className="h-3 w-3" />
        </button>
      )}
    </span>
  )
}

function DeleteRoleButton({ role }: { role: Role }) {
  const [open, setOpen] = useState(false)
  const del = useDeleteRole()

  return (
    <>
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label={`Delete ${role.name}`}
        onClick={() => setOpen(true)}
      >
        <Trash2 />
      </Button>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete {role.name}?</DialogTitle>
            <DialogDescription>
              Members lose this role's permissions. If an approval condition
              references this role, update it too, or pull requests can't be
              approved. This can't be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setOpen(false)}
              disabled={del.isPending}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={del.isPending}
              onClick={() =>
                del.mutate(role.id, {
                  onSuccess: () => {
                    setOpen(false)
                    toast.success(`Deleted ${role.name}`)
                  },
                  onError: (err) =>
                    toast.error("Failed to delete role", {
                      description: getApiErrorMessage(err),
                    }),
                })
              }
            >
              {del.isPending ? "Deleting..." : "Delete role"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
