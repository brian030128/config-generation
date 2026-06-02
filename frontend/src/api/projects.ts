import { client } from "./client"
import type {
  ListResponse,
  Project,
  CreateProjectRequest,
  ProjectMember,
  ProjectMembersResponse,
  AddProjectMemberRequest,
  MemberPermissions,
  ProjectVersion,
  ProjectVersionManifest,
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
      .get<ProjectMembersResponse>(`/projects/${name}/members`)
      .then((r) => r.data),

  addMember: (name: string, userId: number) =>
    client
      .post<ProjectMember>(`/projects/${name}/members`, {
        user_id: userId,
      } satisfies AddProjectMemberRequest)
      .then((r) => r.data),

  removeMember: (name: string, userId: number) =>
    client.delete(`/projects/${name}/members/${userId}`),

  getMemberPermissions: (name: string, userId: number) =>
    client
      .get<MemberPermissions>(`/projects/${name}/members/${userId}/permissions`)
      .then((r) => r.data),

  setMemberPermissions: (
    name: string,
    userId: number,
    perms: MemberPermissions,
  ) =>
    client
      .put<MemberPermissions>(
        `/projects/${name}/members/${userId}/permissions`,
        perms,
      )
      .then((r) => r.data),

  listVersions: (name: string) =>
    client
      .get<ListResponse<ProjectVersion>>(`/projects/${name}/versions`)
      .then((r) => r.data),

  getVersion: (name: string, versionId: number) =>
    client
      .get<ProjectVersionManifest>(`/projects/${name}/versions/${versionId}`)
      .then((r) => r.data),
}
