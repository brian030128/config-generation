import { useEffect, useState } from "react"
import { toast } from "sonner"
import type { PermissionAtomInput, Role } from "@/api/types"
import { useEnvironments } from "@/hooks/use-environments"
import { useProjects } from "@/hooks/use-projects"
import { useGlobalValues } from "@/hooks/use-global-values"
import { useCreateRole, useEditRolePermissions } from "@/hooks/use-roles"
import {
  emptyProjectCapabilities,
  gvAtomsToCapabilities,
  gvCapabilitiesToAtoms,
  inferTarget,
  projectAtomsToCapabilities,
  projectCapabilitiesToAtoms,
  type GvRoleCapabilities,
  type ProjectRoleCapabilities,
  type RoleTarget,
} from "@/lib/role-permissions"
import { cn, getApiErrorMessage } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

function saveButtonLabel({
  pending,
  isEdit,
}: {
  pending: boolean
  isEdit: boolean
}): string {
  if (pending) return "Saving..."
  return isEdit ? "Save changes" : "Create role"
}
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { CapabilityEditor } from "@/components/permissions/capability-editor"

function emptyGvCaps(): GvRoleCapabilities {
  return { read: false, write: false, manage_roles: false, delete: false }
}

function targetToValue(t: RoleTarget): string {
  return t.kind === "project" ? `p:${t.project}` : `g:${t.name}`
}
function valueToTarget(v: string): RoleTarget | null {
  if (v.startsWith("p:")) return { kind: "project", project: v.slice(2) }
  if (v.startsWith("g:")) return { kind: "global-values", name: v.slice(2) }
  return null
}

function CheckboxRow({
  checked,
  onChange,
  label,
  description,
  disabled = false,
}: Readonly<{
  checked: boolean
  onChange: () => void
  label: string
  description: string
  disabled?: boolean
}>) {
  return (
    <label
      className={cn(
        "flex cursor-pointer items-start gap-3 rounded-md border px-3 py-2 transition-colors hover:bg-accent/50",
        disabled && "opacity-60",
      )}
    >
      <input
        type="checkbox"
        aria-label={label}
        className="mt-1 h-4 w-4"
        checked={checked}
        disabled={disabled}
        onChange={onChange}
      />
      <span className="space-y-0.5">
        <span className="text-sm font-medium leading-none cursor-pointer">
          {label}
        </span>
        <span className="block text-xs text-muted-foreground">
          {description}
        </span>
      </span>
    </label>
  )
}

// Toggling a GV capability. Write implies read.
function toggleGvCap(
  c: GvRoleCapabilities,
  key: keyof GvRoleCapabilities,
): GvRoleCapabilities {
  const next = { ...c, [key]: !c[key] }
  if (key === "write" && next.write) next.read = true
  return next
}

const PROJECT_EXTRA_CAPS: {
  key: "manage_environments" | "manage_members_roles" | "delete_project"
  label: string
  description: string
}[] = [
  {
    key: "manage_environments",
    label: "Manage environments",
    description: "Create and delete environments across the project.",
  },
  {
    key: "manage_members_roles",
    label: "Manage members & roles",
    description: "Add/remove members and manage project access (grant).",
  },
  {
    key: "delete_project",
    label: "Delete project",
    description: "Permanently delete this project.",
  },
]

const GV_CAPS: {
  key: keyof GvRoleCapabilities
  label: string
  description: string
}[] = [
  { key: "read", label: "Read values", description: "View this entry and its version history." },
  { key: "write", label: "Write values", description: "Append new versions (implies read)." },
  { key: "manage_roles", label: "Manage roles", description: "Manage access for this entry (grant)." },
  { key: "delete", label: "Delete entry", description: "Delete this global values entry." },
]

