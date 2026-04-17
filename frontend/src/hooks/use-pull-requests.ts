import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { pullRequestsApi } from "@/api/pull-requests"
import type { CreatePullRequestRequest } from "@/api/types"

export const pullRequestKeys = {
  all: ["pull-requests"] as const,
  list: () => ["pull-requests", "list"] as const,
}

export function usePullRequests() {
  return useQuery({
    queryKey: pullRequestKeys.list(),
    queryFn: () => pullRequestsApi.list(),
  })
}

export function useCreatePullRequest() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: CreatePullRequestRequest) => pullRequestsApi.create(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: pullRequestKeys.all }),
  })
}
