import { client } from "./client"
import type {
  ProjectConfigValues,
  CreateProjectConfigValuesRequest,
  AppendProjectConfigValuesVersionRequest,
} from "./types"

export const valuesApi = {
  create: (projectName: string, req: CreateProjectConfigValuesRequest) =>
    client
      .post<ProjectConfigValues>(`/projects/${projectName}/values`, req)
      .then((r) => r.data),

  getLatest: (projectName: string, envName: string) =>
    client
      .get<ProjectConfigValues>(
        `/projects/${projectName}/envs/${envName}/values`,
      )
      .then((r) => r.data),

  appendVersion: (
    projectName: string,
    envName: string,
    req: AppendProjectConfigValuesVersionRequest,
  ) =>
    client
      .post<ProjectConfigValues>(
        `/projects/${projectName}/envs/${envName}/values/versions`,
        req,
      )
      .then((r) => r.data),

  getVersion: (
    projectName: string,
    envName: string,
    versionId: number,
  ) =>
    client
      .get<ProjectConfigValues>(
        `/projects/${projectName}/envs/${envName}/values/versions/${versionId}`,
      )
      .then((r) => r.data),
}
