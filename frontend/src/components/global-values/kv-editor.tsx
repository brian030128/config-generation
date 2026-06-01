import { useState, useEffect } from "react"
import { useNavigate } from "react-router-dom"
import type { GlobalValues, GlobalValueValue } from "@/api/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Trash2 } from "lucide-react"

interface KvEditorProps {
  name: string
  data: GlobalValues
  readOnly?: boolean
}

type Entry = [string, GlobalValueValue]

export function KvEditor({ name, data, readOnly = false }: Readonly<KvEditorProps>) {
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

  // Switch a row between a scalar string and a list of strings. Converting to a list
  // seeds it with the current scalar (so nothing is silently lost); converting back to
  // a string keeps the first item.
  function handleTypeChange(index: number, type: "string" | "list") {
    setEntries((prev) =>
      prev.map((e, i) => {
        if (i !== index) return e
        const [k, v] = e
        if (type === "list") {
          return Array.isArray(v) ? e : [k, [String(v ?? "")]]
        }
        return Array.isArray(v) ? [k, v[0] ?? ""] : e
      }),
    )
  }

  function handleItemChange(index: number, itemIndex: number, value: string) {
    setEntries((prev) =>
      prev.map((e, i) =>
        i === index && Array.isArray(e[1])
          ? [e[0], e[1].map((it, j) => (j === itemIndex ? value : it))]
          : e,
      ),
    )
  }

  function handleAddItem(index: number) {
    setEntries((prev) =>
      prev.map((e, i) =>
        i === index && Array.isArray(e[1]) ? [e[0], [...e[1], ""]] : e,
      ),
    )
  }

  function handleRemoveItem(index: number, itemIndex: number) {
    setEntries((prev) =>
      prev.map((e, i) =>
        i === index && Array.isArray(e[1])
          ? [e[0], e[1].filter((_, j) => j !== itemIndex)]
          : e,
      ),
    )
  }

  function handleDelete(index: number) {
    setEntries((prev) => prev.filter((_, i) => i !== index))
  }

  function handleAdd() {
    const newKey = `new_key_${entries.length}`
    setEntries((prev) => [...prev, [newKey, ""]])
  }

  function entriesToPayload(): Record<string, GlobalValueValue> {
    return Object.fromEntries(entries)
  }

  // A row is "empty" (and blocks PR creation) when a scalar is blank/null, or when a
  // list is empty or contains a blank item.
  const hasEmpty = entries.some(([, v]) => {
    if (Array.isArray(v)) {
      return v.length === 0 || v.some((item) => item.trim() === "")
    }
    return v === "" || v === null || v === undefined
  })

  function hasChanges(): boolean {
    const original = Object.entries(data.payload)
    if (entries.length !== original.length) return true
    for (let i = 0; i < entries.length; i++) {
      if (entries[i][0] !== original[i][0]) return true
      if (JSON.stringify(entries[i][1]) !== JSON.stringify(original[i][1]))
        return true
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
        <div className="grid grid-cols-[1fr_auto_2fr_auto] gap-2 border-b bg-muted/50 px-4 py-2 text-sm font-medium text-muted-foreground">
          <span>Key</span>
          <span>Type</span>
          <span>Value</span>
          <span />
        </div>
        {entries.map(([key, val], index) => (
          // Row identity is positional: editing a key must not remount the
          // input (would steal focus mid-typing), so the index is the correct
          // key here. NOSONAR
          <div
            key={index} // NOSONAR
            className="grid grid-cols-[1fr_auto_2fr_auto] items-start gap-2 border-b px-4 py-2 last:border-0"
          >
            <Input
              value={key}
              onChange={(e) => handleKeyChange(index, e.target.value)}
              className="font-mono text-sm"
              disabled={readOnly}
            />
            <Select
              value={Array.isArray(val) ? "list" : "string"}
              onValueChange={(v) =>
                handleTypeChange(index, v as "string" | "list")
              }
              disabled={readOnly}
            >
              <SelectTrigger size="sm" className="w-[88px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="string">String</SelectItem>
                <SelectItem value="list">List</SelectItem>
              </SelectContent>
            </Select>
            {Array.isArray(val) ? (
              <div className="space-y-1">
                {val.map((item, itemIndex) => (
                  // Same focus-stability reason as the outer row key. NOSONAR
                  <div key={itemIndex} className="flex items-center gap-2"> {/* NOSONAR */}
                    <Input
                      value={item}
                      onChange={(e) =>
                        handleItemChange(index, itemIndex, e.target.value)
                      }
                      className="font-mono text-sm"
                      disabled={readOnly}
                    />
                    {!readOnly && (
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 shrink-0 text-muted-foreground hover:text-destructive"
                        onClick={() => handleRemoveItem(index, itemIndex)}
                      >
                        <Trash2 className="h-3 w-3" />
                      </Button>
                    )}
                  </div>
                ))}
                {val.length === 0 && (
                  <p className="text-xs text-muted-foreground">Empty list.</p>
                )}
                {!readOnly && (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="text-xs"
                    onClick={() => handleAddItem(index)}
                  >
                    + Add item
                  </Button>
                )}
              </div>
            ) : (
              <Input
                value={String(val ?? "")}
                onChange={(e) => handleValueChange(index, e.target.value)}
                className="font-mono text-sm"
                disabled={readOnly}
              />
            )}
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
