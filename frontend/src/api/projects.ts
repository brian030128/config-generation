import { client } from "./client"
import type { ListResponse, Project, CreateProjectRequest } from "./types"

export const projectsApi = {
  list: () =>
    client.get<ListResponse<Project>>("/projects").then((r) => r.data),

  get: (name: string) =>
    client.get<Project>(`/projects/${name}`).then((r) => r.data),

  create: (req: CreateProjectRequest) =>
    client.post<Project>("/projects", req).then((r) => r.data),

  delete: (name: string) => client.delete(`/projects/${name}`),
}
