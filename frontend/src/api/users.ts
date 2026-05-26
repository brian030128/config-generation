import { client } from "./client"
import type { ListResponse, User } from "./types"

export const usersApi = {
  search: (query: string) =>
    client
      .get<ListResponse<User>>("/users", { params: { search: query } })
      .then((r) => r.data),
}
