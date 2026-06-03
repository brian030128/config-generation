// Response wrappers
export interface ListResponse<T> {
  items: T[]
  count: number
}

export interface ErrorResponse {
  error: string
  code?: string
  details?: string
}

// Domain models
export interface User {
  id: number
  username: string
  display_name: string | null
  created_at: string
}

export interface Project {
  id: number
  name: string
  description: string | null
  approval_condition: string
  created_by: number
  created_at: string
  updated_at: string
}

export interface Environment {
  id: number
  project_id: number
  name: string
  description: string | null
  created_by: number
  created_at: string
}

export interface ProjectMember {
  user_id: number
  username: string
  display_name: string | null
  added_by: number
  added_at: string
}

export interface ProjectMembersResponse {
  items: ProjectMember[]
  count: number
  // Whether the current user holds grant(p) and may add/remove members.
  viewer_can_manage: boolean
}

// What a project admin can grant a member. Templates are project-wide; value
// access is granted per environment so a member can get staging but not prod.
export interface MemberPermissions {
  read_templates: boolean
  write_templates: boolean
  delete_templates: boolean
  environments: EnvValueAccess[]
}

// A member's value access to one environment. read only → "Read only";
// write → "Read & write" (read implied). Absent/both false → "No access".
export interface EnvValueAccess {
  env: string
  read: boolean
  write: boolean
}

// An env-admin grant on a single environment.
export interface EnvAdmin {
  user_id: number
  username: string
  display_name: string | null
  granted_by: number
  granted_at: string
}

export interface AddEnvAdminRequest {
  user_id: number
}

export interface ProjectConfigTemplate {
  id: number
  project_id: number
  template_name: string
  version_id: number
  body: string
  commit_message: string | null
  created_by: number
  created_at: string
}

export interface ProjectConfigValues {
  id: number
  project_id: number
  environment_id: number
  version_id: number
  payload: Record<string, unknown>
  commit_message: string | null
  created_by: number
  created_at: string
}

// A global-value payload value: a scalar, or a list of strings (for templates that
// `range` over it). See GlobalValueValue for the shared union.
export type GlobalValueValue = string | number | boolean | null | string[]

// A single immutable global-values content row inside a group.
export interface GlobalValues {
  id: number
  group_id: number
  name: string
  payload: Record<string, GlobalValueValue>
  commit_message: string | null
  created_by: number
  created_at: string
}

// A named bundle of global values. Versioning lives on the group, not on
// individual values.
export interface GlobalValuesGroup {
  id: number
  name: string
  approval_condition: string
  created_by: number
  created_at: string
}

// One snapshot in a group's version history. `values` is the materialized
// (gv_name -> payload) map at this version.
export interface GlobalValuesGroupVersion {
  id: number
  group_id: number
  group_name: string
  version_id: number
  parent_version_id: number | null
  commit_message: string | null
  created_by: number
  created_at: string
  values?: Record<string, Record<string, GlobalValueValue>>
}

// GetLatest response shape: group + latest materialized version + management flag.
export interface GlobalValuesGroupDetailResponse {
  group: GlobalValuesGroup
  latest_version: GlobalValuesGroupVersion
  viewer_can_manage: boolean
}

// A per-project version snapshot listing entry. Each project version covers
// every template and every env values payload at that point in time.
export interface ProjectVersion {
  id: number
  project_id: number
  version_id: number
  parent_version_id: number | null
  commit_message: string | null
  is_anchor: boolean
  created_by: number
  created_at: string
}

export interface ProjectVersionManifest extends ProjectVersion {
  templates: ProjectConfigTemplate[]
  values: ProjectConfigValues[]
}

// Role is a global, named bundle of permission atoms. Its scope comes only from
// the atoms' keys; it is not bound to any project or global values entry.
export interface Role {
  id: number
  name: string
  is_auto_created: boolean
  created_at: string
  permissions?: RolePermission[]
  members?: UserRole[]
}

export interface RolePermission {
  id: number
  role_id: number
  action: string
  resource: string
  key_project: string | null
  key_env: string | null
  key_name: string | null
}

export interface UserRole {
  id: number
  user_id: number
  role_id: number
  granted_by: number
  granted_at: string
  // Populated only by the role-list endpoints (joined from users).
  username?: string
  display_name?: string | null
}

// RolesResponse is the role list for a scope plus whether the caller may manage
// roles (holds grant). Mirrors ProjectMembersResponse.
export interface RolesResponse {
  items: Role[]
  count: number
  viewer_can_manage: boolean
}

// A single permission atom, as accepted by the role create/edit endpoints.
export interface PermissionAtomInput {
  action: string
  resource: string
  key_project?: string | null
  key_env?: string | null
  key_name?: string | null
}

