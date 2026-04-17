import { client } from "./client"
import type {
  ListResponse,
  Environment,
  CreateEnvironmentRequest,
} from "./types"

export const environmentsApi = {
  list: () =>
    client.get<ListResponse<Environment>>("/environments").then((r) => r.data),

  get: (envId: number) =>
    client.get<Environment>(`/environments/${envId}`).then((r) => r.data),

  create: (req: CreateEnvironmentRequest) =>
    client.post<Environment>("/environments", req).then((r) => r.data),
}
