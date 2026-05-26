import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { projectsApi } from "@/api/projects"
import type { CreateProjectRequest } from "@/api/types"

export const projectKeys = {
  all: ["projects"] as const,
  detail: (name: string) => ["projects", name] as const,
  members: (name: string) => ["projects", name, "members"] as const,
}

export function useProjects() {
  return useQuery({
    queryKey: projectKeys.all,
    queryFn: projectsApi.list,
  })
}

export function useProject(name: string) {
  return useQuery({
    queryKey: projectKeys.detail(name),
    queryFn: () => projectsApi.get(name),
  })
}

export function useCreateProject() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: CreateProjectRequest) => projectsApi.create(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: projectKeys.all }),
  })
}

export function useDeleteProject() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => projectsApi.delete(name),
    onSuccess: () => qc.invalidateQueries({ queryKey: projectKeys.all }),
  })
}

export function useProjectMembers(name: string) {
  return useQuery({
    queryKey: projectKeys.members(name),
    queryFn: () => projectsApi.listMembers(name),
    enabled: !!name,
  })
}

export function useAddProjectMember(name: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (userId: number) => projectsApi.addMember(name, userId),
    onSuccess: () => qc.invalidateQueries({ queryKey: projectKeys.members(name) }),
  })
}

export function useRemoveProjectMember(name: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (userId: number) => projectsApi.removeMember(name, userId),
    onSuccess: () => qc.invalidateQueries({ queryKey: projectKeys.members(name) }),
  })
}