export interface CreateRoleRequest {
  name: string
  permissions: PermissionAtomInput[]
}

export interface EditRolePermissionsRequest {
  permissions: PermissionAtomInput[]
}

export interface UpdateApprovalConditionRequest {
  approval_condition: string
}

// Request types
export interface CreateProjectRequest {
  name: string
  description?: string
  approval_condition?: string
}

export interface AddProjectMemberRequest {
  user_id: number
}

export interface CreateTemplateRequest {
  template_name: string
  body: string
  commit_message?: string
}

export interface AppendTemplateVersionRequest {
  body: string
  commit_message?: string
}

export interface AppendProjectConfigValuesVersionRequest {
  payload: Record<string, unknown>
  commit_message?: string
}

// Create a new group with its v1 snapshot — values is a (gv_name -> payload) map.
export interface CreateGlobalValuesGroupRequest {
  name: string
  values: Record<string, Record<string, GlobalValueValue>>
  commit_message?: string
  approval_condition?: string
}

// Append a new group version. values is the new (gv_name -> payload) map; any
// entry that matches the previous version's payload reuses the same content row.
export interface AppendGlobalValuesGroupVersionRequest {
  values: Record<string, Record<string, GlobalValueValue>>
  commit_message?: string
}

// Pull Requests
export interface PullRequest {
  id: number
  project_id: number | null
  global_values_name: string | null
  // Snapshot the PR was authored against. Conflict detection at merge compares
  // these against the project's / group's current latest version.
  base_project_version_id: number | null
  base_group_version_id: number | null
  author_id: number
  title: string
  description: string | null
  status: "draft" | "open" | "approved" | "merged" | "closed"
  is_conflicted: boolean
  approval_condition: string
  created_at: string
  updated_at: string
  merged_at: string | null
  closed_at: string | null
  changes?: PRChange[]
  approvals?: PRApproval[]
}

export interface PRApproval {
  id: number
  pr_id: number
  user_id: number
  approved_at: string
  withdrawn_at: string | null
}

export interface PRChange {
  id: number
  pr_id: number
  object_type: "template" | "values" | "global_values" | "environment"
  operation: "create" | "update" | "delete"
  project_id: number | null
  template_name: string | null
  environment_name: string | null
  global_values_name: string | null
  proposed_payload: string
  created_at: string
}

// Template variables
export interface TemplateVariable {
  name: string
  default?: string
  // How the variable is used in the template: "string" for a plain reference,
  // "list" when it is the target of a {{range}}, "conflict" when used both ways.
  // Older backends may omit this; treat a missing kind as "string".
  kind?: "string" | "list" | "conflict"
}

export interface TemplateVariablesResponse {
  variables: TemplateVariable[]
}

export interface CreatePullRequestRequest {
  title: string
  description?: string
  object_type: string
  global_values_name?: string
  proposed_payload: string
}

// Workspace validation
export interface WorkspaceProblem {
  kind:
    | "no_environments"
    | "missing_values"
    | "unknown_global_values"
    | "unknown_key"
    | "template_parse"
    | "template_exec"
  template_name?: string
  environment_name?: string
  missing_keys?: string[]
  message: string
}

export interface WorkspaceValidationResponse {
  valid: boolean
  problems: WorkspaceProblem[]
}

// Deployment types
//
// Pin a single project_version_id (everything in the project — templates and
// envs — comes from this snapshot) plus one group version per referenced
// global-values group.
export interface DeployPreviewRequest {
  project_version_id: number
  global_values_group_versions: Record<string, number>
}

export interface DeployRequest extends DeployPreviewRequest {
  commit_message?: string
}

export interface TemplateRenderResult {
  template_name: string
  rendered_output?: string
  error?: string
  error_kind?: string
  previous_output?: string
  template_body: string
  previous_template_body?: string
}

// The last successful deployment a preview diffs against; absent when the
// project+env has never been deployed.
export interface PreviousDeploymentInfo {
  deployment_id: number
  project_version: number
  created_at: string
}

export interface DeployPreviewResponse {
  results: TemplateRenderResult[]
  values_payload: Record<string, unknown>
  previous_values?: Record<string, unknown>
  project_version_id: number
  global_values: Record<string, Record<string, unknown>>
  previous_global_values?: Record<string, Record<string, unknown>>
  global_values_group_versions: Record<string, number>
  has_errors: boolean
  previous_deployment?: PreviousDeploymentInfo
}

export interface DeployResponse {
  deployment_id: number
  status: string
  results: TemplateRenderResult[]
}

export interface LatestDeploymentResponse {
  deployment_id: number
  project_version_id: number
  global_values_group_versions: Record<string, number>
  created_at: string
  commit_message?: string
}
