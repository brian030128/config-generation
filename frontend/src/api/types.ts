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
  name: string
  description: string | null
  created_at: string
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
  template_name: string
  environment_id: number
  version_id: number
  payload: Record<string, unknown>
  commit_message: string | null
  created_by: number
  created_at: string
}

export interface GlobalValues {
  id: number
  name: string
  version_id: number
  payload: Record<string, string | number | boolean | null>
  commit_message: string | null
  created_by: number
  created_at: string
}

export interface Role {
  id: number
  name: string
  project_id: number | null
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
}

// Request types
export interface CreateProjectRequest {
  name: string
  description?: string
  approval_condition?: string
}

export interface CreateEnvironmentRequest {
  name: string
  description?: string
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

export interface CreateProjectConfigValuesRequest {
  template_name: string
  environment_id: number
  payload: Record<string, unknown>
  commit_message?: string
}

export interface AppendProjectConfigValuesVersionRequest {
  payload: Record<string, unknown>
  commit_message?: string
}

export interface CreateGlobalValuesRequest {
  name: string
  payload: Record<string, string | number | boolean | null>
  commit_message?: string
}

export interface AppendGlobalValuesVersionRequest {
  payload: Record<string, string | number | boolean | null>
  commit_message?: string
}
