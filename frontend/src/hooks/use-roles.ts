import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { rolesApi } from "@/api/roles"
import type { CreateRoleRequest, PermissionAtomInput } from "@/api/types"
import { projectKeys } from "./use-projects"
import { globalValuesKeys } from "./use-global-values"

export const roleKeys = {
  all: ["roles"] as const,
}

export function useRoles(enabled = true) {
  return useQuery({
    queryKey: roleKeys.all,
    queryFn: rolesApi.list,
    enabled,
  })
}

export function useCreateRole() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: CreateRoleRequest) => rolesApi.create(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: roleKeys.all }),
  })
}

export function useEditRolePermissions() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({
      roleId,
      permissions,
    }: {
      roleId: number
      permissions: PermissionAtomInput[]
    }) => rolesApi.editPermissions(roleId, { permissions }),
    onSuccess: () => qc.invalidateQueries({ queryKey: roleKeys.all }),
  })
}

export function useDeleteRole() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (roleId: number) => rolesApi.delete(roleId),
    onSuccess: () => qc.invalidateQueries({ queryKey: roleKeys.all }),
  })
}

export function useAssignRole() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ roleId, userId }: { roleId: number; userId: number }) =>
      rolesApi.assignUser(roleId, userId),
    onSuccess: () => qc.invalidateQueries({ queryKey: roleKeys.all }),
  })
}

export function useRemoveRoleMember() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ roleId, userId }: { roleId: number; userId: number }) =>
      rolesApi.removeUser(roleId, userId),
    onSuccess: () => qc.invalidateQueries({ queryKey: roleKeys.all }),
  })
}

export function useUpdateProjectApprovalCondition(projectName: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (approvalCondition: string) =>
      rolesApi.updateProjectApprovalCondition(projectName, approvalCondition),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: projectKeys.detail(projectName) })
      qc.invalidateQueries({ queryKey: projectKeys.all })
    },
  })
}

export function useUpdateGvApprovalCondition(name: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (approvalCondition: string) =>
      rolesApi.updateGvApprovalCondition(name, approvalCondition),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: globalValuesKeys.detail(name) })
      qc.invalidateQueries({ queryKey: globalValuesKeys.all })
    },
  })
}
