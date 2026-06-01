import type { PermissionAtomInput, RolePermission } from "@/api/types"

// Translation between a role's raw permission atoms and the friendly,
// capability-based shapes the editor presents. This mirrors the server-side
// mapping in backend/handlers/member_permissions.go, extended for roles with
// manage-members/roles (grant), delete-project, and manage-environments.
//
// Any atom that does not map to a known capability is preserved verbatim
// (returned as `extraAtoms`) and re-emitted on save — the edit endpoint replaces
// *all* atoms, so dropping unknowns would silently lose permissions.

const WILDCARD = "*"

export type EnvLevel = "none" | "read" | "write"

export interface TemplateCaps {
  read_templates: boolean
  write_templates: boolean
  delete_templates: boolean
}

export interface ProjectRoleCapabilities {
  // read_project grants read:project(p) — visibility of the project itself.
  // Without it (and without membership) other project permissions are unusable.
  read_project: boolean
  templates: TemplateCaps
  envLevels: Record<string, EnvLevel>
  manage_members_roles: boolean
  delete_project: boolean
  manage_environments: boolean
}

export interface GvRoleCapabilities {
  read: boolean
  write: boolean
  manage_roles: boolean
  delete: boolean
}

// RoleTarget is the single project / global-values entry a role's capabilities
// are edited against. A role is global; this just scopes the friendly editor.
export type RoleTarget =
  | { kind: "project"; project: string }
  | { kind: "global-values"; name: string }

// inferTarget guesses a role's target from its permission atoms, to seed the
// editor when editing. Project atoms carry key_project; GV atoms are
// global_values with key_name.
export function inferTarget(perms: RolePermission[]): RoleTarget | null {
  for (const p of perms) {
    if (p.key_project) return { kind: "project", project: p.key_project }
    if (p.resource === "global_values" && p.key_name)
      return { kind: "global-values", name: p.key_name }
  }
  return null
}

function toInput(p: RolePermission): PermissionAtomInput {
  return {
    action: p.action,
    resource: p.resource,
    key_project: p.key_project,
    key_env: p.key_env,
    key_name: p.key_name,
  }
}

function atom(
  action: string,
  resource: string,
  keys: { key_project?: string; key_env?: string; key_name?: string } = {},
): PermissionAtomInput {
  return {
    action,
    resource,
    key_project: keys.key_project ?? null,
    key_env: keys.key_env ?? null,
    key_name: keys.key_name ?? null,
  }
}

export function emptyProjectCapabilities(): ProjectRoleCapabilities {
  return {
    read_project: false,
    templates: {
      read_templates: false,
      write_templates: false,
      delete_templates: false,
    },
    envLevels: {},
    manage_members_roles: false,
    delete_project: false,
    manage_environments: false,
  }
}

// roleGrantsProjectRead reports whether a role's atoms include read:project(p).
export function roleGrantsProjectRead(
  perms: RolePermission[],
  project: string,
): boolean {
  return perms.some(
    (p) =>
      p.action === "read" &&
      p.resource === "project" &&
      p.key_project === project,
  )
}

// applyTemplateAtom folds a project_templates atom into caps (or extras).
function applyTemplateAtom(
  p: RolePermission,
  caps: ProjectRoleCapabilities,
  extraAtoms: PermissionAtomInput[],
): void {
  if (p.action === "read") caps.templates.read_templates = true
  else if (p.action === "write") caps.templates.write_templates = true
  else if (p.action === "delete") caps.templates.delete_templates = true
  else extraAtoms.push(toInput(p))
}

// applyProjectValuesAtom folds a project_values atom into envLevels,
// manage_environments, or extras.
function applyProjectValuesAtom(
  p: RolePermission,
  caps: ProjectRoleCapabilities,
  extraAtoms: PermissionAtomInput[],
): void {
  if (p.key_env === WILDCARD) {
    if (p.action === "delete") caps.manage_environments = true
    else extraAtoms.push(toInput(p))
    return
  }
  if (!p.key_env) {
    extraAtoms.push(toInput(p))
    return
  }
  if (p.action === "write") {
    caps.envLevels[p.key_env] = "write"
  } else if (p.action === "read") {
    if (caps.envLevels[p.key_env] !== "write") {
      caps.envLevels[p.key_env] = "read"
    }
  } else {
    extraAtoms.push(toInput(p))
  }
}

// applyEnvValuesAtom routes env_values atoms; per-env creates are deferred so
// they can be reconciled with envLevels after the main pass.
function applyEnvValuesAtom(
  p: RolePermission,
  caps: ProjectRoleCapabilities,
  pendingEnvCreates: RolePermission[],
  extraAtoms: PermissionAtomInput[],
): void {
  if (p.key_env === WILDCARD) {
    if (p.action === "create") caps.manage_environments = true
    else extraAtoms.push(toInput(p))
    return
  }
  if (p.action === "create" && p.key_env) {
    pendingEnvCreates.push(p)
    return
  }
  extraAtoms.push(toInput(p))
}

// applyScopedProjectAtom folds non-resource-specific project atoms (read/
// delete on the project itself, or any grant action) into caps. Returns true
// when the atom was consumed.
function applyScopedProjectAtom(
  p: RolePermission,
  caps: ProjectRoleCapabilities,
): boolean {
  if (p.resource === "project" && p.action === "read") {
    caps.read_project = true
    return true
  }
  if (p.resource === "project" && p.action === "delete") {
    caps.delete_project = true
    return true
  }
  if (p.action === "grant") {
    caps.manage_members_roles = true
    return true
  }
  return false
}

