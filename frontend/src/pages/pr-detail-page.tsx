import { useParams, useNavigate } from "react-router-dom"
import { toast } from "sonner"
import {
  usePullRequest,
  useClosePullRequest,
  useMergePullRequest,
  useApprovePullRequest,
  useWithdrawApproval,
} from "@/hooks/use-pull-requests"
import { useGlobalValue } from "@/hooks/use-global-values"
import { useTemplate } from "@/hooks/use-templates"
import { useProjects } from "@/hooks/use-projects"
import { useAuth } from "@/lib/auth"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { formatRelativeTime, safeString } from "@/lib/utils"
import {
  diffLineBgClass,
  diffLinePrefix,
  diffLineTextClass,
  kvProposedTextClass,
  kvRowBgClass,
} from "@/lib/diff-styles"
import { AlertTriangle, ArrowLeft, Check } from "lucide-react"
import type { PullRequest, PRChange } from "@/api/types"

function statusVariant(status: PullRequest["status"]) {
  switch (status) {
    case "draft":
      return "secondary" as const
    case "open":
      return "default" as const
    case "approved":
      return "default" as const
    case "merged":
      return "outline" as const
    case "closed":
      return "destructive" as const
  }
}

function TemplateDiffCard({ change, projectName }: Readonly<{ change: PRChange; projectName: string }>) {
  const { data: currentTemplate } = useTemplate(projectName, change.template_name ?? "")
  const currentBody = currentTemplate?.body ?? ""
  const proposedBody = change.proposed_payload

  const oldLines = currentBody.split("\n")
  const newLines = proposedBody.split("\n")
  const maxLen = Math.max(oldLines.length, newLines.length)

  type DiffLine = { type: "context" | "added" | "removed" | "changed"; num: number; old?: string; new?: string }
  const lines: DiffLine[] = []
  for (let i = 0; i < maxLen; i++) {
    const ol = i < oldLines.length ? oldLines[i] : undefined
    const nl = i < newLines.length ? newLines[i] : undefined
    if (ol === nl) lines.push({ type: "context", num: i + 1, old: ol, new: nl })
    else if (ol === undefined) lines.push({ type: "added", num: i + 1, new: nl })
    else if (nl === undefined) lines.push({ type: "removed", num: i + 1, old: ol })
    else lines.push({ type: "changed", num: i + 1, old: ol, new: nl })
  }

  return (
    <div className="rounded-lg border">
      <div className="flex items-center justify-between border-b bg-muted/50 px-4 py-2">
        <span className="text-sm font-medium">Template: {change.template_name}</span>
        <span className="text-xs text-muted-foreground">
          v{change.base_version_id} → proposed
        </span>
      </div>
      <div className="max-h-[500px] overflow-auto font-mono text-sm">
        {lines.map((line, i) => {
          if (line.type === "changed") {
            return (
              <div key={`${i}:c:${line.num}`}>
                <div className="flex bg-red-50 dark:bg-red-950/20">
                  <span className="w-10 shrink-0 select-none px-2 py-0.5 text-right text-xs text-muted-foreground">{line.num}</span>
                  <span className="px-2 py-0.5 text-red-700 dark:text-red-400 whitespace-pre">- {line.old}</span>
                </div>
                <div className="flex bg-green-50 dark:bg-green-950/20">
                  <span className="w-10 shrink-0 select-none px-2 py-0.5 text-right text-xs text-muted-foreground">{line.num}</span>
                  <span className="px-2 py-0.5 text-green-700 dark:text-green-400 whitespace-pre">+ {line.new}</span>
                </div>
              </div>
            )
          }
          const bg = diffLineBgClass(line.type)
          const prefix = diffLinePrefix(line.type)
          const textColor = diffLineTextClass(line.type)
          const content = line.type === "removed" ? line.old : (line.new ?? line.old)

          return (
            <div key={`${i}:${line.type}:${line.num}`} className={`flex ${bg}`}>
              <span className="w-10 shrink-0 select-none px-2 py-0.5 text-right text-xs text-muted-foreground">{line.num}</span>
              <span className={`px-2 py-0.5 whitespace-pre ${textColor}`}>{prefix}{content}</span>
            </div>
          )
        })}
      </div>
    </div>
  )
}

// Render a diff value: lists as JSON (e.g. ["a","b"]), scalars as plain text.
function formatVal(v: unknown): string {
  if (v === undefined) return "—"
  return safeString(v)
}

