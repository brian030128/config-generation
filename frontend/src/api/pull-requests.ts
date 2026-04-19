import { client } from "./client"
import type {
  ListResponse,
  PullRequest,
  CreatePullRequestRequest,
} from "./types"

export const pullRequestsApi = {
  create: (req: CreatePullRequestRequest) =>
    client.post<PullRequest>("/pull-requests", req).then((r) => r.data),

  get: (id: number) =>
    client.get<PullRequest>(`/pull-requests/${id}`).then((r) => r.data),

  list: (params?: { global_values_name?: string }) =>
    client
      .get<ListResponse<PullRequest>>("/pull-requests", { params })
      .then((r) => r.data),

  close: (id: number) =>
    client.post<PullRequest>(`/pull-requests/${id}/close`).then((r) => r.data),

  merge: (id: number) =>
    client.post<PullRequest>(`/pull-requests/${id}/merge`).then((r) => r.data),

  approve: (id: number) =>
    client.post<PullRequest>(`/pull-requests/${id}/approve`).then((r) => r.data),

  withdrawApproval: (id: number) =>
    client
      .post<PullRequest>(`/pull-requests/${id}/withdraw-approval`)
      .then((r) => r.data),

  submitDraft: (id: number, req: { title: string; description?: string }) =>
    client.post<PullRequest>(`/pull-requests/${id}/submit`, req).then((r) => r.data),

  getActiveDraft: (projectName: string) =>
    client.get<PullRequest>(`/workspace/${projectName}/draft`).then((r) => r.data),

  stageChange: (projectName: string, req: {
    object_type: string
    template_name?: string
    environment_name?: string
    proposed_payload: string
  }) =>
    client.post<PullRequest>(`/workspace/${projectName}/stage`, req).then((r) => r.data),
}
