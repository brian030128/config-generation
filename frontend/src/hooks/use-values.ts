import { useQuery } from "@tanstack/react-query"
import { valuesApi } from "@/api/values"
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
