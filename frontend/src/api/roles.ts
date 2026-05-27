import { client } from "./client"
import type {
  RolesResponse,
  Role,
  CreateRoleRequest,
  EditRolePermissionsRequest,
  UserRole,
  Project,
  GlobalValues,
} from "./types"

// Roles are a single global namespace. Listing is open to any authenticated
// user; all mutations are superuser-only (enforced by the backend).
export const rolesApi = {
  list: () => client.get<RolesResponse>("/roles").then((r) => r.data),

  create: (req: CreateRoleRequest) =>
    client.post<Role>("/roles", req).then((r) => r.data),

  editPermissions: (roleId: number, req: EditRolePermissionsRequest) =>
    client.put(`/roles/${roleId}/permissions`, req),

  delete: (roleId: number) => client.delete(`/roles/${roleId}`),

  assignUser: (roleId: number, userId: number) =>
    client
      .post<UserRole>(`/roles/${roleId}/members`, { user_id: userId })
      .then((r) => r.data),

  removeUser: (roleId: number, userId: number) =>
    client.delete(`/roles/${roleId}/members/${userId}`),

  updateProjectApprovalCondition: (
    projectName: string,
    approvalCondition: string,
  ) =>
    client
      .put<Project>(`/projects/${projectName}/approval-condition`, {
        approval_condition: approvalCondition,
      })
      .then((r) => r.data),

  updateGvApprovalCondition: (name: string, approvalCondition: string) =>
    client
      .put<GlobalValues>(`/global-values/${name}/approval-condition`, {
        approval_condition: approvalCondition,
      })
      .then((r) => r.data),
}
