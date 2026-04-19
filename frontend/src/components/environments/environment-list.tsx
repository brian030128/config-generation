import { Link } from "react-router-dom"
import { useEnvironments } from "@/hooks/use-environments"
import { useActiveDraft } from "@/hooks/use-pull-requests"
import { useQueries } from "@tanstack/react-query"
import { valuesApi } from "@/api/values"
import { AddEnvironmentDialog } from "./add-environment-dialog"
import { Badge } from "@/components/ui/badge"
import { ChevronRight } from "lucide-react"

interface EnvironmentListProps {
  projectName: string
  workspaceMode?: boolean
}

export function EnvironmentList({
  projectName,
  workspaceMode,
}: EnvironmentListProps) {
  const { data: envData, isLoading: envsLoading } = useEnvironments(projectName)
  const environments = envData?.items ?? []

  // Only fetch draft and show staged envs in workspace mode
  const { data: draft } = useActiveDraft(workspaceMode ? projectName : "")
  const stagedEnvs = workspaceMode
    ? (draft?.changes ?? [])
        .filter((c) => c.object_type === "environment")
        .map((c) => {
          try {
            return JSON.parse(c.proposed_payload) as { name: string; description?: string }
          } catch {
            return null
          }
        })
        .filter((e): e is { name: string; description?: string } => e !== null)
        .filter((e) => !environments.some((env) => env.name === e.name))
    : []

  // For each existing environment, check if it has values
  const valueQueries = useQueries({
    queries: environments.map((env) => ({
      queryKey: ["projects", projectName, "envs", env.name, "values"] as const,
      queryFn: () => valuesApi.getLatest(projectName, env.name),
      enabled: environments.length > 0,
    })),
  })

  const envsWithStatus = environments.map((env, i) => ({
    env,
    hasValues: valueQueries[i]?.isSuccess ?? false,
    isLoading: valueQueries[i]?.isLoading ?? true,
    staged: false,
  }))

  const allEnvs = [
    ...envsWithStatus,
    ...stagedEnvs.map((e) => ({
      env: { id: 0, project_id: 0, name: e.name, description: e.description ?? null, created_by: 0, created_at: "" },
      hasValues: false,
      isLoading: false,
      staged: true,
    })),
  ]

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium">Environments</h3>
        {workspaceMode && <AddEnvironmentDialog projectName={projectName} />}
      </div>

      {envsLoading && (
        <p className="text-sm text-muted-foreground">
          Loading environments...
        </p>
      )}

      {!envsLoading && allEnvs.length === 0 && (
        <p className="text-sm text-muted-foreground">
          No environments configured yet. Add one to start defining values.
        </p>
      )}

      <div className="space-y-2">
        {allEnvs.map(({ env, hasValues, isLoading, staged }) => (
          <Link
            key={env.name}
            to={workspaceMode ? `/workspace/${projectName}/env/${env.name}` : `/projects/${projectName}/env/${env.name}`}
            className="flex items-center justify-between rounded-lg border px-4 py-3 transition-colors hover:bg-accent/50"
          >
            <div className="flex items-center gap-4">
              <span className="font-medium">{env.name}</span>
              {staged && (
                <Badge variant="secondary">in draft</Badge>
              )}
              {!staged && !isLoading && (
                <span className="text-xs text-muted-foreground">
                  {hasValues ? "configured" : "not configured"}
                </span>
              )}
            </div>
            <ChevronRight className="h-4 w-4 text-muted-foreground" />
          </Link>
        ))}
      </div>
    </div>
  )
}
