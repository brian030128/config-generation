import { client } from "./client"
import type {
  ListResponse,
  ProjectConfigTemplate,
  TemplateVariablesResponse,
} from "./types"

// Reads of published templates. Authoring (create/edit/delete) goes through the
// workspace API (see pull-requests.ts).
export const templatesApi = {
  listForProject: (projectName: string) =>
    client
      .get<ListResponse<ProjectConfigTemplate>>(
        `/projects/${projectName}/templates`,
      )
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
