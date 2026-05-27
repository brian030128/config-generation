import { useEffect, useState } from "react"
import { toast } from "sonner"
import type { Role, User } from "@/api/types"
import { useAssignRole } from "@/hooks/use-roles"
import { useProjectMembers } from "@/hooks/use-projects"
import { useUserSearch } from "@/hooks/use-users"
import { inferTarget, roleGrantsProjectRead } from "@/lib/role-permissions"
import { getApiErrorMessage } from "@/lib/utils"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"

function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), delayMs)
    return () => clearTimeout(id)
  }, [value, delayMs])
  return debounced
}

// AssignRoleDialog is a controlled user-picker that assigns a user to a role.
//
// A project-scoped role's permissions are unusable unless the user can read the
// project. read:project comes only from membership (or an explicit read:project
// atom). So if the role targets a project and does NOT itself grant read:project,
// only members of that project may be assigned — others are blocked with a hint.
export function AssignRoleDialog({
  role,
  open,
  onOpenChange,
}: {
  role: Role
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const [search, setSearch] = useState("")
  const debouncedSearch = useDebouncedValue(search, 250)
  const assign = useAssignRole()

  const target = inferTarget(role.permissions ?? [])
  const roleHasProjectRead =
    target?.kind === "project" &&
    roleGrantsProjectRead(role.permissions ?? [], target.project)
  // Only fetch members (and require membership) when gating is needed.
  const gatingProject =
    target?.kind === "project" && !roleHasProjectRead ? target.project : ""
  const membersQuery = useProjectMembers(gatingProject)
  const memberIds = new Set(
    (membersQuery.data?.items ?? []).map((m) => m.user_id),
  )
  const requiresMembership = !!gatingProject

  const { data, isLoading, error } = useUserSearch(debouncedSearch, open)
  const existing = new Set((role.members ?? []).map((m) => m.user_id))
  const results = (data?.items ?? []).filter((u) => !existing.has(u.id))

  function handleAdd(user: User) {
    if (requiresMembership && !memberIds.has(user.id)) return
    assign.mutate(
      { roleId: role.id, userId: user.id },
      {
        onSuccess: () => {
          onOpenChange(false)
          setSearch("")
          toast.success(
            `Assigned ${user.display_name || user.username} to ${role.name}`,
          )
        },
        onError: (err) =>
          toast.error("Failed to assign user", {
            description: getApiErrorMessage(err),
          }),
      },
    )
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next)
        if (!next) setSearch("")
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Assign to {role.name}</DialogTitle>
          <DialogDescription>
            Search for a user to grant this role's permissions.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          {requiresMembership && (
            <p className="rounded-md border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-400">
              This role grants permissions on{" "}
              <span className="font-medium">{gatingProject}</span> but not
              project read access. Only members of {gatingProject} can be
              assigned — add the user to the project first, or include "Read
              project" in the role.
            </p>
          )}

          <Input
            autoFocus
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by name or username"
          />

          <div className="min-h-40 max-h-72 overflow-y-auto rounded-md border">
            {isLoading && (
              <p className="p-3 text-sm text-muted-foreground">Searching...</p>
            )}

            {error && (
              <p className="p-3 text-sm text-destructive">
                {getApiErrorMessage(error)}
              </p>
            )}

            {!isLoading && !error && results.length === 0 && (
              <p className="p-3 text-sm text-muted-foreground">
                {search.trim()
                  ? "No matching users."
                  : "No users available to assign."}
              </p>
            )}

            {results.map((user) => {
              const blocked = requiresMembership && !memberIds.has(user.id)
              return (
                <button
                  key={user.id}
                  type="button"
                  disabled={assign.isPending || blocked}
                  onClick={() => handleAdd(user)}
                  title={
                    blocked
                      ? `Not a member of ${gatingProject}`
                      : undefined
                  }
                  className="flex w-full items-center gap-3 px-3 py-2 text-left transition-colors hover:bg-accent/50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <span className="font-medium">
                    {user.display_name || user.username}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    @{user.username}
                  </span>
                  {blocked && (
                    <span className="ml-auto text-xs text-muted-foreground">
                      not a member
                    </span>
                  )}
                </button>
              )
            })}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
