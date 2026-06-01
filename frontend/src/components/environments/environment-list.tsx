import { Link } from "react-router-dom"
import { useEnvironments } from "@/hooks/use-environments"
import { useActiveDraft } from "@/hooks/use-pull-requests"
import { useQueries } from "@tanstack/react-query"
import { valuesApi } from "@/api/values"
import { AddEnvironmentDialog } from "./add-environment-dialog"
import { Badge } from "@/components/ui/badge"
import { ChevronRight, Layers3 } from "lucide-react"

interface EnvironmentListProps {
  projectName: string
  workspaceMode?: boolean
}

function environmentStatusLabel({
  restricted,
  hasValues,
}: {
  restricted: boolean
  hasValues: boolean
}): string {
  if (restricted) return "restricted"
  if (hasValues) return "configured"
  return "not configured"
}

export function EnvironmentList({
  projectName,
  workspaceMode,
}: Readonly<EnvironmentListProps>) {
  const { data: envData, isLoading: envsLoading } = useEnvironments(projectName)
  const environments = envData?.items ?? []

  // Only fetch draft and show staged envs in workspace mode
  const { data: draft } = useActiveDraft(workspaceMode ? projectName : "")
  const stagedEnvs = workspaceMode
    ? (draft?.changes ?? [])
        .filter((c) => c.object_type === "environment")
        .map((c) => {
          try {
            return JSON.parse(c.proposed_payload) as {
              name: string
              description?: string
            }
          } catch {
            return null
          }
        })
        .filter((e): e is { name: string; description?: string } => e !== null)
        .filter((e) => !environments.some((env) => env.name === e.name))
    : []

  // For each existing environment, check if it has values. A 403 here means the
  // caller lacks read access to that env's values — the environment is still
  // listed, just shown as "restricted" rather than a misleading "not configured".
  const valueQueries = useQueries({
    queries: environments.map((env) => ({
      queryKey: ["projects", projectName, "envs", env.name, "values"] as const,
      queryFn: () => valuesApi.getLatest(projectName, env.name),
      enabled: environments.length > 0,
      retry: false,
    })),
  })

  const envsWithStatus = environments.map((env, i) => {
    const q = valueQueries[i]
    const status = (q?.error as { response?: { status?: number } } | undefined)
      ?.response?.status
    return {
      env,
      hasValues: q?.isSuccess ?? false,
      restricted: status === 403,
      isLoading: q?.isLoading ?? true,
      staged: false,
    }
  })

  const allEnvs = [
    ...envsWithStatus,
    ...stagedEnvs.map((e) => ({
      env: {
        id: 0,
        project_id: 0,
        name: e.name,
        description: e.description ?? null,
        created_by: 0,
        created_at: "",
      },
      hasValues: false,
      restricted: false,
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
        <div className="flex min-h-56 items-center justify-center rounded-lg border border-dashed bg-card/40 p-8 text-center">
          <div className="mx-auto flex max-w-sm flex-col items-center gap-3">
            <div className="flex size-11 items-center justify-center rounded-lg border bg-background text-muted-foreground">
              <Layers3 className="h-5 w-5" />
            </div>
            <div className="space-y-1">
              <h4 className="text-base font-semibold">No environments yet</h4>
              <p className="text-sm text-muted-foreground">
                {workspaceMode
                  ? "Add an environment to start defining project values."
                  : "Open the workspace to add environments and stage changes."}
              </p>
            </div>
          </div>
        </div>
      )}

      <div className="space-y-2">
        {allEnvs.map(({ env, hasValues, restricted, isLoading, staged }) => (
          <Link
            key={env.name}
            to={
              workspaceMode
                ? `/workspace/${projectName}/env/${env.name}`
                : `/projects/${projectName}/env/${env.name}`
            }
            className="flex items-center justify-between rounded-lg border px-4 py-3 transition-colors hover:bg-accent/50"
          >
            <div className="flex items-center gap-4">
              <span className="font-medium">{env.name}</span>
              {staged && (
                <Badge variant="secondary">in draft</Badge>
              )}
              {!staged && !isLoading && (
                <span className="text-xs text-muted-foreground">
                  {environmentStatusLabel({ restricted, hasValues })}
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
