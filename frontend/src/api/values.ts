import { client } from "./client"
import type { ProjectConfigValues } from "./types"

// Reads of published values. Creating/editing/deleting values goes through the
// workspace API (see pull-requests.ts).
export const valuesApi = {
  getLatest: (projectName: string, envName: string) =>
    client
      .get<ProjectConfigValues>(
        `/projects/${projectName}/envs/${envName}/values`,
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
