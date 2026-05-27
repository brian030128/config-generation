import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { environmentsApi } from "@/api/environments"
import { projectKeys } from "./use-projects"

export const environmentKeys = {
  forProject: (project: string) =>
    [...projectKeys.detail(project), "environments"] as const,
  admins: (project: string, env: string) =>
    [...projectKeys.detail(project), "environments", env, "admins"] as const,
}

export function useEnvironments(projectName: string) {
  return useQuery({
    queryKey: environmentKeys.forProject(projectName),
    queryFn: () => environmentsApi.list(projectName),
    enabled: !!projectName,
  })
}

export function useEnvAdmins(projectName: string, envName: string) {
  return useQuery({
    queryKey: environmentKeys.admins(projectName, envName),
    queryFn: () => environmentsApi.listAdmins(projectName, envName),
    enabled: !!projectName && !!envName,
  })
}

export function useAddEnvAdmin(projectName: string, envName: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (userId: number) =>
      environmentsApi.addAdmin(projectName, envName, userId),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: environmentKeys.admins(projectName, envName),
      }),
  })
}

export function useRemoveEnvAdmin(projectName: string, envName: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (userId: number) =>
      environmentsApi.removeAdmin(projectName, envName, userId),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: environmentKeys.admins(projectName, envName),
      }),
  })
}
