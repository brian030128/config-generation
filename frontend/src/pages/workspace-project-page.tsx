import { useState } from "react"
import { useParams, useNavigate } from "react-router-dom"
import { toast } from "sonner"
import { useProject } from "@/hooks/use-projects"
import {
  useActiveDraft,
  useSubmitDraft,
  useValidateWorkspace,
} from "@/hooks/use-pull-requests"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { TemplateList } from "@/components/templates/template-list"
import { EnvironmentList } from "@/components/environments/environment-list"
import type { WorkspaceProblem } from "@/api/types"
import { ArrowLeft, AlertTriangle } from "lucide-react"

function problemText(p: WorkspaceProblem): string {
  if (p.kind === "no_environments") {
    return "Add an environment before submitting — templates have no values to render against."
  }
  if (p.kind === "missing_values" && p.missing_keys?.length) {
    return `Environment "${p.environment_name}" is missing values for template "${p.template_name}": ${p.missing_keys.join(", ")}`
  }
  return p.message
}

export default function WorkspaceProjectPage() {
  const { name = "" } = useParams<{ name: string }>()
  const navigate = useNavigate()
  const { data: project, isLoading: projectLoading } = useProject(name)
  const { data: draft, isLoading: draftLoading } = useActiveDraft(name)
  const submitDraft = useSubmitDraft()

  const [submitOpen, setSubmitOpen] = useState(false)
  const [submitTitle, setSubmitTitle] = useState("")
  const [submitDesc, setSubmitDesc] = useState("")

  const changeCount = draft?.changes?.length ?? 0
  const isDraftWithChanges = draft?.status === "draft" && changeCount > 0

  const { data: validation } = useValidateWorkspace(name, isDraftWithChanges)
  const problems = validation?.problems ?? []
  const isInvalid = !!validation && !validation.valid

  // Which templates/envs have changes in the draft
  const modifiedTemplates = new Set(
    draft?.changes
      ?.filter((c) => c.object_type === "template")
      .map((c) => c.template_name) ?? [],
  )

  function handleSubmit() {
    if (!draft || !submitTitle.trim()) return
    submitDraft.mutate(
      { id: draft.id, title: submitTitle.trim(), description: submitDesc.trim() || undefined },
      {
        onSuccess: () => {
          toast.success("PR submitted for review")
          setSubmitOpen(false)
          setSubmitTitle("")
          setSubmitDesc("")
        },
        onError: (err) => {
          const e = err as { response?: { data?: { error?: string } }; message?: string }
          toast.error("Failed to submit", {
            description: e.response?.data?.error ?? e.message ?? "Unknown error",
          })
        },
      },
    )
  }

  if (projectLoading || draftLoading) {
    return <p className="text-muted-foreground">Loading...</p>
  }

  if (!project) {
    return <p className="text-destructive">Project not found</p>
  }

  return (
    <div className="space-y-6">
      <button
        onClick={() => navigate("/workspace")}
        className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="h-4 w-4" />
        Workspace
      </button>

      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-semibold">{project.name}</h1>
            {draft && (
              <Badge variant={draft.status === "draft" ? "secondary" : "default"}>
                {draft.status}
              </Badge>
            )}
          </div>
          {draft && (
            <p className="mt-1 text-sm text-muted-foreground">
              {changeCount} change{changeCount === 1 ? "" : "s"}
              {draft.title ? ` · PR #${draft.id}: ${draft.title}` : ""}
            </p>
          )}
        </div>
        <div className="flex gap-2">
          {isDraftWithChanges && (
            <Button onClick={() => setSubmitOpen(true)} disabled={isInvalid}>
              Submit PR
            </Button>
          )}
          {draft && draft.status !== "draft" && (
            <Button
              variant="outline"
              onClick={() => navigate(`/pull-requests/${draft.id}`)}
            >
              View PR
            </Button>
          )}
        </div>
      </div>

      {/* Change summary */}
      {changeCount > 0 && (
        <div className="rounded-lg border bg-muted/30 p-3">
          <h3 className="text-sm font-medium mb-2">Staged Changes ({changeCount})</h3>
          <div className="space-y-1">
            {draft?.changes?.map((c) => (
              <div key={c.id} className="flex items-center gap-2 text-sm">
                <Badge variant="outline" className="text-xs">
                  {c.object_type}
                </Badge>
                <span className="font-mono text-xs">
                  {c.object_type === "template"
                    ? c.template_name
                    : c.environment_name}
                </span>
                {c.object_type !== "environment" && (
                  <span className="text-muted-foreground text-xs">
                    draft change
                  </span>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Validation problems — block submit until resolved */}
      {isDraftWithChanges && isInvalid && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-3">
          <h3 className="mb-2 flex items-center gap-2 text-sm font-medium text-destructive">
            <AlertTriangle className="h-4 w-4" />
            Resolve {problems.length} problem{problems.length === 1 ? "" : "s"} before submitting
          </h3>
          <ul className="list-disc space-y-1 pl-5 text-sm text-muted-foreground">
            {problems.map((p) => {
              const text = problemText(p)
              return <li key={text}>{text}</li>
            })}
          </ul>
        </div>
      )}

      <Tabs defaultValue="templates">
        <TabsList>
          <TabsTrigger value="templates">Templates</TabsTrigger>
          <TabsTrigger value="environments">Environments</TabsTrigger>
        </TabsList>

        <TabsContent value="templates" className="mt-4">
          <TemplateList
            projectName={name}
            workspaceMode
            modifiedTemplates={modifiedTemplates}
          />
        </TabsContent>

        <TabsContent value="environments" className="mt-4">
          <EnvironmentList
            projectName={name}
            workspaceMode
          />
        </TabsContent>
      </Tabs>

      {/* Submit PR dialog */}
      <Dialog open={submitOpen} onOpenChange={setSubmitOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Submit PR for Review</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="pr-title">Title</Label>
              <Input
                id="pr-title"
                value={submitTitle}
                onChange={(e) => setSubmitTitle(e.target.value)}
                placeholder="Brief description of changes"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="pr-desc">Description</Label>
              <Textarea
                id="pr-desc"
                value={submitDesc}
                onChange={(e) => setSubmitDesc(e.target.value)}
                placeholder="Optional details"
                rows={3}
              />
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setSubmitOpen(false)}>
                Cancel
              </Button>
              <Button
                onClick={handleSubmit}
                disabled={!submitTitle.trim() || submitDraft.isPending || isInvalid}
              >
                {submitDraft.isPending ? "Submitting..." : "Submit"}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
