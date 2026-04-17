import { useState } from "react"
import { useParams, useLocation, useNavigate, Navigate } from "react-router-dom"
import { toast } from "sonner"
import { useGlobalValue } from "@/hooks/use-global-values"
import { useCreatePullRequest } from "@/hooks/use-pull-requests"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"

export default function CreatePRPage() {
  const { name } = useParams<{ name: string }>()
  const location = useLocation()
  const navigate = useNavigate()
  const proposedPayload = (location.state as { payload?: Record<string, string | number | boolean | null> })?.payload
  const { data: current } = useGlobalValue(name!)
  const createPR = useCreatePullRequest()

  const [title, setTitle] = useState("")
  const [description, setDescription] = useState("")

  if (!proposedPayload) {
    return <Navigate to={`/global-values/${name}`} replace />
  }

  const currentEntries = current ? Object.entries(current.payload) : []
  const proposedEntries = Object.entries(proposedPayload)

  // Build a unified set of keys for diffing
  const allKeys = Array.from(
    new Set([
      ...currentEntries.map(([k]) => k),
      ...proposedEntries.map(([k]) => k),
    ]),
  )

  function handleSubmit() {
    createPR.mutate(
      {
        title,
        description: description.trim() || undefined,
        object_type: "global_values",
        global_values_name: name,
        proposed_payload: JSON.stringify(proposedPayload),
      },
      {
        onSuccess: () => {
          toast.success("Pull request created")
          navigate(`/global-values/${name}`)
        },
        onError: (err) => {
          toast.error("Failed to create pull request", {
            description: (err as Error).message,
          })
        },
      },
    )
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Create Pull Request</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Proposing changes to <span className="font-mono font-medium">{name}</span>
        </p>
      </div>

      <div className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="pr-title">Title</Label>
          <Input
            id="pr-title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Brief description of changes"
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="pr-desc">Description</Label>
          <Textarea
            id="pr-desc"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Optional details about this change"
            rows={3}
          />
        </div>
      </div>

      <div className="space-y-2">
        <h2 className="text-sm font-medium">Changes</h2>
        <div className="rounded-lg border">
          <div className="grid grid-cols-[1fr_1fr_1fr] gap-2 border-b bg-muted/50 px-4 py-2 text-sm font-medium text-muted-foreground">
            <span>Key</span>
            <span>Current</span>
            <span>Proposed</span>
          </div>
          {allKeys.map((key) => {
            const currentVal = current?.payload[key]
            const proposedVal = proposedPayload[key]
            const isAdded = currentVal === undefined
            const isRemoved = proposedVal === undefined
            const isChanged = !isAdded && !isRemoved && String(currentVal) !== String(proposedVal)
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
                <span className={`${unchanged ? "text-muted-foreground" : ""} ${isRemoved ? "line-through text-red-600" : ""}`}>
                  {currentVal !== undefined ? String(currentVal) : "—"}
                </span>
                <span className={`${unchanged ? "text-muted-foreground" : ""} ${isAdded ? "text-green-600" : isChanged ? "text-yellow-600" : ""}`}>
                  {proposedVal !== undefined ? String(proposedVal) : "—"}
                </span>
              </div>
            )
          })}
        </div>
      </div>

      <div className="flex gap-3">
        <Button
          onClick={handleSubmit}
          disabled={!title.trim() || createPR.isPending}
        >
          {createPR.isPending ? "Creating..." : "Create Pull Request"}
        </Button>
        <Button
          variant="outline"
          onClick={() => navigate(-1)}
        >
          Cancel
        </Button>
      </div>
    </div>
  )
}
