import { useState } from "react"
import { useNavigate } from "react-router-dom"
import { useTemplates } from "@/hooks/use-templates"
import { formatRelativeTime } from "@/lib/utils"
import { TemplateEditor } from "./template-editor"
import { CreateTemplateDialog } from "./create-template-dialog"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Pencil, ExternalLink, ChevronRight, ChevronDown } from "lucide-react"

interface TemplateListProps {
  projectName: string
  workspaceMode?: boolean
  modifiedTemplates?: Set<string | null>
}

export function TemplateList({ projectName, workspaceMode, modifiedTemplates }: TemplateListProps) {
  const { data, isLoading } = useTemplates(projectName)
  const [editingTemplate, setEditingTemplate] = useState<string | null>(null)
  const [expandedTemplate, setExpandedTemplate] = useState<string | null>(null)
  const navigate = useNavigate()

  const templates = data?.items ?? []

  function toggleExpand(name: string) {
    setExpandedTemplate(expandedTemplate === name ? null : name)
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium">Templates</h3>
        {workspaceMode && <CreateTemplateDialog projectName={projectName} />}
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
        {templates.map((t) => {
          const isExpanded = !workspaceMode && expandedTemplate === t.template_name
          const isEditing = workspaceMode && editingTemplate === t.template_name

          return (
            <div key={t.template_name}>
              <div className="flex items-center justify-between rounded-lg border px-4 py-3">
                <div className="flex items-center gap-4">
                  {!workspaceMode && (
                    <button
                      onClick={() => toggleExpand(t.template_name)}
                      className="text-muted-foreground hover:text-foreground"
                    >
                      {isExpanded
                        ? <ChevronDown className="h-4 w-4" />
                        : <ChevronRight className="h-4 w-4" />}
                    </button>
                  )}
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
                {workspaceMode ? (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() =>
                      setEditingTemplate(
                        isEditing ? null : t.template_name,
                      )
                    }
                  >
                    <Pencil className="mr-2 h-3 w-3" />
                    {isEditing ? "Close" : "Edit"}
                  </Button>
                ) : (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => navigate(`/workspace/${projectName}`)}
                  >
                    <ExternalLink className="mr-2 h-3 w-3" />
                    Edit in Workspace
                  </Button>
                )}
              </div>

              {/* Read-only collapsible preview on project pages */}
              {isExpanded && (
                <pre className="mt-1 max-h-[400px] overflow-auto rounded-lg border bg-muted/30 px-4 py-3 text-sm font-mono whitespace-pre-wrap">
                  {t.body}
                </pre>
              )}

              {/* Editable editor in workspace mode */}
              {isEditing && (
                <div className="mt-2">
                  <TemplateEditor
                    projectName={projectName}
                    templateName={t.template_name}
                    onClose={() => setEditingTemplate(null)}
                  />
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
