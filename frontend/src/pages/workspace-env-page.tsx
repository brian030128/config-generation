import { useState, useEffect, useMemo } from "react"
import { useParams, useNavigate } from "react-router-dom"
import { toast } from "sonner"
import { useProjectVariables } from "@/hooks/use-templates"
import { useValues } from "@/hooks/use-values"
import { useStageChange, useActiveDraft } from "@/hooks/use-pull-requests"
import type { TemplateVariable } from "@/api/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { ArrowLeft } from "lucide-react"
import {
  ReferenceSelector,
  parseReference,
  buildReference,
} from "@/components/values/reference-selector"

export default function WorkspaceEnvPage() {
  const { name: projectName, env: envName } = useParams<{
    name: string
    env: string
  }>()
  const navigate = useNavigate()
  const { data: draft } = useActiveDraft(projectName!)
  const stageChange = useStageChange(projectName!)

  const { data: values } = useValues(projectName!, envName!)

  const { data: varsData, isLoading: varsLoading } = useProjectVariables(projectName!)

  // Merge DB variables with variables from staged templates in the draft
  const variables = useMemo(() => {
    const dbVars = varsData?.variables ?? []
    const seen = new Set(dbVars.map((v) => v.name))
    const draftVars: TemplateVariable[] = []

    // Extract variables from staged template bodies
    const stagedTemplates = (draft?.changes ?? []).filter(
      (c) => c.object_type === "template",
    )
    for (const c of stagedTemplates) {
      const matches = c.proposed_payload.matchAll(/\{\{\s*\.(\w+)\s*(?:\|[^}]*)?\}\}/g)
      for (const m of matches) {
        const name = m[1]
        if (!seen.has(name)) {
          seen.add(name)
          // Try to extract default: {{ .name | default "value" }}
          const defaultMatch = m[0].match(/\|\s*default\s+"([^"]*)"/)
          draftVars.push({
            name,
            default: defaultMatch ? defaultMatch[1] : undefined,
          })
        }
      }
    }

    return [...dbVars, ...draftVars]
  }, [varsData, draft])

  const [payload, setPayload] = useState<Record<string, unknown>>({})
  const [refMode, setRefMode] = useState<Record<string, boolean>>({})
  const [refState, setRefState] = useState<Record<string, { group: string; key: string }>>({})
  // Check if there's already a staged change for this env in the draft
  const stagedChange = draft?.changes?.find(
    (c) =>
      c.object_type === "values" &&
      c.environment_name === envName,
  )

  // Initialize payload from staged change, existing values, or defaults
  useEffect(() => {
    if (variables.length === 0) return
    const source = stagedChange
      ? JSON.parse(stagedChange.proposed_payload)
      : values?.payload

    const newPayload: Record<string, unknown> = {}
    const newRefMode: Record<string, boolean> = {}
    const newRefState: Record<string, { group: string; key: string }> = {}
    for (const v of variables) {
      if (source && v.name in source) {
        newPayload[v.name] = source[v.name]
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
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [varsData, draft, values?.id, stagedChange?.id])

  function handleChange(key: string, newValue: unknown) {
    setPayload((prev) => ({ ...prev, [key]: newValue }))
  }

  function hasEmptyValues(): boolean {
    for (const v of variables) {
      const val = payload[v.name]
      if (val === "" || val === null || val === undefined) return true
    }
    return false
  }

  function handleSave() {
    stageChange.mutate(
      {
        object_type: "values",
        environment_name: envName,
        proposed_payload: JSON.stringify(payload),
      },
      {
        onSuccess: () => {
          toast.success("Change staged in draft PR")
        },
        onError: (err) => {
          toast.error("Failed to stage change", {
            description: (err as Error).message,
          })
        },
      },
    )
  }

  if (varsLoading) {
    return <p className="text-muted-foreground">Loading...</p>
  }

  const canSave = variables.length > 0 && !hasEmptyValues()

  return (
    <div className="space-y-6">
      <button
        onClick={() => navigate(`/workspace/${projectName}`)}
        className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="h-4 w-4" />
        {projectName}
      </button>

      <div>
        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-semibold">
            {projectName} / {envName}
          </h1>
          {stagedChange && (
            <Badge variant="secondary">modified in draft</Badge>
          )}
        </div>
      </div>

      {variables.length === 0 && (
        <p className="text-muted-foreground">
          No templates found or no variables to configure.
        </p>
      )}

      {values && (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <span>Live version:</span>
          <Badge variant="outline">v{values.version_id}</Badge>
        </div>
      )}

      {variables.length > 0 && (
        <div className="space-y-4">
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
                    <ReferenceSelector
                      group={ref.group}
                      keyName={ref.key}
                      onGroupChange={(g) => {
                        const newRef = { group: g, key: "" }
                        setRefState((prev) => ({ ...prev, [v.name]: newRef }))
                        handleChange(v.name, "")
                      }}
                      onKeyChange={(k) => {
                        const newRef = { group: ref.group, key: k }
                        setRefState((prev) => ({ ...prev, [v.name]: newRef }))
                        handleChange(v.name, buildReference(ref.group, k))
                      }}
                    />
                  ) : (
                    <Input
                      className="font-mono text-sm"
                      value={String(payload[v.name] ?? "")}
                      onChange={(e) => handleChange(v.name, e.target.value)}
                      placeholder={v.default !== undefined ? `default: ${v.default}` : undefined}
                    />
                  )}
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                      if (isRef) {
                        setRefMode((prev) => ({ ...prev, [v.name]: false }))
                        setRefState((prev) => ({ ...prev, [v.name]: { group: "", key: "" } }))
                        handleChange(v.name, "")
                      } else {
                        setRefMode((prev) => ({ ...prev, [v.name]: true }))
                        handleChange(v.name, "")
                      }
                    }}
                  >
                    <Badge variant="outline" className="text-xs">
                      {isRef ? "Ref" : "Text"}
                    </Badge>
                  </Button>
                </div>
              )
            })}
          </div>

          <div className="flex justify-end">
            <Button
              onClick={handleSave}
              disabled={!canSave || stageChange.isPending}
              size="sm"
            >
              {stageChange.isPending ? "Saving..." : "Save to Draft"}
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
