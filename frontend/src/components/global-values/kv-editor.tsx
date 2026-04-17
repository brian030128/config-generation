import { useState, useEffect } from "react"
import { useNavigate } from "react-router-dom"
import type { GlobalValues } from "@/api/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Trash2 } from "lucide-react"

interface KvEditorProps {
  name: string
  data: GlobalValues
  readOnly?: boolean
}

export function KvEditor({ name, data, readOnly = false }: KvEditorProps) {
  type Entry = [string, string | number | boolean | null]
  const [entries, setEntries] = useState<Entry[]>([])
  const navigate = useNavigate()

  useEffect(() => {
    setEntries(Object.entries(structuredClone(data.payload)))
  }, [data.id])

  function handleKeyChange(index: number, newKey: string) {
    setEntries((prev) => prev.map((e, i) => (i === index ? [newKey, e[1]] : e)))
  }

  function handleValueChange(index: number, value: string) {
    setEntries((prev) => prev.map((e, i) => (i === index ? [e[0], value] : e)))
  }

  function handleDelete(index: number) {
    setEntries((prev) => prev.filter((_, i) => i !== index))
  }

  function handleAdd() {
    const newKey = `new_key_${entries.length}`
    setEntries((prev) => [...prev, [newKey, ""]])
  }

  function entriesToPayload(): Record<string, string | number | boolean | null> {
    return Object.fromEntries(entries)
  }

  const hasEmpty = entries.some(
    ([, v]) => v === "" || v === null || v === undefined,
  )

  function hasChanges(): boolean {
    const original = Object.entries(data.payload)
    if (entries.length !== original.length) return true
    for (let i = 0; i < entries.length; i++) {
      if (entries[i][0] !== original[i][0]) return true
      if (String(entries[i][1]) !== String(original[i][1])) return true
    }
    return false
  }

  const canCreatePR = !readOnly && entries.length > 0 && !hasEmpty && hasChanges()

  function handleCreatePR() {
    navigate(`/global-values/${name}/create-pr`, {
      state: { payload: entriesToPayload() },
    })
  }

  return (
    <div className="space-y-4">
      <div className="rounded-lg border">
        <div className="grid grid-cols-[1fr_2fr_auto] gap-2 border-b bg-muted/50 px-4 py-2 text-sm font-medium text-muted-foreground">
          <span>Key</span>
          <span>Value</span>
          <span />
        </div>
        {entries.map(([key, val], index) => (
          <div
            key={index}
            className="grid grid-cols-[1fr_2fr_auto] items-center gap-2 border-b px-4 py-2 last:border-0"
          >
            <Input
              value={key}
              onChange={(e) => handleKeyChange(index, e.target.value)}
              className="font-mono text-sm"
              disabled={readOnly}
            />
            <Input
              value={String(val ?? "")}
              onChange={(e) => handleValueChange(index, e.target.value)}
              className="font-mono text-sm"
              disabled={readOnly}
            />
            {!readOnly && (
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7 text-muted-foreground hover:text-destructive"
                onClick={() => handleDelete(index)}
              >
                <Trash2 className="h-3 w-3" />
              </Button>
            )}
          </div>
        ))}
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

          <Button
            onClick={handleCreatePR}
            disabled={!canCreatePR}
            size="sm"
          >
            Create PR
          </Button>
        </>
      )}
    </div>
  )
}
