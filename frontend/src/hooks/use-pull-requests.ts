import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { pullRequestsApi } from "@/api/pull-requests"
import type { CreatePullRequestRequest } from "@/api/types"

export const pullRequestKeys = {
  all: ["pull-requests"] as const,
  list: () => ["pull-requests", "list"] as const,
  detail: (id: number) => ["pull-requests", id] as const,
}

export function usePullRequests() {
  return useQuery({
    queryKey: pullRequestKeys.list(),
    queryFn: () => pullRequestsApi.list(),
  })
}

export function usePullRequest(id: number) {
  return useQuery({
    queryKey: pullRequestKeys.detail(id),
    queryFn: () => pullRequestsApi.get(id),
    enabled: id > 0,
  })
}

export function useCreatePullRequest() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: CreatePullRequestRequest) => pullRequestsApi.create(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: pullRequestKeys.all }),
  })
}

export function useClosePullRequest() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => pullRequestsApi.close(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: pullRequestKeys.all }),
  })
}
