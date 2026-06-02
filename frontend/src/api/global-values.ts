import { client } from "./client"
import type {
  ListResponse,
  GlobalValuesGroup,
  GlobalValuesGroupVersion,
  GlobalValuesGroupDetailResponse,
  CreateGlobalValuesGroupRequest,
  AppendGlobalValuesGroupVersionRequest,
} from "./types"

export const globalValuesApi = {
  list: () =>
    client
      .get<ListResponse<GlobalValuesGroup>>("/global-values-groups")
      .then((r) => r.data),

  create: (req: CreateGlobalValuesGroupRequest) =>
    client
      .post<GlobalValuesGroupDetailResponse>("/global-values-groups", req)
      .then((r) => r.data),

  getLatest: (name: string) =>
    client
      .get<GlobalValuesGroupDetailResponse>(`/global-values-groups/${name}`)
      .then((r) => r.data),

  listVersions: (name: string) =>
    client
      .get<ListResponse<GlobalValuesGroupVersion>>(
        `/global-values-groups/${name}/versions`,
      )
      .then((r) => r.data),

  appendVersion: (name: string, req: AppendGlobalValuesGroupVersionRequest) =>
    client
      .post<GlobalValuesGroupVersion>(
        `/global-values-groups/${name}/versions`,
        req,
      )
      .then((r) => r.data),

  getVersion: (name: string, versionId: number) =>
    client
      .get<GlobalValuesGroupVersion>(
        `/global-values-groups/${name}/versions/${versionId}`,
      )
      .then((r) => r.data),
}