function KvDiffCard({ change }: Readonly<{ change: PRChange }>) {
  const globalValuesName = change.object_type === "global_values" ? change.global_values_name : null
  const { data: currentGV } = useGlobalValue(globalValuesName ?? "")

  const proposed: Record<string, unknown> = (() => {
    try {
      return JSON.parse(change.proposed_payload)
    } catch {
      return {}
    }
  })()

  const current: Record<string, unknown> = currentGV?.payload ?? {}

  const allKeys = Array.from(
    new Set([...Object.keys(current), ...Object.keys(proposed)]),
  )

  const label =
    change.object_type === "global_values"
      ? `Global Values: ${change.global_values_name}`
      : `Values: ${change.environment_name ?? "unknown"}`

  return (
    <div className="rounded-lg border">
      <div className="flex items-center justify-between border-b bg-muted/50 px-4 py-2">
        <span className="text-sm font-medium">{label}</span>
        <span className="text-xs text-muted-foreground">
          v{change.base_version_id} → proposed
        </span>
      </div>
      <div className="grid grid-cols-[1fr_1fr_1fr] gap-2 border-b bg-muted/30 px-4 py-2 text-sm font-medium text-muted-foreground">
        <span>Key</span>
        <span>Current</span>
        <span>Proposed</span>
      </div>
      {allKeys.map((key) => {
        const currentVal = current[key]
        const proposedVal = proposed[key]
        const isAdded = currentVal === undefined
        const isRemoved = proposedVal === undefined
        const isChanged =
          !isAdded && !isRemoved && formatVal(currentVal) !== formatVal(proposedVal)
        const unchanged = !isAdded && !isRemoved && !isChanged

        return (
          <div
            key={key}
            className={`grid grid-cols-[1fr_1fr_1fr] items-center gap-2 border-b px-4 py-2 last:border-0 text-sm font-mono ${kvRowBgClass(
              { isAdded, isRemoved, isChanged },
            )}`}
          >
            <span className="font-medium">{key}</span>
            <span
              className={`${unchanged ? "text-muted-foreground" : ""} ${isRemoved ? "line-through text-red-600" : ""}`}
            >
              {formatVal(currentVal)}
            </span>
            <span
              className={`${unchanged ? "text-muted-foreground" : ""} ${kvProposedTextClass({ isAdded, isChanged })}`}
            >
              {formatVal(proposedVal)}
            </span>
          </div>
        )
      })}
    </div>
  )
}

function EnvCreateCard({ change }: Readonly<{ change: PRChange }>) {
  const envData = (() => {
    try { return JSON.parse(change.proposed_payload) as { name: string; description?: string } }
    catch { return { name: "unknown" } }
  })()

  return (
    <div className="rounded-lg border">
      <div className="flex items-center justify-between border-b bg-muted/50 px-4 py-2">
        <span className="text-sm font-medium">New Environment: {envData.name}</span>
      </div>
      {envData.description && (
        <div className="px-4 py-2 text-sm text-muted-foreground">{envData.description}</div>
      )}
    </div>
  )
}

function ChangeCard({ change, projectName }: Readonly<{ change: PRChange; projectName: string }>) {
  if (change.object_type === "template") {
    return <TemplateDiffCard change={change} projectName={projectName} />
  }
  if (change.object_type === "environment") {
    return <EnvCreateCard change={change} />
  }
  return <KvDiffCard change={change} />
}

// PRActionBar renders the bottom action buttons. Extracted so PRDetailPage
// stays within Sonar's cognitive-complexity limit.
function PRActionBar({
  canClose,
  canMerge,
  isOpenForApproval,
  hasApproved,
  approvePR,
  withdrawApproval,
  mergePR,
  closePR,
  runPRAction,
}: {
  canClose: boolean
  canMerge: boolean
  isOpenForApproval: boolean
  hasApproved: boolean
  approvePR: { isPending: boolean }
  withdrawApproval: { isPending: boolean }
  mergePR: { isPending: boolean }
  closePR: { isPending: boolean }
  runPRAction: (
    mutation: { mutate: (id: number, opts: { onSuccess: () => void; onError: (err: unknown) => void }) => void },
    successMsg: string,
    errorMsg: string,
  ) => void
}) {
  if (!(canClose || canMerge || isOpenForApproval)) return null
  return (
    <div className="flex gap-3 border-t pt-4">
      {isOpenForApproval && !hasApproved && (
        <Button
          variant="outline"
          onClick={() => runPRAction(approvePR as never, "Pull request approved", "Failed to approve pull request")}
          disabled={approvePR.isPending}
        >
          {approvePR.isPending ? "Approving..." : "Approve"}
        </Button>
      )}
      {isOpenForApproval && hasApproved && (
        <Button
          variant="ghost"
          onClick={() => runPRAction(withdrawApproval as never, "Approval withdrawn", "Failed to withdraw approval")}
          disabled={withdrawApproval.isPending}
        >
          {withdrawApproval.isPending ? "Withdrawing..." : "Withdraw Approval"}
        </Button>
      )}
      {canMerge && (
        <Button
          onClick={() => runPRAction(mergePR as never, "Pull request merged", "Failed to merge pull request")}
          disabled={mergePR.isPending}
        >
          {mergePR.isPending ? "Merging..." : "Merge"}
        </Button>
      )}
      {canClose && (
        <Button
          variant="destructive"
          onClick={() => runPRAction(closePR as never, "Pull request closed", "Failed to close pull request")}
          disabled={closePR.isPending}
        >
          {closePR.isPending ? "Closing..." : "Close PR"}
        </Button>
      )}
    </div>
  )
}

