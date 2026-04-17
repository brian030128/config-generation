import { useState, useEffect } from "react"
import { toast } from "sonner"
import { useAppendValuesVersion } from "@/hooks/use-values"
import type { ProjectConfigValues } from "@/api/types"
import { ValueRow } from "./value-row"
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

  useEffect(() => {
    if (values?.payload) {
      setPayload(structuredClone(values.payload))
    }
  }, [values?.id])

  const entries = Object.entries(payload)

  function handleChange(key: string, newValue: unknown) {
    setPayload((prev) => ({ ...prev, [key]: newValue }))
  }

  function handleDelete(key: string) {
    setPayload((prev) => {
      const copy = { ...prev }
      delete copy[key]
      return copy
    })
  }

  function handleAddKey() {
    const newKey = `new_key_${entries.length}`
    setPayload((prev) => ({ ...prev, [newKey]: "" }))
  }

  // Check if any leaf value is empty
  function hasEmptyValues(obj: Record<string, unknown>): boolean {
    for (const v of Object.values(obj)) {
      if (typeof v === "object" && v !== null && !Array.isArray(v)) {
        if (hasEmptyValues(v as Record<string, unknown>)) return true
      } else if (v === "" || v === null || v === undefined) {
        return true
      }
    }
    return false
  }

  const canSave = entries.length > 0 && !hasEmptyValues(payload)

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

  return (
    <div className="space-y-4">
      {values && (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <span>Version:</span>
          <Badge variant="outline">v{values.version_id}</Badge>
        </div>
      )}

      <div className="rounded-lg border p-4">
        {entries.length === 0 && (
          <p className="text-sm text-muted-foreground">
            No values defined. Add a key to get started.
          </p>
        )}

        {entries.map(([key, val]) => (
          <ValueRow
            key={key}
            keyName={key}
            value={val}
            onChange={(nv) => handleChange(key, nv)}
            onDelete={() => handleDelete(key)}
          />
        ))}

        <Button
          variant="ghost"
          size="sm"
          className="mt-2 text-xs"
          onClick={handleAddKey}
        >
          + Add Key
        </Button>
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
