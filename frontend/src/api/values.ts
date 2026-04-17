import { client } from "./client"
import type {
  ListResponse,
  ProjectConfigValues,
  CreateProjectConfigValuesRequest,
  AppendProjectConfigValuesVersionRequest,
} from "./types"

export const valuesApi = {
  create: (projectName: string, req: CreateProjectConfigValuesRequest) =>
    client
      .post<ProjectConfigValues>(`/projects/${projectName}/values`, req)
      .then((r) => r.data),

  getLatest: (
    projectName: string,
    templateName: string,
    envName: string,
  ) =>
    client
      .get<ProjectConfigValues>(
        `/projects/${projectName}/templates/${templateName}/envs/${envName}/values`,
      )
      .then((r) => r.data),

  appendVersion: (
    projectName: string,
    templateName: string,
    envName: string,
    req: AppendProjectConfigValuesVersionRequest,
  ) =>
    client
      .post<ProjectConfigValues>(
        `/projects/${projectName}/templates/${templateName}/envs/${envName}/values/versions`,
        req,
      )
      .then((r) => r.data),

  getVersion: (
    projectName: string,
    templateName: string,
    envName: string,
    versionId: number,
  ) =>
    client
      .get<ProjectConfigValues>(
        `/projects/${projectName}/templates/${templateName}/envs/${envName}/values/versions/${versionId}`,
      )
      .then((r) => r.data),

  listForProjectEnv: (projectName: string, envName: string) =>
    client
      .get<ListResponse<ProjectConfigValues>>(
        `/projects/${projectName}/envs/${envName}/values`,
      )
      .then((r) => r.data),
}
