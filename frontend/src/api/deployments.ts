import { client } from "./client"
import type {
  DeployPreviewRequest,
  DeployPreviewResponse,
  DeployRequest,
  DeployResponse,
  LatestDeploymentResponse,
} from "./types"

export const deploymentsApi = {
  preview: (projectName: string, envName: string, req: DeployPreviewRequest) =>
    client
      .post<DeployPreviewResponse>(
        `/projects/${projectName}/envs/${envName}/deploy/preview`,
        req,
      )
      .then((r) => r.data),

  execute: (projectName: string, envName: string, req: DeployRequest) =>
    client
      .post<DeployResponse>(
        `/projects/${projectName}/envs/${envName}/deploy`,
        req,
      )
      .then((r) => r.data),

  getLatest: (projectName: string, envName: string) =>
    client
      .get<LatestDeploymentResponse>(
        `/projects/${projectName}/envs/${envName}/deployments/latest`,
      )
      .then((r) => r.data),
}
