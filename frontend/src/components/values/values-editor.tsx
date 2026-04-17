import { useState, useEffect } from "react"
import { toast } from "sonner"
import { useAppendValuesVersion } from "@/hooks/use-values"
import { useTemplateVariables } from "@/hooks/use-templates"
import type { ProjectConfigValues } from "@/api/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"

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
  const [commitMsg, setCommitMsg] = useState("")
  const appendVersion = useAppendValuesVersion(projectName, templateName, envName)
  const { data: varsData, isLoading: varsLoading } = useTemplateVariables(projectName, templateName)

  const variables = varsData?.variables ?? []

  // Initialize payload from template variables, using existing values or defaults
  useEffect(() => {
    if (variables.length === 0) return
    const newPayload: Record<string, unknown> = {}
    for (const v of variables) {
      if (values?.payload && v.name in values.payload) {
        newPayload[v.name] = values.payload[v.name]
      } else if (v.default !== undefined) {
        newPayload[v.name] = v.default
      } else {
        newPayload[v.name] = ""
      }
    }
    setPayload(newPayload)
  }, [varsData, values?.id])

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

  const canSave = variables.length > 0 && !hasEmptyValues()

  function handleSave() {
    appendVersion.mutate(
      { payload, commit_message: commitMsg.trim() || undefined },
      {
        onSuccess: () => {
          toast.success("Values saved")
          setCommitMsg("")
        },
        onError: (err) => {
          toast.error("Failed to save values", {
            description: (err as Error).message,
          })
        },
      },
    )
  }

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
        <div className="grid grid-cols-[1fr_2fr] gap-2 border-b bg-muted/50 px-4 py-2 text-sm font-medium text-muted-foreground">
          <span>Key</span>
          <span>Value</span>
        </div>
        {variables.map((v) => (
          <div
            key={v.name}
            className="grid grid-cols-[1fr_2fr] items-center gap-2 border-b px-4 py-2 last:border-0"
          >
            <span className="font-mono text-sm">{v.name}</span>
            <Input
              className="font-mono text-sm"
              value={String(payload[v.name] ?? "")}
              onChange={(e) => handleChange(v.name, e.target.value)}
              placeholder={v.default !== undefined ? `default: ${v.default}` : undefined}
            />
          </div>
        ))}
      </div>

      <div className="flex items-end gap-3">
        <div className="flex-1 space-y-1">
          <Label htmlFor="val-commit" className="text-xs">
            Commit Message
          </Label>
          <Input
            id="val-commit"
            value={commitMsg}
            onChange={(e) => setCommitMsg(e.target.value)}
            placeholder="Optional commit message"
            className="text-sm"
          />
        </div>
        <Button
          onClick={handleSave}
          disabled={!canSave || appendVersion.isPending}
          size="sm"
        >
          {appendVersion.isPending ? "Saving..." : "Save"}
        </Button>
        <Tooltip>
          <TooltipTrigger asChild>
            <span>
              <Button size="sm" variant="outline" disabled>
                Save to PR
              </Button>
            </span>
          </TooltipTrigger>
          <TooltipContent>Coming in a future update</TooltipContent>
        </Tooltip>
      </div>
    </div>
  )
}
