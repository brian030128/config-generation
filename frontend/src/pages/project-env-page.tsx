import { useState, useEffect } from "react"
import { useParams } from "react-router-dom"
import { useTemplates } from "@/hooks/use-templates"
import { useValues } from "@/hooks/use-values"
import { TemplateSelector } from "@/components/values/template-selector"
import { ValuesEditor } from "@/components/values/values-editor"

export default function ProjectEnvPage() {
  const { name: projectName, env: envName } = useParams<{
    name: string
    env: string
  }>()
  const { data: templates, isLoading: templatesLoading } = useTemplates(
    projectName!,
  )
  const [selectedTemplate, setSelectedTemplate] = useState("")

  // Auto-select the first template
  useEffect(() => {
    if (!selectedTemplate && templates?.items.length) {
      setSelectedTemplate(templates.items[0].template_name)
    }
  }, [templates, selectedTemplate])

  const {
    data: values,
    isLoading: valuesLoading,
    error: valuesError,
  } = useValues(projectName!, selectedTemplate, envName!)

  if (templatesLoading) {
    return <p className="text-muted-foreground">Loading...</p>
  }

  const hasTemplates = (templates?.items.length ?? 0) > 0

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">
          {projectName} / {envName}
        </h1>
      </div>

      {!hasTemplates && (
        <p className="text-muted-foreground">
          No templates found for this project. Create a template first.
        </p>
      )}

      {hasTemplates && (
        <>
          <div className="flex items-center gap-4">
            <span className="text-sm font-medium">Template:</span>
            <TemplateSelector
              projectName={projectName!}
              value={selectedTemplate}
              onChange={setSelectedTemplate}
            />
          </div>

          {valuesLoading && (
            <p className="text-sm text-muted-foreground">Loading values...</p>
          )}

          {valuesError && !valuesLoading && (
            <div className="rounded-lg border border-dashed p-6 text-center text-muted-foreground">
              <p>No values defined for this template and environment yet.</p>
              <p className="text-sm mt-1">
                Add keys below and save to create the first version.
              </p>
            </div>
          )}

          <ValuesEditor
            projectName={projectName!}
            templateName={selectedTemplate}
            envName={envName!}
            values={valuesError ? null : (values ?? null)}
          />
        </>
      )}
    </div>
  )
}