export default function PRDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const prId = Number(id)
  const { user } = useAuth()
  const { data: pr, isLoading, error } = usePullRequest(prId)
  const { data: projects } = useProjects()
  const projectName =
    projects?.items.find((p) => p.id === pr?.project_id)?.name ?? ""
  const closePR = useClosePullRequest()
  const mergePR = useMergePullRequest()
  const approvePR = useApprovePullRequest()
  const withdrawApproval = useWithdrawApproval()

  const canClose =
    pr && ["draft", "open", "approved"].includes(pr.status)
  const canMerge = pr?.status === "approved" && !pr.is_conflicted
  const isOpenForApproval =
    pr?.status === "open" || pr?.status === "approved"
  const hasApproved =
    user && pr?.approvals?.some((a) => a.user_id === user.id && !a.withdrawn_at)

  function runPRAction(
    mutation: { mutate: (id: number, opts: { onSuccess: () => void; onError: (err: unknown) => void }) => void },
    successMsg: string,
    errorMsg: string,
  ) {
    if (!pr) return
    mutation.mutate(pr.id, {
      onSuccess: () => toast.success(successMsg),
      onError: (err) => toast.error(errorMsg, { description: (err as Error).message }),
    })
  }

  if (isLoading) {
    return (
      <div className="space-y-6">
        <p className="text-muted-foreground">Loading pull request...</p>
      </div>
    )
  }

  if (error || !pr) {
    return (
      <div className="space-y-6">
        <p className="text-destructive">
          {error
            ? `Failed to load pull request: ${(error as Error).message}`
            : "Pull request not found"}
        </p>
      </div>
    )
  }

  const activeApprovals = pr.approvals?.filter((a) => !a.withdrawn_at) ?? []

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      {/* Back link */}
      <button
        onClick={() => navigate("/pull-requests")}
        className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="h-4 w-4" />
        Pull Requests
      </button>

      {/* Header */}
      <div className="space-y-2">
        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-semibold">
            <span className="text-muted-foreground">#{pr.id}</span>{" "}
            {pr.title}
          </h1>
          <Badge variant={statusVariant(pr.status)}>{pr.status}</Badge>
          {pr.is_conflicted && (
            <AlertTriangle className="h-5 w-5 text-amber-500" />
          )}
        </div>
        <p className="text-sm text-muted-foreground">
          opened {formatRelativeTime(pr.created_at)} · updated{" "}
          {formatRelativeTime(pr.updated_at)}
        </p>
      </div>

      {/* Conflict warning */}
      {pr.is_conflicted && (
        <div className="rounded-lg border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-700 dark:bg-amber-950/30 dark:text-amber-200">
          This PR has conflicts with the latest version. Close this PR and
          create a new one incorporating the latest changes.
        </div>
      )}

      {/* Description */}
      {pr.description && (
        <div className="space-y-1">
          <h2 className="text-sm font-medium text-muted-foreground">
            Description
          </h2>
          <p className="text-sm">{pr.description}</p>
        </div>
      )}

      {/* Changes */}
      <div className="space-y-3">
        <h2 className="text-sm font-medium">
          Changes ({pr.changes?.length ?? 0})
        </h2>
        {pr.changes && pr.changes.length > 0 ? (
          pr.changes.map((change) => (
            <ChangeCard key={change.id} change={change} projectName={projectName} />
          ))
        ) : (
          <p className="text-sm text-muted-foreground">No changes.</p>
        )}
      </div>

      {/* Approvals */}
      <div className="space-y-3">
        <h2 className="text-sm font-medium">Approvals</h2>
        {pr.approval_condition && (
          <p className="text-xs text-muted-foreground">
            Condition: <span className="font-mono">{pr.approval_condition}</span>
          </p>
        )}
        {activeApprovals.length > 0 ? (
          <div className="space-y-1">
            {activeApprovals.map((approval) => (
              <div
                key={approval.id}
                className="flex items-center gap-2 text-sm"
              >
                <Check className="h-4 w-4 text-green-600" />
                <span>User #{approval.user_id}</span>
                <span className="text-muted-foreground">
                  approved {formatRelativeTime(approval.approved_at)}
                </span>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">No approvals yet.</p>
        )}
      </div>

      {/* Actions */}
      <PRActionBar
        canClose={!!canClose}
        canMerge={!!canMerge}
        isOpenForApproval={!!isOpenForApproval}
        hasApproved={!!hasApproved}
        approvePR={approvePR}
        withdrawApproval={withdrawApproval}
        mergePR={mergePR}
        closePR={closePR}
        runPRAction={runPRAction}
      />
    </div>
  )
}
