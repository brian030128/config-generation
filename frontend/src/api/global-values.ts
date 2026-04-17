import { client } from "./client"
import type {
  ListResponse,
  GlobalValues,
  CreateGlobalValuesRequest,
  AppendGlobalValuesVersionRequest,
} from "./types"

export const globalValuesApi = {
  list: () =>
    client.get<ListResponse<GlobalValues>>("/global-values").then((r) => r.data),

  create: (req: CreateGlobalValuesRequest) =>
    client.post<GlobalValues>("/global-values", req).then((r) => r.data),

  getLatest: (name: string) =>
    client.get<GlobalValues>(`/global-values/${name}`).then((r) => r.data),

  listVersions: (name: string) =>
    client
      .get<ListResponse<GlobalValues>>(`/global-values/${name}/versions`)
      .then((r) => r.data),

  appendVersion: (name: string, req: AppendGlobalValuesVersionRequest) =>
    client
      .post<GlobalValues>(`/global-values/${name}/versions`, req)
      .then((r) => r.data),

  getVersion: (name: string, versionId: number) =>
    client
      .get<GlobalValues>(`/global-values/${name}/versions/${versionId}`)
      .then((r) => r.data),
}
