import { useQuery } from "@tanstack/react-query"
import { environmentsApi } from "@/api/environments"
import { projectKeys } from "./use-projects"

export const environmentKeys = {
  forProject: (project: string) =>
    [...projectKeys.detail(project), "environments"] as const,
}

export function useEnvironments(projectName: string) {
  return useQuery({
    queryKey: environmentKeys.forProject(projectName),
    queryFn: () => environmentsApi.list(projectName),
    enabled: !!projectName,
  })
}
