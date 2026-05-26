import { client } from "./client"
import type {
  ListResponse,
  Project,
  CreateProjectRequest,
  ProjectMember,
  AddProjectMemberRequest,
} from "./types"

export const projectsApi = {
  list: () =>
    client.get<ListResponse<Project>>("/projects").then((r) => r.data),

  get: (name: string) =>
    client.get<Project>(`/projects/${name}`).then((r) => r.data),

  create: (req: CreateProjectRequest) =>
    client.post<Project>("/projects", req).then((r) => r.data),

  delete: (name: string) => client.delete(`/projects/${name}`),

  listMembers: (name: string) =>
    client
      .get<ListResponse<ProjectMember>>(`/projects/${name}/members`)
      .then((r) => r.data),

  addMember: (name: string, userId: number) =>
    client
      .post<ProjectMember>(`/projects/${name}/members`, {
        user_id: userId,
      } satisfies AddProjectMemberRequest)
      .then((r) => r.data),

  removeMember: (name: string, userId: number) =>
    client.delete(`/projects/${name}/members/${userId}`),
}
