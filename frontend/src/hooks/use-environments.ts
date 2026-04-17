import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { environmentsApi } from "@/api/environments"
import type { CreateEnvironmentRequest } from "@/api/types"

export const environmentKeys = {
  all: ["environments"] as const,
  detail: (id: number) => ["environments", id] as const,
}

export function useEnvironments() {
  return useQuery({
    queryKey: environmentKeys.all,
    queryFn: environmentsApi.list,
  })
}

export function useCreateEnvironment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: CreateEnvironmentRequest) => environmentsApi.create(req),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: environmentKeys.all }),
  })
}
