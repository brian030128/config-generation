import { useState, useEffect } from "react"
import { useNavigate } from "react-router-dom"
import { useTemplateVariables } from "@/hooks/use-templates"
import type { ProjectConfigValues } from "@/api/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import {
  parseReference,
} from "./reference-selector"
import { ExternalLink } from "lucide-react"

interface ValuesEditorProps {
  projectName: string
  templateName: string
  envName: string
  values: ProjectConfigValues | null
}

export function ValuesEditor({
  projectName,
  templateName,
  envName,
  values,
}: ValuesEditorProps) {
  const [payload, setPayload] = useState<Record<string, unknown>>({})
  const [refMode, setRefMode] = useState<Record<string, boolean>>({})
  const [refState, setRefState] = useState<Record<string, { group: string; key: string }>>({})
  const { data: varsData, isLoading: varsLoading } = useTemplateVariables(projectName, templateName)
  const navigate = useNavigate()

  const variables = varsData?.variables ?? []

  useEffect(() => {
    if (variables.length === 0) return
    const newPayload: Record<string, unknown> = {}
    const newRefMode: Record<string, boolean> = {}
    const newRefState: Record<string, { group: string; key: string }> = {}
    for (const v of variables) {
      if (values?.payload && v.name in values.payload) {
        newPayload[v.name] = values.payload[v.name]
      } else if (v.default !== undefined) {
        newPayload[v.name] = v.default
      } else {
        newPayload[v.name] = ""
      }
      const ref = parseReference(String(newPayload[v.name] ?? ""))
      if (ref) {
        newRefMode[v.name] = true
        newRefState[v.name] = ref
      }
    }
    setPayload(newPayload)
    setRefMode(newRefMode)
    setRefState(newRefState)
  }, [varsData, values?.id])

  if (varsLoading) {
    return <p className="text-sm text-muted-foreground">Loading template variables...</p>
  }

  if (variables.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        This template has no variables to configure.
      </p>
    )
  }

  return (
    <div className="space-y-4">
      {values && (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <span>Version:</span>
          <Badge variant="outline">v{values.version_id}</Badge>
        </div>
      )}

      <div className="rounded-lg border">
        <div className="grid grid-cols-[1fr_2fr_auto] gap-2 border-b bg-muted/50 px-4 py-2 text-sm font-medium text-muted-foreground">
          <span>Key</span>
          <span>Value</span>
          <span>Mode</span>
        </div>
        {variables.map((v) => {
          const isRef = !!refMode[v.name]
          const ref = refState[v.name] ?? { group: "", key: "" }

          return (
            <div
              key={v.name}
              className="grid grid-cols-[1fr_2fr_auto] items-center gap-2 border-b px-4 py-2 last:border-0"
            >
              <span className="font-mono text-sm">{v.name}</span>
              {isRef ? (
                <span className="font-mono text-sm text-muted-foreground">
                  {`\${${ref.group}.${ref.key}}`}
                </span>
              ) : (
                <Input
                  className="font-mono text-sm"
                  value={String(payload[v.name] ?? "")}
                  disabled
                />
              )}
              <Badge variant="outline" className="text-xs">
                {isRef ? "Ref" : "Text"}
              </Badge>
            </div>
          )
        })}
      </div>

      <div className="flex justify-end">
        <Button
          variant="outline"
          size="sm"
          onClick={() => navigate(`/workspace/${projectName}/env/${envName}`)}
        >
          <ExternalLink className="mr-2 h-3 w-3" />
          Edit in Workspace
        </Button>
      </div>
    </div>
  )
}
