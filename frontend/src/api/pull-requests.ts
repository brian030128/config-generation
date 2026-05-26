import { client } from "./client"
import type {
  ListResponse,
  PullRequest,
  PRChange,
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

  // ── Workspace: the project write surface (one method per object/operation) ──

  getActiveDraft: (projectName: string) =>
    client.get<PullRequest>(`/workspace/${projectName}`).then((r) => r.data),

  discardWorkspace: (projectName: string) =>
    client.delete(`/workspace/${projectName}`).then((r) => r.data),

  listChanges: (projectName: string) =>
    client
      .get<ListResponse<PRChange>>(`/workspace/${projectName}/changes`)
      .then((r) => r.data),

  unstageChange: (projectName: string, changeID: number) =>
    client
      .delete<PullRequest>(`/workspace/${projectName}/changes/${changeID}`)
      .then((r) => r.data),

  // Templates
  createTemplate: (
    projectName: string,
    req: { template_name: string; body: string; commit_message?: string },
  ) =>
    client
      .post<PullRequest>(`/workspace/${projectName}/templates`, req)
      .then((r) => r.data),

  updateTemplate: (
    projectName: string,
    templateName: string,
    req: { body: string; commit_message?: string },
  ) =>
    client
      .put<PullRequest>(`/workspace/${projectName}/templates/${templateName}`, req)
      .then((r) => r.data),

  deleteTemplate: (projectName: string, templateName: string) =>
    client
      .delete<PullRequest>(`/workspace/${projectName}/templates/${templateName}`)
      .then((r) => r.data),

  // Environments
  createEnvironment: (
    projectName: string,
    req: { name: string; description?: string },
  ) =>
    client
      .post<PullRequest>(`/workspace/${projectName}/environments`, req)
      .then((r) => r.data),

  deleteEnvironment: (projectName: string, envName: string) =>
    client
      .delete<PullRequest>(`/workspace/${projectName}/environments/${envName}`)
      .then((r) => r.data),

  // Values
  putValues: (
    projectName: string,
    envName: string,
    req: { payload: Record<string, unknown>; commit_message?: string },
  ) =>
    client
      .put<PullRequest>(`/workspace/${projectName}/envs/${envName}/values`, req)
      .then((r) => r.data),

  deleteValues: (projectName: string, envName: string) =>
    client
      .delete<PullRequest>(`/workspace/${projectName}/envs/${envName}/values`)
      .then((r) => r.data),
}
