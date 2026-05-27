import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { valuesApi } from "@/api/values"
import type {
  CreateProjectConfigValuesRequest,
  AppendProjectConfigValuesVersionRequest,
} from "@/api/types"
import { projectKeys } from "./use-projects"

export const valuesKeys = {
  forProjectEnv: (project: string, env: string) =>
    [...projectKeys.detail(project), "envs", env, "values"] as const,
}

export function useValues(project: string, env: string) {
  return useQuery({
    queryKey: valuesKeys.forProjectEnv(project, env),
    queryFn: () => valuesApi.getLatest(project, env),
    enabled: !!env,
  })
}

export function useCreateValues(project: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: CreateProjectConfigValuesRequest) =>
      valuesApi.create(project, req),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: projectKeys.detail(project) }),
  })
}

export function useAppendValuesVersion(project: string, env: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: AppendProjectConfigValuesVersionRequest) =>
      valuesApi.appendVersion(project, env, req),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: valuesKeys.forProjectEnv(project, env),
      })
    },
  })
}
