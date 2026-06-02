import { useGlobalValues, useGlobalValue } from "@/hooks/use-global-values"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

interface ReferenceSelectorProps {
  group: string
  keyName: string
  onGroupChange: (group: string) => void
  onKeyChange: (key: string) => void
}

export function ReferenceSelector({
  group,
  keyName,
  onGroupChange,
  onKeyChange,
}: Readonly<ReferenceSelectorProps>) {
  const { data: gvList } = useGlobalValues()
  const { data: gvDetail } = useGlobalValue(group)

  const groups = gvList?.items ?? []
  // Keys for the picker are the union of keys across every value entry in the
  // group's latest version — the flat merge that renderers do at lookup time.
  const keys = gvDetail
    ? Array.from(
        new Set(
          Object.values(gvDetail.latest_version.values ?? {}).flatMap(
            (payload) => Object.keys(payload ?? {}),
          ),
        ),
      )
    : []

  return (
    <div className="flex gap-2">
      <Select value={group || undefined} onValueChange={onGroupChange}>
        <SelectTrigger className="w-40">
          <SelectValue placeholder="Group" />
        </SelectTrigger>
        <SelectContent>
          {groups.map((g) => (
            <SelectItem key={g.name} value={g.name}>
              {g.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select value={keyName || undefined} onValueChange={onKeyChange} disabled={!group}>
        <SelectTrigger className="w-40">
          <SelectValue placeholder="Key" />
        </SelectTrigger>
        <SelectContent>
          {keys.map((k) => (
            <SelectItem key={k} value={k}>
              {k}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}

// Parse a reference string like "${group.key}" into {group, key}
const REFERENCE_RE = /^\$\{(\w+)\.(\w+)\}$/

export function parseReference(value: string): {
  group: string
  key: string
} | null {
  const match = REFERENCE_RE.exec(value)
  if (!match) return null
  return { group: match[1], key: match[2] }
}

export function buildReference(group: string, key: string): string {
  return `\${${group}.${key}}`
}
