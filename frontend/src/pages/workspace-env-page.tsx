import { useState, useEffect, useMemo } from "react"
import type { Dispatch, SetStateAction } from "react"
import { useParams, useNavigate } from "react-router-dom"
import { toast } from "sonner"
import { useProjectVariables } from "@/hooks/use-templates"
import { useValues } from "@/hooks/use-values"
import { useStageValues, useActiveDraft } from "@/hooks/use-pull-requests"
import type { TemplateVariable } from "@/api/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { ArrowLeft, AlertCircle } from "lucide-react"
import { safeString } from "@/lib/utils"
import { ListItemsEditor } from "@/components/values/list-items-editor"
import {
  ReferenceSelector,
  parseReference,
  buildReference,
} from "@/components/values/reference-selector"

type VarKind = "string" | "list" | "conflict"

function varKind(v: TemplateVariable): VarKind {
  return v.kind ?? "string"
}

function modeLabel(isRef: boolean, isList: boolean): string {
  if (isRef) return "Ref"
  return isList ? "List" : "Text"
}

// Detect variable usage in unsaved draft template bodies. The backend computes
// kinds for saved templates; here we approximate the same for templates that
// only exist as staged changes, so list editors show before the draft is merged.
type DraftKind = "string" | "list" | "conflict"

function extractDraftVars(bodies: string[]): TemplateVariable[] {
  const kinds = new Map<string, DraftKind>()
  const defaults = new Map<string, string>()
  const order: string[] = []

  const mark = (name: string, kind: "string" | "list") => {
    const prev = kinds.get(name)
    if (prev === undefined) {
      kinds.set(name, kind)
      order.push(name)
    } else if (prev !== kind && prev !== "conflict") {
      kinds.set(name, "conflict")
    }
  }

  for (const body of bodies) {
    // Plain references: {{ .name }} or {{ .name | default "x" }}
    for (const m of body.matchAll(/\{\{-?\s*\.(\w+)\s*(?:\|[^}]*)?-?\}\}/g)) {
      mark(m[1], "string")
      const d = m[0].match(/\|\s*default\s+"([^"]*)"/)
      if (d && !defaults.has(m[1])) defaults.set(m[1], d[1])
    }
    // Range targets: {{ range .name }} or {{ range $k, $v := .name }}
    for (const m of body.matchAll(/\{\{-?\s*range\b[^}]*?\.(\w+)\s*-?\}\}/g)) {
      mark(m[1], "list")
    }
  }

  return order.map((name) => ({
    name,
    kind: kinds.get(name),
    default: defaults.get(name),
  }))
}

// Coerce a stored value into the list editor's string[] shape without losing a
// pre-existing scalar.
function toListItems(val: unknown): string[] {
  if (Array.isArray(val)) return val.map((x) => safeString(x))
  if (val === "" || val === null || val === undefined) return []
  return [safeString(val)]
}

