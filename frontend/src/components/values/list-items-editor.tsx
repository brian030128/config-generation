import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Trash2 } from "lucide-react"

interface ListItemsEditorProps {
  items: string[]
  onChange: (items: string[]) => void
  readOnly?: boolean
}

// replaceAt / removeAt are module-scope list helpers; lifting them out keeps the
// onChange callbacks flat (Sonar S2004).
function replaceAt(items: string[], index: number, value: string): string[] {
  return items.map((it, j) => (j === index ? value : it))
}

function removeAt(items: string[], index: number): string[] {
  return items.filter((_, j) => j !== index)
}

// ListItemsEditor renders an editable list of string items with add/remove
// controls. Shared by the global-values editor and the env-values page so both
// surfaces edit lists identically.
export function ListItemsEditor({
  items,
  onChange,
  readOnly = false,
}: Readonly<ListItemsEditorProps>) {
  return (
    <div className="space-y-1">
      {items.map((item, itemIndex) => (
        // Row identity is positional so editing an item doesn't steal focus. NOSONAR
        <div key={itemIndex} className="flex items-center gap-2"> {/* NOSONAR */}
          <Input
            value={item}
            onChange={(e) => onChange(replaceAt(items, itemIndex, e.target.value))}
            className="font-mono text-sm"
            disabled={readOnly}
          />
          {!readOnly && (
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 shrink-0 text-muted-foreground hover:text-destructive"
              onClick={() => onChange(removeAt(items, itemIndex))}
            >
              <Trash2 className="h-3 w-3" />
            </Button>
          )}
        </div>
      ))}
      {items.length === 0 && (
        <p className="text-xs text-muted-foreground">Empty list.</p>
      )}
      {!readOnly && (
        <Button
          variant="ghost"
          size="sm"
          className="text-xs"
          onClick={() => onChange([...items, ""])}
        >
          + Add item
        </Button>
      )}
    </div>
  )
}
