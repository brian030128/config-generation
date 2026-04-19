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

export function useMergePullRequest() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => pullRequestsApi.merge(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: pullRequestKeys.all })
      qc.invalidateQueries({ queryKey: ["global-values"] })
      // Merge creates new templates, environments, and values — invalidate all project data.
      qc.invalidateQueries({ queryKey: ["projects"] })
      qc.invalidateQueries({ queryKey: ["workspace"] })
    },
  })
}

export function useApprovePullRequest() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => pullRequestsApi.approve(id),
    onSuccess: (_, id) => {
      qc.invalidateQueries({ queryKey: pullRequestKeys.all })
      qc.invalidateQueries({ queryKey: pullRequestKeys.detail(id) })
    },
  })
}

export const workspaceKeys = {
  draft: (projectName: string) => ["workspace", projectName, "draft"] as const,
}

export function useActiveDraft(projectName: string) {
  return useQuery({
    queryKey: workspaceKeys.draft(projectName),
    queryFn: () => pullRequestsApi.getActiveDraft(projectName),
    enabled: !!projectName,
  })
}

export function useStageChange(projectName: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: {
      object_type: string
      template_name?: string
      environment_name?: string
      proposed_payload: string
    }) => pullRequestsApi.stageChange(projectName, req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: workspaceKeys.draft(projectName) })
      qc.invalidateQueries({ queryKey: pullRequestKeys.all })
    },
  })
}

export function useSubmitDraft() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (params: { id: number; title: string; description?: string }) =>
      pullRequestsApi.submitDraft(params.id, { title: params.title, description: params.description }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: pullRequestKeys.all })
      qc.invalidateQueries({ queryKey: ["workspace"] })
    },
  })
}

export function useWithdrawApproval() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => pullRequestsApi.withdrawApproval(id),
    onSuccess: (_, id) => {
      qc.invalidateQueries({ queryKey: pullRequestKeys.all })
      qc.invalidateQueries({ queryKey: pullRequestKeys.detail(id) })
    },
  })
}
