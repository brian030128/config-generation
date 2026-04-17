import { useState, useEffect } from "react"
import { toast } from "sonner"
import { useAppendGlobalValueVersion } from "@/hooks/use-global-values"
import type { GlobalValues } from "@/api/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { Trash2, Eye, EyeOff } from "lucide-react"

const SENSITIVE_PATTERNS = ["password", "secret", "token", "key"]

function isSensitiveKey(key: string): boolean {
  const lower = key.toLowerCase()
  return SENSITIVE_PATTERNS.some((p) => lower.includes(p))
}

interface KvEditorProps {
  name: string
  data: GlobalValues
  readOnly?: boolean
}

export function KvEditor({ name, data, readOnly = false }: KvEditorProps) {
  const [payload, setPayload] = useState<
    Record<string, string | number | boolean | null>
  >({})
  const [commitMsg, setCommitMsg] = useState("")
  const [visibleKeys, setVisibleKeys] = useState<Set<string>>(new Set())
  const appendVersion = useAppendGlobalValueVersion(name)

  useEffect(() => {
    setPayload(structuredClone(data.payload))
  }, [data.id])

  const entries = Object.entries(payload)

  function handleKeyChange(oldKey: string, newKey: string) {
    const newPayload: typeof payload = {}
    for (const [k, v] of Object.entries(payload)) {
      newPayload[k === oldKey ? newKey : k] = v
    }
    setPayload(newPayload)
  }

  function handleValueChange(key: string, value: string) {
    setPayload((prev) => ({ ...prev, [key]: value }))
  }

  function handleDelete(key: string) {
    setPayload((prev) => {
      const copy = { ...prev }
      delete copy[key]
      return copy
    })
  }

  function handleAdd() {
    const newKey = `new_key_${entries.length}`
    setPayload((prev) => ({ ...prev, [newKey]: "" }))
  }

  function toggleVisibility(key: string) {
    setVisibleKeys((prev) => {
      const next = new Set(prev)
      if (next.has(key)) {
        next.delete(key)
      } else {
        next.add(key)
      }
      return next
    })
  }

  const hasEmpty = entries.some(
    ([, v]) => v === "" || v === null || v === undefined,
  )
  const canSave = !readOnly && entries.length > 0 && !hasEmpty

  function handleSave() {
    appendVersion.mutate(
      { payload, commit_message: commitMsg.trim() || undefined },
      {
        onSuccess: () => {
          toast.success("Global values saved")
          setCommitMsg("")
        },
        onError: (err) => {
          toast.error("Failed to save", {
            description: (err as Error).message,
          })
        },
      },
    )
  }

  return (
    <div className="space-y-4">
      <div className="rounded-lg border">
        <div className="grid grid-cols-[1fr_2fr_auto] gap-2 border-b bg-muted/50 px-4 py-2 text-sm font-medium text-muted-foreground">
          <span>Key</span>
          <span>Value</span>
          <span />
        </div>
        {entries.map(([key, val]) => {
          const sensitive = isSensitiveKey(key)
          const visible = visibleKeys.has(key)
          return (
            <div
              key={key}
              className="grid grid-cols-[1fr_2fr_auto] items-center gap-2 border-b px-4 py-2 last:border-0"
            >
              <Input
                value={key}
                onChange={(e) => handleKeyChange(key, e.target.value)}
                className="font-mono text-sm"
                disabled={readOnly}
              />
              <div className="flex items-center gap-1">
                <Input
                  type={sensitive && !visible ? "password" : "text"}
                  value={String(val ?? "")}
                  onChange={(e) => handleValueChange(key, e.target.value)}
                  className="font-mono text-sm"
                  disabled={readOnly}
                />
                {sensitive && (
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 shrink-0"
                    onClick={() => toggleVisibility(key)}
                    type="button"
                  >
                    {visible ? (
                      <EyeOff className="h-3 w-3" />
                    ) : (
                      <Eye className="h-3 w-3" />
                    )}
                  </Button>
                )}
              </div>
              {!readOnly && (
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7 text-muted-foreground hover:text-destructive"
                  onClick={() => handleDelete(key)}
                >
                  <Trash2 className="h-3 w-3" />
                </Button>
              )}
            </div>
          )
        })}
        {entries.length === 0 && (
          <p className="px-4 py-4 text-sm text-muted-foreground">
            No keys defined.
          </p>
        )}
      </div>

      {!readOnly && (
        <>
          <Button variant="ghost" size="sm" className="text-xs" onClick={handleAdd}>
            + Add Key
          </Button>

          <div className="flex items-end gap-3">
            <div className="flex-1 space-y-1">
              <Label htmlFor="gv-commit" className="text-xs">
                Commit Message
              </Label>
              <Input
                id="gv-commit"
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
        </>
      )}
    </div>
  )
}
