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
import { formatRelativeTime } from "@/lib/utils"
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

function TemplateDiffCard({ change, projectName }: { change: PRChange; projectName: string }) {
  const { data: currentTemplate } = useTemplate(projectName, change.template_name ?? "")
  const currentBody = currentTemplate?.body ?? ""
  const proposedBody = change.proposed_payload

  const oldLines = currentBody.split("\n")
  const newLines = proposedBody.split("\n")
  const maxLen = Math.max(oldLines.length, newLines.length)

  type DiffLine = { type: "unchanged" | "added" | "removed" | "changed"; num: number; old?: string; new?: string }
  const lines: DiffLine[] = []
  for (let i = 0; i < maxLen; i++) {
    const ol = i < oldLines.length ? oldLines[i] : undefined
    const nl = i < newLines.length ? newLines[i] : undefined
    if (ol === nl) lines.push({ type: "unchanged", num: i + 1, old: ol, new: nl })
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
              <div key={i}>
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
          const bg =
            line.type === "added" ? "bg-green-50 dark:bg-green-950/20"
            : line.type === "removed" ? "bg-red-50 dark:bg-red-950/20"
            : ""
          const prefix = line.type === "added" ? "+ " : line.type === "removed" ? "- " : "  "
          const textColor =
            line.type === "added" ? "text-green-700 dark:text-green-400"
            : line.type === "removed" ? "text-red-700 dark:text-red-400"
            : ""
          const content = line.type === "removed" ? line.old : (line.new ?? line.old)

          return (
            <div key={i} className={`flex ${bg}`}>
              <span className="w-10 shrink-0 select-none px-2 py-0.5 text-right text-xs text-muted-foreground">{line.num}</span>
              <span className={`px-2 py-0.5 whitespace-pre ${textColor}`}>{prefix}{content}</span>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function KvDiffCard({ change }: { change: PRChange }) {
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
      : `Values: ${change.template_name}`

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
          !isAdded && !isRemoved && String(currentVal) !== String(proposedVal)
        const unchanged = !isAdded && !isRemoved && !isChanged

        return (
          <div
            key={key}
            className={`grid grid-cols-[1fr_1fr_1fr] items-center gap-2 border-b px-4 py-2 last:border-0 text-sm font-mono ${
              isAdded
                ? "bg-green-50 dark:bg-green-950/20"
                : isRemoved
                  ? "bg-red-50 dark:bg-red-950/20"
                  : isChanged
                    ? "bg-yellow-50 dark:bg-yellow-950/20"
                    : ""
            }`}
          >
            <span className="font-medium">{key}</span>
            <span
              className={`${unchanged ? "text-muted-foreground" : ""} ${isRemoved ? "line-through text-red-600" : ""}`}
            >
              {currentVal !== undefined ? String(currentVal) : "—"}
            </span>
            <span
              className={`${unchanged ? "text-muted-foreground" : ""} ${isAdded ? "text-green-600" : isChanged ? "text-yellow-600" : ""}`}
            >
              {proposedVal !== undefined ? String(proposedVal) : "—"}
            </span>
          </div>
        )
      })}
    </div>
  )
}

function ChangeCard({ change, projectName }: { change: PRChange; projectName: string }) {
  if (change.object_type === "template") {
    return <TemplateDiffCard change={change} projectName={projectName} />
  }
  return <KvDiffCard change={change} />
}

export default function PRDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const prId = Number(id)
  const { user } = useAuth()
  const { data: pr, isLoading, error } = usePullRequest(prId)
  const { data: projects } = useProjects()
  const closePR = useClosePullRequest()
  const mergePR = useMergePullRequest()
  const approvePR = useApprovePullRequest()
  const withdrawApproval = useWithdrawApproval()

  const canClose =
    pr && ["draft", "open", "approved"].includes(pr.status)
  const canMerge =
    pr && pr.status === "approved" && !pr.is_conflicted
  const isOpenForApproval =
    pr && (pr.status === "open" || pr.status === "approved")
  const hasApproved =
    pr && user && pr.approvals?.some((a) => a.user_id === user.id && !a.withdrawn_at)

  function handleClose() {
    if (!pr) return
    closePR.mutate(pr.id, {
      onSuccess: () => {
        toast.success("Pull request closed")
      },
      onError: (err) => {
        toast.error("Failed to close pull request", {
          description: (err as Error).message,
        })
      },
    })
  }

  function handleMerge() {
    if (!pr) return
    mergePR.mutate(pr.id, {
      onSuccess: () => {
        toast.success("Pull request merged")
      },
      onError: (err) => {
        toast.error("Failed to merge pull request", {
          description: (err as Error).message,
        })
      },
    })
  }

  function handleApprove() {
    if (!pr) return
    approvePR.mutate(pr.id, {
      onSuccess: () => {
        toast.success("Pull request approved")
      },
      onError: (err) => {
        toast.error("Failed to approve pull request", {
          description: (err as Error).message,
        })
      },
    })
  }

  function handleWithdraw() {
    if (!pr) return
    withdrawApproval.mutate(pr.id, {
      onSuccess: () => {
        toast.success("Approval withdrawn")
      },
      onError: (err) => {
        toast.error("Failed to withdraw approval", {
          description: (err as Error).message,
        })
      },
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
  const projectName = pr.project_id
    ? projects?.items.find((p) => p.id === pr.project_id)?.name ?? ""
    : ""

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
      {(canClose || canMerge || isOpenForApproval) && (
        <div className="flex gap-3 border-t pt-4">
          {isOpenForApproval && !hasApproved && (
            <Button
              variant="outline"
              onClick={handleApprove}
              disabled={approvePR.isPending}
            >
              {approvePR.isPending ? "Approving..." : "Approve"}
            </Button>
          )}
          {isOpenForApproval && hasApproved && (
            <Button
              variant="ghost"
              onClick={handleWithdraw}
              disabled={withdrawApproval.isPending}
            >
              {withdrawApproval.isPending ? "Withdrawing..." : "Withdraw Approval"}
            </Button>
          )}
          {canMerge && (
            <Button
              onClick={handleMerge}
              disabled={mergePR.isPending}
            >
              {mergePR.isPending ? "Merging..." : "Merge"}
            </Button>
          )}
          {canClose && (
            <Button
              variant="destructive"
              onClick={handleClose}
              disabled={closePR.isPending}
            >
              {closePR.isPending ? "Closing..." : "Close PR"}
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
