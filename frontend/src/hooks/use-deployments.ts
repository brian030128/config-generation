import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { deploymentsApi } from "@/api/deployments"
import type { DeployPreviewRequest, DeployRequest } from "@/api/types"
import { projectKeys } from "./use-projects"

export const deploymentKeys = {
  latest: (project: string, env: string) =>
    [...projectKeys.detail(project), "envs", env, "deployment-latest"] as const,
}

export function useLatestDeployment(project: string, env: string) {
  return useQuery({
    queryKey: deploymentKeys.latest(project, env),
    queryFn: () => deploymentsApi.getLatest(project, env),
    enabled: !!project && !!env,
    retry: false,
  })
}

export function useDeployPreview(project: string, env: string) {
  return useMutation({
    mutationFn: (req: DeployPreviewRequest) =>
      deploymentsApi.preview(project, env, req),
  })
}

export function useExecuteDeploy(project: string, env: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: DeployRequest) =>
      deploymentsApi.execute(project, env, req),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: deploymentKeys.latest(project, env),
      })
    },
  })
}
