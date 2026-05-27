import { client } from "./client"
import type {
  ListResponse,
  Environment,
  EnvAdmin,
  AddEnvAdminRequest,
} from "./types"

export const environmentsApi = {
  list: (projectName: string) =>
    client.get<ListResponse<Environment>>(`/projects/${projectName}/environments`).then((r) => r.data),

  get: (projectName: string, envName: string) =>
    client.get<Environment>(`/projects/${projectName}/environments/${envName}`).then((r) => r.data),

  listAdmins: (projectName: string, envName: string) =>
    client
      .get<ListResponse<EnvAdmin>>(
        `/projects/${projectName}/environments/${envName}/admins`,
      )
      .then((r) => r.data),

  addAdmin: (projectName: string, envName: string, userId: number) =>
    client
      .post<EnvAdmin>(
        `/projects/${projectName}/environments/${envName}/admins`,
        { user_id: userId } satisfies AddEnvAdminRequest,
      )
      .then((r) => r.data),

  removeAdmin: (projectName: string, envName: string, userId: number) =>
    client.delete(
      `/projects/${projectName}/environments/${envName}/admins/${userId}`,
    ),
}
