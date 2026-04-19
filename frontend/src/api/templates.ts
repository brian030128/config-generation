import { client } from "./client"
import type {
  ListResponse,
  ProjectConfigTemplate,
  CreateTemplateRequest,
  AppendTemplateVersionRequest,
  TemplateVariablesResponse,
} from "./types"

export const templatesApi = {
  listForProject: (projectName: string) =>
    client
      .get<ListResponse<ProjectConfigTemplate>>(
        `/projects/${projectName}/templates`,
      )
      .then((r) => r.data),

  create: (projectName: string, req: CreateTemplateRequest) =>
    client
      .post<ProjectConfigTemplate>(`/projects/${projectName}/templates`, req)
      .then((r) => r.data),

  getLatest: (projectName: string, templateName: string) =>
    client
      .get<ProjectConfigTemplate>(
        `/projects/${projectName}/templates/${templateName}`,
      )
      .then((r) => r.data),

  listVersions: (projectName: string, templateName: string) =>
    client
      .get<ListResponse<ProjectConfigTemplate>>(
        `/projects/${projectName}/templates/${templateName}/versions`,
      )
      .then((r) => r.data),

  appendVersion: (
    projectName: string,
    templateName: string,
    req: AppendTemplateVersionRequest,
  ) =>
    client
      .post<ProjectConfigTemplate>(
        `/projects/${projectName}/templates/${templateName}/versions`,
        req,
      )
      .then((r) => r.data),

  getVariables: (projectName: string, templateName: string) =>
    client
      .get<TemplateVariablesResponse>(
        `/projects/${projectName}/templates/${templateName}/variables`,
      )
      .then((r) => r.data),

  getProjectVariables: (projectName: string) =>
    client
      .get<TemplateVariablesResponse>(
        `/projects/${projectName}/variables`,
      )
      .then((r) => r.data),

  getVersion: (
    projectName: string,
    templateName: string,
    versionId: number,
  ) =>
    client
      .get<ProjectConfigTemplate>(
        `/projects/${projectName}/templates/${templateName}/versions/${versionId}`,
      )
      .then((r) => r.data),
}
