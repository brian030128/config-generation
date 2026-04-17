import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { globalValuesApi } from "@/api/global-values"
import type {
  CreateGlobalValuesRequest,
  AppendGlobalValuesVersionRequest,
} from "@/api/types"

export const globalValuesKeys = {
  all: ["global-values"] as const,
  detail: (name: string) => ["global-values", name] as const,
  versions: (name: string) => ["global-values", name, "versions"] as const,
}

export function useGlobalValues() {
  return useQuery({
    queryKey: globalValuesKeys.all,
    queryFn: globalValuesApi.list,
  })
}

export function useGlobalValue(name: string) {
  return useQuery({
    queryKey: globalValuesKeys.detail(name),
    queryFn: () => globalValuesApi.getLatest(name),
    enabled: !!name,
  })
}

export function useGlobalValueVersions(name: string) {
  return useQuery({
    queryKey: globalValuesKeys.versions(name),
    queryFn: () => globalValuesApi.listVersions(name),
    enabled: !!name,
  })
}

export function useCreateGlobalValues() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: CreateGlobalValuesRequest) =>
      globalValuesApi.create(req),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: globalValuesKeys.all }),
  })
}

export function useAppendGlobalValueVersion(name: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: AppendGlobalValuesVersionRequest) =>
      globalValuesApi.appendVersion(name, req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: globalValuesKeys.all })
      qc.invalidateQueries({ queryKey: globalValuesKeys.detail(name) })
      qc.invalidateQueries({ queryKey: globalValuesKeys.versions(name) })
    },
  })
}
