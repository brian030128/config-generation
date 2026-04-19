import { useParams } from "react-router-dom"
import { useValues } from "@/hooks/use-values"
import { ValuesEditor } from "@/components/values/values-editor"

export default function ProjectEnvPage() {
  const { name: projectName, env: envName } = useParams<{
    name: string
    env: string
  }>()

  const {
    data: values,
    isLoading: valuesLoading,
    error: valuesError,
  } = useValues(projectName!, envName!)

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">
          {projectName} / {envName}
        </h1>
      </div>

      {valuesLoading && (
        <p className="text-sm text-muted-foreground">Loading values...</p>
      )}

      {valuesError && !valuesLoading && (
        <div className="rounded-lg border border-dashed p-6 text-center text-muted-foreground">
          <p>No values defined for this environment yet.</p>
          <p className="text-sm mt-1">
            Add values in the workspace to create the first version.
          </p>
        </div>
      )}

      <ValuesEditor
        projectName={projectName!}
        envName={envName!}
        values={valuesError ? null : (values ?? null)}
      />
    </div>
  )
}
