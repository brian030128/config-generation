import { Link } from "react-router-dom"
import { useEnvironments } from "@/hooks/use-environments"
import { useQueries } from "@tanstack/react-query"
import { valuesApi } from "@/api/values"
import { AddEnvironmentDialog } from "./add-environment-dialog"
import { ChevronRight } from "lucide-react"

interface EnvironmentListProps {
  projectName: string
  templateCount: number
}

export function EnvironmentList({
  projectName,
  templateCount,
}: EnvironmentListProps) {
  const { data: envData, isLoading: envsLoading } = useEnvironments()
  const environments = envData?.items ?? []

  // For each environment, check if it has values for this project
  const valueQueries = useQueries({
    queries: environments.map((env) => ({
      queryKey: ["projects", projectName, "envs", env.name, "values"] as const,
      queryFn: () => valuesApi.listForProjectEnv(projectName, env.name),
      enabled: environments.length > 0,
    })),
  })

  const envsWithStatus = environments.map((env, i) => ({
    env,
    valuesCount: valueQueries[i]?.data?.count ?? 0,
    isLoading: valueQueries[i]?.isLoading ?? true,
  }))

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium">Environments</h3>
        <AddEnvironmentDialog projectName={projectName} />
      </div>

      {envsLoading && (
        <p className="text-sm text-muted-foreground">
          Loading environments...
        </p>
      )}

      {!envsLoading && envsWithStatus.length === 0 && (
        <p className="text-sm text-muted-foreground">
          No environments configured yet. Add one to start defining values.
        </p>
      )}

      <div className="space-y-2">
        {envsWithStatus.map(({ env, valuesCount, isLoading }) => (
          <Link
            key={env.id}
            to={`/projects/${projectName}/env/${env.name}`}
            className="flex items-center justify-between rounded-lg border px-4 py-3 transition-colors hover:bg-accent/50"
          >
            <div className="flex items-center gap-4">
              <span className="font-medium">{env.name}</span>
              {!isLoading && (
                <span className="text-xs text-muted-foreground">
                  {valuesCount}/{templateCount} templates configured
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
