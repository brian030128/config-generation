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
}
