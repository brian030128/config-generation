import { useState } from "react"
import { useTemplates } from "@/hooks/use-templates"
import { formatRelativeTime } from "@/lib/utils"
import { TemplateEditor } from "./template-editor"
import { CreateTemplateDialog } from "./create-template-dialog"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Pencil } from "lucide-react"

interface TemplateListProps {
  projectName: string
  workspaceMode?: boolean
  modifiedTemplates?: Set<string | null>
}

export function TemplateList({ projectName, workspaceMode, modifiedTemplates }: TemplateListProps) {
  const { data, isLoading } = useTemplates(projectName)
  const [editingTemplate, setEditingTemplate] = useState<string | null>(null)

  const templates = data?.items ?? []

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium">Templates</h3>
        <CreateTemplateDialog projectName={projectName} />
      </div>

      {isLoading && (
        <p className="text-sm text-muted-foreground">Loading templates...</p>
      )}

      {!isLoading && templates.length === 0 && (
        <p className="text-sm text-muted-foreground">
          No templates yet. Create one to get started.
        </p>
      )}

      <div className="space-y-2">
        {templates.map((t) => (
          <div key={t.template_name}>
            <div className="flex items-center justify-between rounded-lg border px-4 py-3">
              <div className="flex items-center gap-4">
                <span className="font-mono text-sm">{t.template_name}</span>
                <span className="text-xs text-muted-foreground">
                  v{t.version_id}
                </span>
                <span className="text-xs text-muted-foreground">
                  {formatRelativeTime(t.created_at)}
                </span>
                {modifiedTemplates?.has(t.template_name) && (
                  <Badge variant="secondary" className="text-xs">modified</Badge>
                )}
              </div>
              <Button
                variant="ghost"
                size="sm"
                onClick={() =>
                  setEditingTemplate(
                    editingTemplate === t.template_name
                      ? null
                      : t.template_name,
                  )
                }
              >
                <Pencil className="mr-2 h-3 w-3" />
                {editingTemplate === t.template_name ? "Close" : "Edit"}
              </Button>
            </div>
            {editingTemplate === t.template_name && (
              <div className="mt-2">
                <TemplateEditor
                  projectName={projectName}
                  templateName={t.template_name}
                  onClose={() => setEditingTemplate(null)}
                />
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}
