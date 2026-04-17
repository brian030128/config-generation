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
}: ReferenceSelectorProps) {
  const { data: gvList } = useGlobalValues()
  const { data: gvDetail } = useGlobalValue(group)

  const groups = gvList?.items ?? []
  const keys = gvDetail ? Object.keys(gvDetail.payload) : []

  return (
    <div className="flex gap-2">
      <Select value={group} onValueChange={onGroupChange}>
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
      <Select value={keyName} onValueChange={onKeyChange}>
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
export function parseReference(value: string): {
  group: string
  key: string
} | null {
  const match = value.match(/^\$\{(\w+)\.(\w+)\}$/)
  if (!match) return null
  return { group: match[1], key: match[2] }
}

export function buildReference(group: string, key: string): string {
  return `\${${group}.${key}}`
}
