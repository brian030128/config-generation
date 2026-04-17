import { useState } from "react"
import { useParams } from "react-router-dom"
import {
  useGlobalValue,
} from "@/hooks/use-global-values"
import { globalValuesApi } from "@/api/global-values"
import { KvEditor } from "@/components/global-values/kv-editor"
import { GvVersionHistory } from "@/components/global-values/version-history"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { formatRelativeTime } from "@/lib/utils"
import type { GlobalValues } from "@/api/types"

export default function GlobalValuesDetailPage() {
  const { name } = useParams<{ name: string }>()
  const { data: latest, isLoading, error } = useGlobalValue(name!)
  const [viewingVersion, setViewingVersion] = useState<GlobalValues | null>(
    null,
  )

  const latestVersionId = latest?.version_id ?? null

  async function handleSelectVersion(versionId: number) {
    if (versionId === latestVersionId) {
      setViewingVersion(null)
      return
    }
    try {
      const ver = await globalValuesApi.getVersion(name!, versionId)
      setViewingVersion(ver)
    } catch {
      // ignore
    }
  }

  if (isLoading) {
    return <p className="text-muted-foreground">Loading...</p>
  }

  if (error || !latest) {
    return (
      <p className="text-destructive">
        Failed to load global values: {(error as Error)?.message ?? "Not found"}
      </p>
    )
  }

  const displayData = viewingVersion ?? latest
  const isReadOnly = viewingVersion !== null

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">{name}</h1>
        <div className="mt-1 flex items-center gap-2 text-sm text-muted-foreground">
          <Badge variant="outline">v{displayData.version_id}</Badge>
          <span>{formatRelativeTime(displayData.created_at)}</span>
          {isReadOnly && (
            <span className="text-amber-600">
              (viewing historical version —{" "}
              <button
                className="underline"
                onClick={() => setViewingVersion(null)}
              >
                return to latest
              </button>
              )
            </span>
          )}
        </div>
      </div>

      <div className="flex gap-6">
        <div className="flex-1">
          <KvEditor name={name!} data={displayData} readOnly={isReadOnly} />
        </div>

        <Separator orientation="vertical" className="h-auto" />

        <div className="w-64 shrink-0">
          <GvVersionHistory
            name={name!}
            selectedVersion={displayData.version_id}
            onSelectVersion={handleSelectVersion}
          />

          <div className="mt-6">
            <h4 className="text-sm font-medium">Referenced By</h4>
            <p className="mt-1 text-xs text-muted-foreground">
              Not yet available
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}