export default function WorkspaceEnvPage() {
  const { name: projectName = "", env: envName = "" } = useParams<{
    name: string
    env: string
  }>()
  const navigate = useNavigate()
  const { data: draft } = useActiveDraft(projectName)
  const stageValues = useStageValues(projectName)

  const { data: values } = useValues(projectName, envName)

  const { data: varsData, isLoading: varsLoading } = useProjectVariables(projectName)

  // Merge DB variables with variables from staged templates in the draft
  const variables = useMemo(() => {
    const dbVars = varsData?.variables ?? []
    const seen = new Set(dbVars.map((v) => v.name))

    const stagedBodies = (draft?.changes ?? [])
      .filter((c) => c.object_type === "template")
      .map((c) => c.proposed_payload)

    const draftVars = extractDraftVars(stagedBodies).filter((v) => !seen.has(v.name))

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
      const raw = source && v.name in source ? source[v.name] : v.default ?? ""

      // Reference mode is only meaningful for a ${group.key} string.
      const ref = typeof raw === "string" ? parseReference(raw) : null
      if (ref) {
        newRefMode[v.name] = true
        newRefState[v.name] = ref
        newPayload[v.name] = raw
        continue
      }

      newPayload[v.name] = varKind(v) === "list" ? toListItems(raw) : raw
    }
    setPayload(newPayload)
    setRefMode(newRefMode)
    setRefState(newRefState)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [varsData, draft, values?.id, stagedChange?.id])

  function handleChange(key: string, newValue: unknown) {
    setPayload((prev) => ({ ...prev, [key]: newValue }))
  }

  function toggleRef(v: TemplateVariable, isRef: boolean) {
    if (isRef) {
      setRefMode((prev) => ({ ...prev, [v.name]: false }))
      setRefState((prev) => ({ ...prev, [v.name]: { group: "", key: "" } }))
      handleChange(v.name, varKind(v) === "list" ? [] : "")
    } else {
      setRefMode((prev) => ({ ...prev, [v.name]: true }))
      handleChange(v.name, "")
    }
  }

  const conflicts = variables.filter((v) => varKind(v) === "conflict")

  function hasEmptyValues(): boolean {
    for (const v of variables) {
      if (varKind(v) === "conflict") continue // blocks save separately
      const val = payload[v.name]
      if (Array.isArray(val)) {
        if (val.length === 0 || val.some((it) => it.trim() === "")) return true
      } else if (val === "" || val === null || val === undefined) {
        return true
      }
    }
    return false
  }

  function handleSave() {
    stageValues.mutate(
      {
        envName: envName,
        payload,
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

  const canSave =
    variables.length > 0 && !hasEmptyValues() && conflicts.length === 0

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

      {conflicts.length > 0 && (
        <div className="flex items-start gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
          <span>
            {conflicts.map((v) => v.name).join(", ")}{" "}
            {conflicts.length === 1 ? "is" : "are"} used as both a list and a
            string across your templates. Fix the template usage before setting
            a value.
          </span>
        </div>
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
              const kind = varKind(v)
              const isConflict = kind === "conflict"
              const isList = kind === "list"
              const isRef = !!refMode[v.name]
              const ref = refState[v.name] ?? { group: "", key: "" }

              return (
                <div
                  key={v.name}
                  className="grid grid-cols-[1fr_2fr_auto] items-start gap-2 border-b px-4 py-2 last:border-0"
                >
                  <span className="font-mono text-sm">{v.name}</span>
                  {renderValueCell({
                    v,
                    isConflict,
                    isRef,
                    isList,
                    ref,
                    payload,
                    handleChange,
                    setRefState,
                  })}
                  {isConflict ? (
                    <span />
                  ) : (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => toggleRef(v, isRef)}
                    >
                      <Badge variant="outline" className="text-xs">
                        {modeLabel(isRef, isList)}
                      </Badge>
                    </Button>
                  )}
                </div>
              )
            })}
          </div>

          <div className="flex justify-end">
            <Button
              onClick={handleSave}
              disabled={!canSave || stageValues.isPending}
              size="sm"
            >
              {stageValues.isPending ? "Saving..." : "Save to Draft"}
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}

// renderValueCell picks the editor for a variable row: a conflict message, a
// global-values reference selector, a list editor, or a plain text input.
function renderValueCell({
  v,
  isConflict,
  isRef,
  isList,
  ref,
  payload,
  handleChange,
  setRefState,
}: {
  v: TemplateVariable
  isConflict: boolean
  isRef: boolean
  isList: boolean
  ref: { group: string; key: string }
  payload: Record<string, unknown>
  handleChange: (key: string, value: unknown) => void
  setRefState: Dispatch<
    SetStateAction<Record<string, { group: string; key: string }>>
  >
}) {
  if (isConflict) {
    return (
      <span className="text-sm text-destructive">
        Conflicting usage — cannot set a value.
      </span>
    )
  }

  if (isRef) {
    return (
      <ReferenceSelector
        group={ref.group}
        keyName={ref.key}
        onGroupChange={(g) => {
          setRefState((prev) => ({ ...prev, [v.name]: { group: g, key: "" } }))
          handleChange(v.name, "")
        }}
        onKeyChange={(k) => {
          setRefState((prev) => ({ ...prev, [v.name]: { group: ref.group, key: k } }))
          handleChange(v.name, buildReference(ref.group, k))
        }}
      />
    )
  }

  if (isList) {
    return (
      <ListItemsEditor
        items={Array.isArray(payload[v.name]) ? (payload[v.name] as string[]) : []}
        onChange={(items) => handleChange(v.name, items)}
      />
    )
  }

  return (
    <Input
      className="font-mono text-sm"
      value={safeString(payload[v.name])}
      onChange={(e) => handleChange(v.name, e.target.value)}
      placeholder={v.default === undefined ? undefined : `default: ${v.default}`}
    />
  )
}
