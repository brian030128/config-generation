import { useState } from "react"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  ReferenceSelector,
  parseReference,
  buildReference,
} from "./reference-selector"
import { Trash2 } from "lucide-react"

interface ValueRowProps {
  keyName: string
  value: unknown
  depth?: number
  onChange: (newValue: unknown) => void
  onDelete: () => void
}

export function ValueRow({
  keyName,
  value,
  depth = 0,
  onChange,
  onDelete,
}: ValueRowProps) {
  const isObject = typeof value === "object" && value !== null && !Array.isArray(value)

  if (isObject) {
    return (
      <NestedRow
        keyName={keyName}
        value={value as Record<string, unknown>}
        depth={depth}
        onChange={onChange}
        onDelete={onDelete}
      />
    )
  }

  const strValue = String(value ?? "")
  const ref = parseReference(strValue)
  const isRef = ref !== null

  return (
    <div
      className="flex items-center gap-2 py-1.5"
      style={{ paddingLeft: depth * 24 }}
    >
      <span className="w-40 shrink-0 truncate font-mono text-sm">
        {keyName}
      </span>

      {isRef ? (
        <ReferenceSelector
          group={ref.group}
          keyName={ref.key}
          onGroupChange={(g) => onChange(buildReference(g, ref.key))}
          onKeyChange={(k) => onChange(buildReference(ref.group, k))}
        />
      ) : (
        <Input
          className="flex-1 font-mono text-sm"
          value={strValue}
          onChange={(e) => onChange(e.target.value)}
        />
      )}

      <Button
        variant="ghost"
        size="sm"
        onClick={() => {
          if (isRef) {
            // Switch to text mode
            onChange("")
          } else {
            // Switch to ref mode
            onChange("${.}")
          }
        }}
      >
        <Badge variant="outline" className="text-xs">
          {isRef ? "Ref" : "Text"}
        </Badge>
      </Button>

      <Button
        variant="ghost"
        size="icon"
        className="h-7 w-7 text-muted-foreground hover:text-destructive"
        onClick={onDelete}
      >
        <Trash2 className="h-3 w-3" />
      </Button>
    </div>
  )
}

function NestedRow({
  keyName,
  value,
  depth,
  onChange,
  onDelete,
}: {
  keyName: string
  value: Record<string, unknown>
  depth: number
  onChange: (newValue: unknown) => void
  onDelete: () => void
}) {
  const [expanded, setExpanded] = useState(true)
  const entries = Object.entries(value)

  function handleChildChange(childKey: string, newChildValue: unknown) {
    onChange({ ...value, [childKey]: newChildValue })
  }

  function handleChildDelete(childKey: string) {
    const copy = { ...value }
    delete copy[childKey]
    onChange(copy)
  }

  function handleAddKey() {
    const newKey = `new_key_${entries.length}`
    onChange({ ...value, [newKey]: "" })
  }

  return (
    <div style={{ paddingLeft: depth * 24 }}>
      <div className="flex items-center gap-2 py-1.5">
        <button
          onClick={() => setExpanded(!expanded)}
          className="text-xs text-muted-foreground"
        >
          {expanded ? "▾" : "▸"}
        </button>
        <span className="font-mono text-sm font-medium">{keyName}</span>
        <Badge variant="secondary" className="text-xs">
          object
        </Badge>
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7 text-muted-foreground hover:text-destructive"
          onClick={onDelete}
        >
          <Trash2 className="h-3 w-3" />
        </Button>
      </div>
      {expanded && (
        <div className="border-l ml-2 pl-2">
          {entries.map(([k, v]) => (
            <ValueRow
              key={k}
              keyName={k}
              value={v}
              depth={depth + 1}
              onChange={(nv) => handleChildChange(k, nv)}
              onDelete={() => handleChildDelete(k)}
            />
          ))}
          <Button
            variant="ghost"
            size="sm"
            className="mt-1 text-xs"
            style={{ marginLeft: (depth + 1) * 24 }}
            onClick={handleAddKey}
          >
            + Add Key
          </Button>
        </div>
      )}
    </div>
  )
}