// RoleEditorDialog creates a new global role (when `role` is undefined) or edits
// an existing role's permissions. The superuser picks a single target (a project
// or a GV entry) and the friendly capability editor builds the atoms for it; any
// atoms targeting other scopes are preserved untouched.
export function RoleEditorDialog({
  role,
  open,
  onOpenChange,
}: Readonly<{
  role?: Role
  open: boolean
  onOpenChange: (open: boolean) => void
}>) {
  const isEdit = !!role

  const projectsQuery = useProjects()
  const gvQuery = useGlobalValues()

  const createRole = useCreateRole()
  const editPerms = useEditRolePermissions()

  const [name, setName] = useState("")
  const [target, setTarget] = useState<RoleTarget | null>(null)
  const [projectCaps, setProjectCaps] = useState<ProjectRoleCapabilities>(
    emptyProjectCapabilities(),
  )
  const [gvCaps, setGvCaps] = useState<GvRoleCapabilities>(emptyGvCaps())
  const [extraAtoms, setExtraAtoms] = useState<PermissionAtomInput[]>([])

  const projectName = target?.kind === "project" ? target.project : ""
  const envsQuery = useEnvironments(projectName)
  const environments = envsQuery.data?.items ?? []

  // Seed the draft from the existing role (or reset for create) when opening.
  useEffect(() => {
    if (!open) return
    setName(role?.name ?? "")
    const perms = role?.permissions ?? []
    const inferred = inferTarget(perms)
    setTarget(inferred)
    if (inferred?.kind === "project") {
      const { caps, extraAtoms: extra } = projectAtomsToCapabilities(
        perms,
        inferred.project,
      )
      setProjectCaps(caps)
      setGvCaps(emptyGvCaps())
      setExtraAtoms(extra)
    } else if (inferred?.kind === "global-values") {
      const { caps, extraAtoms: extra } = gvAtomsToCapabilities(
        perms,
        inferred.name,
      )
      setGvCaps(caps)
      setProjectCaps(emptyProjectCapabilities())
      setExtraAtoms(extra)
    } else {
      setProjectCaps(emptyProjectCapabilities())
      setGvCaps(emptyGvCaps())
      setExtraAtoms(perms.map((p) => ({ ...p })))
    }
  }, [open, role])

  const pending = createRole.isPending || editPerms.isPending

  function buildPermissions(): PermissionAtomInput[] {
    if (target?.kind === "project") {
      return projectCapabilitiesToAtoms(projectCaps, target.project, extraAtoms)
    }
    if (target?.kind === "global-values") {
      return gvCapabilitiesToAtoms(gvCaps, target.name, extraAtoms)
    }
    return extraAtoms
  }

  function handleSave() {
    const trimmed = name.trim()
    if (!isEdit && !trimmed) return
    const permissions = buildPermissions()

    if (isEdit && role) {
      editPerms.mutate(
        { roleId: role.id, permissions },
        {
          onSuccess: () => {
            toast.success(`Updated ${role.name}`)
            onOpenChange(false)
          },
          onError: (err) =>
            toast.error("Failed to update role", {
              description: getApiErrorMessage(err),
            }),
        },
      )
    } else {
      createRole.mutate(
        { name: trimmed, permissions },
        {
          onSuccess: () => {
            toast.success(`Created role "${trimmed}"`)
            onOpenChange(false)
          },
          onError: (err) =>
            toast.error("Failed to create role", {
              description: getApiErrorMessage(err),
            }),
        },
      )
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{isEdit ? `Edit ${role?.name}` : "Create role"}</DialogTitle>
          <DialogDescription>
            A role is global. Pick what it applies to, then choose its
            capabilities. Assign members to it afterwards.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {!isEdit && (
            <div className="space-y-2">
              <Label htmlFor="role-name">Name</Label>
              <Input
                id="role-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="release_manager"
                autoFocus
              />
            </div>
          )}

          <div className="space-y-2">
            <Label>Applies to</Label>
            <Select
              value={target ? targetToValue(target) : ""}
              onValueChange={(v) => setTarget(valueToTarget(v))}
            >
              <SelectTrigger>
                <SelectValue placeholder="Select a project or global values entry" />
              </SelectTrigger>
              <SelectContent>
                {(projectsQuery.data?.items ?? []).map((p) => (
                  <SelectItem key={`p:${p.name}`} value={`p:${p.name}`}>
                    Project: {p.name}
                  </SelectItem>
                ))}
                {(gvQuery.data?.items ?? []).map((g) => (
                  <SelectItem key={`g:${g.name}`} value={`g:${g.name}`}>
                    Global values: {g.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {extraAtoms.length > 0 && (
              <p className="text-xs text-muted-foreground">
                This role also has {extraAtoms.length} permission(s) on other
                scopes, which are preserved when you save.
              </p>
            )}
          </div>

          {target?.kind === "project" && (
            <>
              <CheckboxRow
                label="Read project"
                description="View the project. Required for any project access — granting it unlocks the permissions below."
                checked={projectCaps.read_project}
                onChange={() =>
                  setProjectCaps((c) =>
                    // Re-locking clears the now-unusable dependent capabilities.
                    c.read_project
                      ? emptyProjectCapabilities()
                      : { ...c, read_project: true },
                  )
                }
              />
              {!projectCaps.read_project && (
                <p className="text-xs text-muted-foreground">
                  Grant <span className="font-medium">Read project</span> to
                  enable the permissions below.
                </p>
              )}
              <CapabilityEditor
                environments={environments}
                templates={projectCaps.templates}
                onTemplatesChange={(t) =>
                  setProjectCaps((c) => ({ ...c, templates: t }))
                }
                envLevels={projectCaps.envLevels}
                onEnvLevelChange={(env, level) =>
                  setProjectCaps((c) => ({
                    ...c,
                    envLevels: { ...c.envLevels, [env]: level },
                  }))
                }
                disabled={!projectCaps.read_project}
              />
              <div className="space-y-3">
                {PROJECT_EXTRA_CAPS.map((cap) => (
                  <CheckboxRow
                    key={cap.key}
                    label={cap.label}
                    description={cap.description}
                    checked={projectCaps[cap.key]}
                    disabled={!projectCaps.read_project}
                    onChange={() =>
                      setProjectCaps((c) => ({ ...c, [cap.key]: !c[cap.key] }))
                    }
                  />
                ))}
              </div>
            </>
          )}

          {target?.kind === "global-values" && (
            <div className="space-y-3">
              {GV_CAPS.map((cap) => {
                // Read is implied by write: show it checked and locked when write is on.
                const lockedByWrite = cap.key === "read" && gvCaps.write
                const checked =
                  cap.key === "read" ? gvCaps.read || gvCaps.write : gvCaps[cap.key]
                return (
                  <CheckboxRow
                    key={cap.key}
                    label={cap.label}
                    description={cap.description}
                    checked={checked}
                    disabled={lockedByWrite}
                    onChange={() => setGvCaps((c) => toggleGvCap(c, cap.key))}
                  />
                )
              })}
            </div>
          )}

          {!target && (
            <p className="text-sm text-muted-foreground">
              Select what this role applies to to choose its capabilities.
            </p>
          )}
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={pending}
          >
            Cancel
          </Button>
          <Button onClick={handleSave} disabled={pending || (!isEdit && !name.trim())}>
            {saveButtonLabel({ pending, isEdit })}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
