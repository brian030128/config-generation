import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { templatesApi } from "@/api/templates"
import type {
  CreateTemplateRequest,
  AppendTemplateVersionRequest,
} from "@/api/types"
import { projectKeys } from "./use-projects"

export const templateKeys = {
  forProject: (project: string) =>
    [...projectKeys.detail(project), "templates"] as const,
  detail: (project: string, template: string) =>
    [...projectKeys.detail(project), "templates", template] as const,
  versions: (project: string, template: string) =>
    [...projectKeys.detail(project), "templates", template, "versions"] as const,
}

export function useTemplates(projectName: string) {
  return useQuery({
    queryKey: templateKeys.forProject(projectName),
    queryFn: () => templatesApi.listForProject(projectName),
  })
}

export function useTemplateVariables(projectName: string, templateName: string) {
  return useQuery({
    queryKey: [...templateKeys.detail(projectName, templateName), "variables"] as const,
    queryFn: () => templatesApi.getVariables(projectName, templateName),
    enabled: !!templateName,
  })
}

export function useTemplate(projectName: string, templateName: string) {
  return useQuery({
    queryKey: templateKeys.detail(projectName, templateName),
    queryFn: () => templatesApi.getLatest(projectName, templateName),
  })
}

export function useTemplateVersions(
  projectName: string,
  templateName: string,
) {
  return useQuery({
    queryKey: templateKeys.versions(projectName, templateName),
    queryFn: () => templatesApi.listVersions(projectName, templateName),
    enabled: !!templateName,
  })
}

export function useCreateTemplate(projectName: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: CreateTemplateRequest) =>
      templatesApi.create(projectName, req),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: templateKeys.forProject(projectName),
      }),
  })
}

export function useAppendTemplateVersion(
  projectName: string,
  templateName: string,
) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: AppendTemplateVersionRequest) =>
      templatesApi.appendVersion(projectName, templateName, req),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: templateKeys.forProject(projectName),
      })
      qc.invalidateQueries({
        queryKey: templateKeys.versions(projectName, templateName),
      })
      qc.invalidateQueries({
        queryKey: templateKeys.detail(projectName, templateName),
      })
    },
  })
}
