import { useMemo, useState } from "react"
import { useParams } from "react-router-dom"
import { useGlobalValue } from "@/hooks/use-global-values"
import { globalValuesApi } from "@/api/global-values"
import { KvEditor } from "@/components/global-values/kv-editor"
import { GvVersionHistory } from "@/components/global-values/version-history"
import { ApprovalPolicyCard } from "@/components/roles/approval-policy-card"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { formatRelativeTime } from "@/lib/utils"
import type {
  GlobalValues,
  GlobalValuesGroupVersion,
  GlobalValueValue,
} from "@/api/types"

// Synthesize a single-entry GlobalValues view from a group version. The legacy
// UI assumed one editable payload per "name"; in the new model every value in
// the group has its own payload, so we pick the entry that matches the group
// name (the migration default and the createGlobalValues convention) and fall
// back to the first entry.
function selectEditableEntry(
  groupName: string,
  ver: GlobalValuesGroupVersion | undefined,
): { entry: GlobalValues; entryName: string } | null {
  if (!ver?.values) return null
  const names = Object.keys(ver.values)
  if (names.length === 0) return null
  const chosen = names.includes(groupName) ? groupName : names[0]
  const payload = ver.values[chosen] as Record<string, GlobalValueValue>
  return {
    entry: {
      id: ver.id,
      group_id: ver.group_id,
      name: chosen,
      payload,
      commit_message: ver.commit_message,
      created_by: ver.created_by,
      created_at: ver.created_at,
    },
    entryName: chosen,
  }
}

export default function GlobalValuesDetailPage() {
  const { name = "" } = useParams<{ name: string }>()
  const { data, isLoading, error } = useGlobalValue(name)
  const [viewingVersion, setViewingVersion] =
    useState<GlobalValuesGroupVersion | null>(null)

  const latestVersionId = data?.latest_version.version_id ?? null

  async function handleSelectVersion(versionId: number) {
    if (versionId === latestVersionId) {
      setViewingVersion(null)
      return
    }
    try {
      const ver = await globalValuesApi.getVersion(name, versionId)
      setViewingVersion(ver)
    } catch {
      // ignore
    }
  }

  const displayVersion: GlobalValuesGroupVersion | undefined =
    viewingVersion ?? data?.latest_version
  const editable = useMemo(
    () => selectEditableEntry(name, displayVersion),
    [name, displayVersion],
  )

  if (isLoading) {
    return <p className="text-muted-foreground">Loading...</p>
  }

  if (error || !data || !displayVersion) {
    return (
      <p className="text-destructive">
        Failed to load global values: {(error as Error)?.message ?? "Not found"}
      </p>
    )
  }

  const isReadOnly = viewingVersion !== null

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">{name}</h1>
        <div className="mt-1 flex items-center gap-2 text-sm text-muted-foreground">
          <Badge variant="outline">v{displayVersion.version_id}</Badge>
          <span>{formatRelativeTime(displayVersion.created_at)}</span>
          {isReadOnly && (
            <span className="text-amber-600">
              (viewing historical version —{" "}
              <button
                className="underline"
                onClick={() => setViewingVersion(null)}
              >
                return to latest
              </button>
              {")"}
            </span>
          )}
        </div>
      </div>

      <div className="flex gap-6">
        <div className="flex-1">
          {editable ? (
            <KvEditor
              name={name}
              data={editable.entry}
              readOnly={isReadOnly}
            />
          ) : (
            <p className="text-sm text-muted-foreground">
              This group has no values yet.
            </p>
          )}
        </div>

        <Separator orientation="vertical" className="h-auto" />

        <div className="w-64 shrink-0">
          <GvVersionHistory
            name={name}
            selectedVersion={displayVersion.version_id}
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

      {data.viewer_can_manage && (
        <div className="max-w-2xl space-y-3">
          <Separator />
          <ApprovalPolicyCard
            kind="global-values"
            name={name}
            currentCondition={data.group.approval_condition}
          />
          <p className="text-sm text-muted-foreground">
            Roles are managed globally on the Roles page. Create approver roles
            there, then reference them by name above.
          </p>
        </div>
      )}
    </div>
  )
}
