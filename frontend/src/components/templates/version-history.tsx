import { useTemplateVersions } from "@/hooks/use-templates"
import { formatRelativeTime, cn } from "@/lib/utils"

interface VersionHistoryProps {
  projectName: string
  templateName: string
  selectedVersion: number | null
  onSelectVersion: (versionId: number) => void
}

export function VersionHistory({
  projectName,
  templateName,
  selectedVersion,
  onSelectVersion,
}: VersionHistoryProps) {
  const { data, isLoading } = useTemplateVersions(projectName, templateName)

  if (isLoading) {
    return <p className="text-sm text-muted-foreground">Loading versions...</p>
  }

  const versions = data?.items ?? []
  if (versions.length === 0) return null

  const sorted = [...versions].sort((a, b) => b.version_id - a.version_id)
  const latestVersion = sorted[0].version_id

  return (
    <div className="space-y-1">
      <h4 className="text-sm font-medium">Version History</h4>
      <div className="max-h-64 space-y-1 overflow-auto">
        {sorted.map((v) => (
          <button
            key={v.version_id}
            onClick={() => onSelectVersion(v.version_id)}
            className={cn(
              "flex w-full items-center justify-between rounded px-2 py-1.5 text-left text-sm transition-colors",
              selectedVersion === v.version_id
                ? "bg-accent text-accent-foreground"
                : "hover:bg-accent/50",
            )}
          >
            <span className="font-mono">
              v{v.version_id}
              {v.version_id === latestVersion && (
                <span className="ml-1.5 text-xs text-muted-foreground">
                  (latest)
                </span>
              )}
            </span>
            <span className="text-xs text-muted-foreground">
              {formatRelativeTime(v.created_at)}
            </span>
          </button>
        ))}
      </div>
    </div>
  )
}