export function projectAtomsToCapabilities(
  perms: RolePermission[],
  projectName: string,
): { caps: ProjectRoleCapabilities; extraAtoms: PermissionAtomInput[] } {
  const caps = emptyProjectCapabilities()
  const extraAtoms: PermissionAtomInput[] = []
  // create:env_values(p, env) is the bootstrap atom paired with write access;
  // resolve these against envLevels after the main pass.
  const pendingEnvCreates: RolePermission[] = []

  for (const p of perms) {
    if (p.key_project !== projectName) {
      extraAtoms.push(toInput(p))
      continue
    }
    if (p.resource === "project_templates") {
      applyTemplateAtom(p, caps, extraAtoms)
    } else if (p.resource === "project_values") {
      applyProjectValuesAtom(p, caps, extraAtoms)
    } else if (p.resource === "env_values") {
      applyEnvValuesAtom(p, caps, pendingEnvCreates, extraAtoms)
    } else if (!applyScopedProjectAtom(p, caps)) {
      extraAtoms.push(toInput(p))
    }
  }

  // A per-env create that pairs with write access is implied (re-emitted on
  // save); one without write access is unusual, so preserve it.
  for (const p of pendingEnvCreates) {
    if (caps.envLevels[p.key_env as string] !== "write") {
      extraAtoms.push(toInput(p))
    }
  }

  return { caps, extraAtoms }
}

export function projectCapabilitiesToAtoms(
  caps: ProjectRoleCapabilities,
  projectName: string,
  extraAtoms: PermissionAtomInput[] = [],
): PermissionAtomInput[] {
  const p = projectName
  const atoms: PermissionAtomInput[] = []

  if (caps.read_project) atoms.push(atom("read", "project", { key_project: p }))
  if (caps.templates.read_templates)
    atoms.push(atom("read", "project_templates", { key_project: p }))
  if (caps.templates.write_templates)
    atoms.push(atom("write", "project_templates", { key_project: p }))
  if (caps.templates.delete_templates)
    atoms.push(atom("delete", "project_templates", { key_project: p }))

  for (const [env, level] of Object.entries(caps.envLevels)) {
    if (level === "read") {
      atoms.push(atom("read", "project_values", { key_project: p, key_env: env }))
    } else if (level === "write") {
      atoms.push(
        atom("write", "project_values", { key_project: p, key_env: env }),
        atom("create", "env_values", { key_project: p, key_env: env }),
      )
    }
  }

  if (caps.manage_members_roles) atoms.push(atom("grant", "", { key_project: p }))
  if (caps.delete_project) atoms.push(atom("delete", "project", { key_project: p }))
  if (caps.manage_environments) {
    atoms.push(
      atom("create", "env_values", { key_project: p, key_env: WILDCARD }),
      atom("delete", "project_values", { key_project: p, key_env: WILDCARD }),
    )
  }

  return [...atoms, ...extraAtoms]
}

export function gvAtomsToCapabilities(
  perms: RolePermission[],
  name: string,
): { caps: GvRoleCapabilities; extraAtoms: PermissionAtomInput[] } {
  const caps: GvRoleCapabilities = {
    read: false,
    write: false,
    manage_roles: false,
    delete: false,
  }
  const extraAtoms: PermissionAtomInput[] = []

  for (const p of perms) {
    if (p.key_name === name && p.resource === "global_values") {
      if (p.action === "read") caps.read = true
      else if (p.action === "write") caps.write = true
      else if (p.action === "delete") caps.delete = true
      else if (p.action === "grant") caps.manage_roles = true
      else extraAtoms.push(toInput(p))
      continue
    }
    extraAtoms.push(toInput(p))
  }

  return { caps, extraAtoms }
}

export function gvCapabilitiesToAtoms(
  caps: GvRoleCapabilities,
  name: string,
  extraAtoms: PermissionAtomInput[] = [],
): PermissionAtomInput[] {
  const atoms: PermissionAtomInput[] = []
  if (caps.read) atoms.push(atom("read", "global_values", { key_name: name }))
  if (caps.write) atoms.push(atom("write", "global_values", { key_name: name }))
  if (caps.delete) atoms.push(atom("delete", "global_values", { key_name: name }))
  if (caps.manage_roles)
    atoms.push(atom("grant", "global_values", { key_name: name }))
  return [...atoms, ...extraAtoms]
}

// Short human-readable labels for a role's capabilities, for list summaries.
export function projectCapabilityLabels(
  caps: ProjectRoleCapabilities,
): string[] {
  const labels: string[] = []
  if (caps.read_project) labels.push("Read project")
  if (caps.templates.write_templates) labels.push("Write templates")
  else if (caps.templates.read_templates) labels.push("Read templates")
  if (caps.templates.delete_templates) labels.push("Delete templates")

  const writeEnvs = Object.entries(caps.envLevels)
    .filter(([, l]) => l === "write")
    .map(([e]) => e)
  const readEnvs = Object.entries(caps.envLevels)
    .filter(([, l]) => l === "read")
    .map(([e]) => e)
  if (writeEnvs.length) labels.push(`Write values: ${writeEnvs.join(", ")}`)
  if (readEnvs.length) labels.push(`Read values: ${readEnvs.join(", ")}`)

  if (caps.manage_environments) labels.push("Manage environments")
  if (caps.manage_members_roles) labels.push("Manage members & roles")
  if (caps.delete_project) labels.push("Delete project")
  return labels
}

export function gvCapabilityLabels(caps: GvRoleCapabilities): string[] {
  const labels: string[] = []
  if (caps.write) labels.push("Write values")
  else if (caps.read) labels.push("Read values")
  if (caps.delete) labels.push("Delete entry")
  if (caps.manage_roles) labels.push("Manage roles")
  return labels
}
