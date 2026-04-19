import { client } from "./client"
import type {
  ListResponse,
  Environment,
} from "./types"

export const environmentsApi = {
  list: (projectName: string) =>
    client.get<ListResponse<Environment>>(`/projects/${projectName}/environments`).then((r) => r.data),

  get: (projectName: string, envName: string) =>
    client.get<Environment>(`/projects/${projectName}/environments/${envName}`).then((r) => r.data),
}
