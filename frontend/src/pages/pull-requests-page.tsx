import { useState } from "react"
import { useNavigate } from "react-router-dom"
import { usePullRequests } from "@/hooks/use-pull-requests"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Badge } from "@/components/ui/badge"
import { formatRelativeTime } from "@/lib/utils"
import { AlertTriangle } from "lucide-react"
import type { PullRequest } from "@/api/types"

const OPEN_STATUSES = ["draft", "open", "approved"]
const CLOSED_STATUSES = ["merged", "closed"]

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

export default function PullRequestsPage() {
  const navigate = useNavigate()
  const [tab, setTab] = useState("open")
  const { data, isLoading, error } = usePullRequests()

  const allItems = data?.items ?? []
  const filtered = allItems.filter((pr) =>
    tab === "open"
      ? OPEN_STATUSES.includes(pr.status)
      : CLOSED_STATUSES.includes(pr.status),
  )

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Pull Requests</h1>

      <Tabs value={tab} onValueChange={setTab}>
        <TabsList>
          <TabsTrigger value="open">Open</TabsTrigger>
          <TabsTrigger value="closed">Closed</TabsTrigger>
        </TabsList>

        <TabsContent value={tab} className="mt-4">
          {isLoading && (
            <p className="text-muted-foreground">Loading pull requests...</p>
          )}

          {error && (
            <p className="text-destructive">
              Failed to load pull requests: {(error as Error).message}
            </p>
          )}

          {!isLoading && !error && filtered.length === 0 && (
            <p className="text-muted-foreground">
              No {tab} pull requests.
            </p>
          )}

          <div className="space-y-2">
            {filtered.map((pr) => (
              <div
                key={pr.id}
                onClick={() => navigate(`/pull-requests/${pr.id}`)}
                className="flex cursor-pointer items-center justify-between rounded-lg border px-4 py-3 transition-colors hover:bg-accent/50"
              >
                <div className="flex items-center gap-3">
                  <span className="text-sm font-medium text-muted-foreground">
                    #{pr.id}
                  </span>
                  <span className="text-sm font-medium">{pr.title}</span>
                  <Badge variant={statusVariant(pr.status)}>{pr.status}</Badge>
                  {pr.is_conflicted && (
                    <AlertTriangle className="h-4 w-4 text-amber-500" />
                  )}
                </div>
                <span className="text-xs text-muted-foreground">
                  {formatRelativeTime(pr.updated_at)}
                </span>
              </div>
            ))}
          </div>
        </TabsContent>
      </Tabs>
    </div>
  )
}
