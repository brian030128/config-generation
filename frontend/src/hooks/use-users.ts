import { useQuery, keepPreviousData } from "@tanstack/react-query"
import { usersApi } from "@/api/users"

export function useUserSearch(query: string, enabled = true) {
  return useQuery({
    queryKey: ["users", "search", query],
    queryFn: () => usersApi.search(query),
    enabled,
    placeholderData: keepPreviousData,
  })
}
