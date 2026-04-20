import { useState, useEffect, useCallback } from "react"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import {
  Rocket,
  ChevronDown,
  ChevronRight,
  AlertCircle,
  Download,
  RefreshCw,
  Loader2,
} from "lucide-react"
import { toast } from "sonner"
import JSZip from "jszip"
import { saveAs } from "file-saver"
import { cn } from "@/lib/utils"

import { useProjects } from "@/hooks/use-projects"
import { useEnvironments } from "@/hooks/use-environments"
import { useTemplates, useTemplateVersions } from "@/hooks/use-templates"
import { useValues } from "@/hooks/use-values"
import { useGlobalValueVersions } from "@/hooks/use-global-values"
import {
  useLatestDeployment,
  useDeployPreview,
  useExecuteDeploy,
} from "@/hooks/use-deployments"
import { DiffViewer } from "@/components/deploy/diff-viewer"
import type {
  DeployPreviewResponse,
  TemplateRenderResult,
} from "@/api/types"

export default function DeployPage() {
  const [projectName, setProjectName] = useState("")
  const [envName, setEnvName] = useState("")
  const [templateVersions, setTemplateVersions] = useState<
    Record<string, number>
  >({})
  const [valuesVersionId, setValuesVersionId] = useState<number>(0)
  const [gvVersions, setGvVersions] = useState<Record<string, number>>({})
  const [selectedTemplate, setSelectedTemplate] = useState("")
  const [commitMessage, setCommitMessage] = useState("")
  const [showConfirm, setShowConfirm] = useState(false)
  const [versionsInitialized, setVersionsInitialized] = useState(false)

  // Data queries
  const { data: projectsData } = useProjects()
  const { data: envsData } = useEnvironments(projectName)
  const { data: templatesData } = useTemplates(projectName)
  const { data: valuesData, isLoading: valuesLoading } = useValues(projectName, envName)
  const { data: latestDeploy, error: latestDeployError } =
    useLatestDeployment(projectName, envName)

  // Preview and deploy mutations
  const previewMutation = useDeployPreview(projectName, envName)
  const deployMutation = useExecuteDeploy(projectName, envName)

  const projects = projectsData?.items ?? []
  const environments = envsData?.items ?? []
  const templates = templatesData?.items ?? []
  const preview = previewMutation.data as DeployPreviewResponse | undefined

  // Reset env when project changes
  useEffect(() => {
    setEnvName("")
    setVersionsInitialized(false)
    setSelectedTemplate("")
    previewMutation.reset()
  }, [projectName])

  // Initialize version defaults from last deployment or latest versions
  useEffect(() => {
    if (!projectName || !envName || versionsInitialized) return
    if (templates.length === 0) return
    if (valuesLoading) return // wait for values query to complete

    // Build default template versions (latest)
    const defaultTmplVersions: Record<string, number> = {}
    for (const t of templates) {
      defaultTmplVersions[t.template_name] = t.version_id
    }

    const defaultValuesVersion = valuesData?.version_id ?? 0

    if (latestDeploy && !latestDeployError) {
      // Use last deployment versions as defaults, falling back to latest for new templates
      const mergedTmplVersions = { ...defaultTmplVersions }
      for (const [name, ver] of Object.entries(
        latestDeploy.template_versions,
      )) {
        if (name in mergedTmplVersions) {
          mergedTmplVersions[name] = ver
        }
      }
      setTemplateVersions(mergedTmplVersions)
      setValuesVersionId(latestDeploy.values_version_id || defaultValuesVersion)
      setGvVersions(latestDeploy.global_values_versions ?? {})
    } else {
      setTemplateVersions(defaultTmplVersions)
      setValuesVersionId(defaultValuesVersion)
      // Scan values payload for global value refs to determine defaults
      if (valuesData?.payload) {
        const refs = extractGlobalValueRefs(valuesData.payload)
        const defaultGvVersions: Record<string, number> = {}
        for (const name of refs) {
          defaultGvVersions[name] = 0 // will be resolved when version lists load
        }
        setGvVersions(defaultGvVersions)
      }
    }
    setVersionsInitialized(true)
    if (!selectedTemplate && templates.length > 0) {
      setSelectedTemplate(templates[0].template_name)
    }
  }, [
    projectName,
    envName,
    templates,
    valuesData,
    valuesLoading,
    latestDeploy,
    latestDeployError,
    versionsInitialized,
  ])

  // Trigger preview when versions are initialized and all resolved
  const allGvResolved = Object.values(gvVersions).every((v) => v > 0) || Object.keys(gvVersions).length === 0
  const canPreview = versionsInitialized && valuesVersionId > 0 && Object.keys(templateVersions).length > 0 && allGvResolved

  const triggerPreview = useCallback(() => {
    if (!projectName || !envName || !canPreview) return

    previewMutation.mutate({
      template_versions: templateVersions,
      values_version_id: valuesVersionId,
      global_values_versions: gvVersions,
    })
  }, [
    projectName,
    envName,
    canPreview,
    templateVersions,
    valuesVersionId,
    gvVersions,
  ])

  // Auto-trigger preview when all versions are resolved
  useEffect(() => {
    if (canPreview) {
      triggerPreview()
    }
  }, [canPreview])

  const handleDeploy = () => {
    deployMutation.mutate(
      {
        template_versions: templateVersions,
        values_version_id: valuesVersionId,
        global_values_versions: gvVersions,
        commit_message: commitMessage || undefined,
      },
      {
        onSuccess: (data) => {
          // Generate and download zip
          const zip = new JSZip()
          for (const result of data.results) {
            if (result.rendered_output) {
              zip.file(result.template_name, result.rendered_output)
            }
          }
          zip.generateAsync({ type: "blob" }).then((blob) => {
            saveAs(blob, `${projectName}-${envName}-deploy.zip`)
          })
          toast.success(
            `Deployed successfully (deployment #${data.deployment_id})`,
          )
          setShowConfirm(false)
          setCommitMessage("")
          // Re-trigger preview to refresh diffs
          setVersionsInitialized(false)
        },
        onError: (err: unknown) => {
          const message =
            (err as { response?: { data?: { error?: string } } })?.response
              ?.data?.error ?? "Deploy failed"
          toast.error(message)
        },
      },
    )
  }

  const selectedResult = preview?.results.find(
    (r) => r.template_name === selectedTemplate,
  )
  const hasErrors = preview?.has_errors ?? false

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Rocket className="h-6 w-6" />
        <h1 className="text-2xl font-semibold">Deploy</h1>
      </div>

      {/* Project & Environment selectors */}
      <div className="flex items-end gap-4">
        <div className="space-y-1.5">
          <Label>Project</Label>
          <Select value={projectName} onValueChange={setProjectName}>
            <SelectTrigger className="w-[220px]">
              <SelectValue placeholder="Select project" />
            </SelectTrigger>
            <SelectContent>
              {projects.map((p) => (
                <SelectItem key={p.name} value={p.name}>
                  {p.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="space-y-1.5">
          <Label>Environment</Label>
          <Select
            value={envName}
            onValueChange={(v) => {
              setEnvName(v)
              setVersionsInitialized(false)
              previewMutation.reset()
            }}
            disabled={!projectName}
          >
            <SelectTrigger className="w-[180px]">
              <SelectValue placeholder="Select environment" />
            </SelectTrigger>
            <SelectContent>
              {environments.map((e) => (
                <SelectItem key={e.name} value={e.name}>
                  {e.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {versionsInitialized && (
          <Button
            variant="outline"
            size="sm"
            onClick={triggerPreview}
            disabled={previewMutation.isPending}
          >
            <RefreshCw
              className={cn(
                "h-4 w-4 mr-1",
                previewMutation.isPending && "animate-spin",
              )}
            />
            Refresh Preview
          </Button>
        )}
      </div>

      {!projectName || !envName ? (
        <div className="rounded-lg border border-dashed p-8 text-center text-muted-foreground">
          Select a project and environment to begin.
        </div>
      ) : !versionsInitialized ? (
        <div className="flex items-center gap-2 text-sm text-muted-foreground p-4">
          <Loader2 className="h-4 w-4 animate-spin" />
          Loading version data...
        </div>
      ) : (
        <>
          {/* Version Pinning */}
          <VersionPinning
            projectName={projectName}
            templates={templates}
            templateVersions={templateVersions}
            setTemplateVersions={setTemplateVersions}
            valuesVersionId={valuesVersionId}
            gvVersions={gvVersions}
            setGvVersions={setGvVersions}
          />

          {previewMutation.isPending && (
            <div className="flex items-center gap-2 text-sm text-muted-foreground p-4">
              <Loader2 className="h-4 w-4 animate-spin" />
              Rendering preview...
            </div>
          )}

          {previewMutation.error && (
            <div className="rounded-lg border border-destructive bg-destructive/10 p-4 text-sm text-destructive">
              Failed to load preview:{" "}
              {(
                previewMutation.error as {
                  response?: { data?: { error?: string } }
                }
              )?.response?.data?.error ?? "Unknown error"}
            </div>
          )}

          {preview && (
            <>
              {/* Template tabs */}
              <div className="flex gap-1 flex-wrap">
                {preview.results.map((result) => (
                  <button
                    key={result.template_name}
                    onClick={() => setSelectedTemplate(result.template_name)}
                    className={cn(
                      "flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm transition-colors",
                      selectedTemplate === result.template_name
                        ? "bg-accent text-accent-foreground font-medium"
                        : "text-muted-foreground hover:bg-accent/50",
                    )}
                  >
                    {result.template_name}
                    {result.error && (
                      <AlertCircle className="h-3.5 w-3.5 text-destructive" />
                    )}
                    {!result.error && hasTemplateChanges(result) && (
                      <span className="h-2 w-2 rounded-full bg-blue-500" />
                    )}
                  </button>
                ))}
              </div>

              <Separator />

              {/* Split pane */}
              {selectedResult && (
                <div className="grid grid-cols-2 gap-4 min-h-[400px]">
                  {/* Left pane: Inputs */}
                  <div className="space-y-3 overflow-auto">
                    <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">
                      Inputs
                    </h3>
                    <InputDiffSection
                      title={`Template: ${selectedResult.template_name}`}
                      subtitle={`v${selectedResult.template_version_id}`}
                      hasChanges={
                        selectedResult.previous_template_body == null ||
                        selectedResult.template_body !==
                          selectedResult.previous_template_body
                      }
                      oldText={selectedResult.previous_template_body}
                      newText={selectedResult.template_body}
                    />
                    <InputDiffSection
                      title={`Values (v${preview.values_version_id})`}
                      hasChanges={
                        preview.previous_values == null ||
                        JSON.stringify(preview.values_payload) !==
                          JSON.stringify(preview.previous_values)
                      }
                      oldText={
                        preview.previous_values
                          ? JSON.stringify(preview.previous_values, null, 2)
                          : undefined
                      }
                      newText={JSON.stringify(preview.values_payload, null, 2)}
                    />
                    {Object.entries(preview.global_values).map(
                      ([name, payload]) => {
                        const prevPayload =
                          preview.previous_global_values?.[name]
                        return (
                          <InputDiffSection
                            key={name}
                            title={`Global: ${name} (v${preview.global_values_versions[name]})`}
                            hasChanges={
                              prevPayload == null ||
                              JSON.stringify(payload) !==
                                JSON.stringify(prevPayload)
                            }
                            oldText={
                              prevPayload
                                ? JSON.stringify(prevPayload, null, 2)
                                : undefined
                            }
                            newText={JSON.stringify(payload, null, 2)}
                          />
                        )
                      },
                    )}
                  </div>

                  {/* Right pane: Rendered output */}
                  <div className="space-y-3 overflow-auto">
                    <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">
                      Rendered Output
                    </h3>
                    {selectedResult.error ? (
                      <div className="rounded-lg border border-destructive bg-destructive/10 p-4 space-y-2">
                        <div className="flex items-center gap-2 text-destructive font-medium text-sm">
                          <AlertCircle className="h-4 w-4" />
                          Config generation failed
                        </div>
                        <p className="text-sm text-destructive/90">
                          {selectedResult.error}
                        </p>
                        {selectedResult.error_kind && (
                          <Badge variant="destructive" className="text-xs">
                            {selectedResult.error_kind}
                          </Badge>
                        )}
                      </div>
                    ) : selectedResult.rendered_output != null ? (
                      <div className="rounded-lg border overflow-hidden">
                        <DiffViewer
                          oldText={selectedResult.previous_output ?? ""}
                          newText={selectedResult.rendered_output}
                        />
                      </div>
                    ) : null}
                  </div>
                </div>
              )}

              <Separator />

              {/* Deploy action */}
              <div className="flex items-end gap-4">
                <div className="flex-1 max-w-md space-y-1.5">
                  <Label htmlFor="commit-msg">Commit message (optional)</Label>
                  <Input
                    id="commit-msg"
                    value={commitMessage}
                    onChange={(e) => setCommitMessage(e.target.value)}
                    placeholder="Describe this deployment..."
                  />
                </div>
                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <span>
                        <Button
                          onClick={() => setShowConfirm(true)}
                          disabled={hasErrors || deployMutation.isPending}
                        >
                          <Download className="h-4 w-4 mr-2" />
                          Deploy
                        </Button>
                      </span>
                    </TooltipTrigger>
                    {hasErrors && (
                      <TooltipContent>
                        Fix rendering errors before deploying
                      </TooltipContent>
                    )}
                  </Tooltip>
                </TooltipProvider>
              </div>
            </>
          )}
        </>
      )}

      {/* Confirmation dialog */}
      <Dialog open={showConfirm} onOpenChange={setShowConfirm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Confirm Deployment</DialogTitle>
            <DialogDescription>
              Deploy <strong>{projectName}</strong> to{" "}
              <strong>{envName}</strong>? This will use the exact versions shown
              in this review and download the rendered configs as a zip file.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowConfirm(false)}>
              Cancel
            </Button>
            <Button onClick={handleDeploy} disabled={deployMutation.isPending}>
              {deployMutation.isPending ? (
                <>
                  <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                  Deploying...
                </>
              ) : (
                "Deploy"
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

// --- Sub-components ---

function InputDiffSection({
  title,
  subtitle,
  hasChanges,
  oldText,
  newText,
}: {
  title: string
  subtitle?: string
  hasChanges: boolean
  oldText?: string
  newText: string
}) {
  const [expanded, setExpanded] = useState(hasChanges)

  useEffect(() => {
    setExpanded(hasChanges)
  }, [hasChanges])

  return (
    <div className="rounded-lg border overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex items-center gap-2 w-full px-3 py-2 text-sm hover:bg-muted/50 transition-colors text-left"
      >
        {expanded ? (
          <ChevronDown className="h-3.5 w-3.5" />
        ) : (
          <ChevronRight className="h-3.5 w-3.5" />
        )}
        <span className="font-medium">{title}</span>
        {subtitle && (
          <span className="text-muted-foreground">{subtitle}</span>
        )}
        {!hasChanges && (
          <Badge variant="secondary" className="text-xs ml-auto">
            unchanged
          </Badge>
        )}
      </button>
      {expanded && (
        <div className="border-t">
          <DiffViewer oldText={oldText ?? ""} newText={newText} />
        </div>
      )}
    </div>
  )
}

function VersionPinning({
  projectName,
  templates,
  templateVersions,
  setTemplateVersions,
  valuesVersionId,
  gvVersions,
  setGvVersions,
}: {
  projectName: string
  templates: { template_name: string; version_id: number }[]
  templateVersions: Record<string, number>
  setTemplateVersions: (v: Record<string, number>) => void
  valuesVersionId: number
  gvVersions: Record<string, number>
  setGvVersions: (v: Record<string, number>) => void
}) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div className="rounded-lg border">
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex items-center gap-2 w-full px-3 py-2 text-sm hover:bg-muted/50 transition-colors"
      >
        {expanded ? (
          <ChevronDown className="h-3.5 w-3.5" />
        ) : (
          <ChevronRight className="h-3.5 w-3.5" />
        )}
        <span className="font-medium">Version Pinning</span>
        <span className="text-muted-foreground text-xs">
          ({templates.length} templates, values v{valuesVersionId},{" "}
          {Object.keys(gvVersions).length} global value groups)
        </span>
      </button>
      {expanded && (
        <div className="border-t p-4 space-y-4">
          {/* Template versions */}
          <div className="space-y-2">
            <Label className="text-xs text-muted-foreground uppercase tracking-wider">
              Template Versions
            </Label>
            <div className="flex flex-wrap gap-3">
              {templates.map((t) => (
                <TemplateVersionSelect
                  key={t.template_name}
                  projectName={projectName}
                  templateName={t.template_name}
                  currentVersion={
                    templateVersions[t.template_name] ?? t.version_id
                  }
                  onChange={(ver) =>
                    setTemplateVersions({
                      ...templateVersions,
                      [t.template_name]: ver,
                    })
                  }
                />
              ))}
            </div>
          </div>

          {/* Values version */}
          <div className="space-y-2">
            <Label className="text-xs text-muted-foreground uppercase tracking-wider">
              Values Version
            </Label>
            <div className="text-sm">v{valuesVersionId}</div>
          </div>

          {/* Global value group versions */}
          {Object.keys(gvVersions).length > 0 && (
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground uppercase tracking-wider">
                Global Value Group Versions
              </Label>
              <div className="flex flex-wrap gap-3">
                {Object.entries(gvVersions).map(([name, ver]) => (
                  <GvVersionSelect
                    key={name}
                    gvName={name}
                    currentVersion={ver}
                    onChange={(newVer) =>
                      setGvVersions({ ...gvVersions, [name]: newVer })
                    }
                  />
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function TemplateVersionSelect({
  projectName,
  templateName,
  currentVersion,
  onChange,
}: {
  projectName: string
  templateName: string
  currentVersion: number
  onChange: (ver: number) => void
}) {
  const { data: versionsData } = useTemplateVersions(projectName, templateName)
  const versions = versionsData?.items ?? []

  return (
    <div className="flex items-center gap-2">
      <span className="text-sm">{templateName}:</span>
      <Select
        value={String(currentVersion)}
        onValueChange={(v) => onChange(Number(v))}
      >
        <SelectTrigger className="w-[90px]" size="sm">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {versions.map((v) => (
            <SelectItem key={v.version_id} value={String(v.version_id)}>
              v{v.version_id}
            </SelectItem>
          ))}
          {versions.length === 0 && (
            <SelectItem value={String(currentVersion)}>
              v{currentVersion}
            </SelectItem>
          )}
        </SelectContent>
      </Select>
    </div>
  )
}

function GvVersionSelect({
  gvName,
  currentVersion,
  onChange,
}: {
  gvName: string
  currentVersion: number
  onChange: (ver: number) => void
}) {
  const { data: versionsData } = useGlobalValueVersions(gvName)
  const versions = versionsData?.items ?? []

  // Auto-select latest version if current is 0 (unresolved)
  useEffect(() => {
    if (currentVersion === 0 && versions.length > 0) {
      onChange(versions[0].version_id)
    }
  }, [currentVersion, versions])

  return (
    <div className="flex items-center gap-2">
      <span className="text-sm">{gvName}:</span>
      <Select
        value={String(currentVersion)}
        onValueChange={(v) => onChange(Number(v))}
      >
        <SelectTrigger className="w-[90px]" size="sm">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {versions.map((v) => (
            <SelectItem key={v.version_id} value={String(v.version_id)}>
              v{v.version_id}
            </SelectItem>
          ))}
          {versions.length === 0 && currentVersion > 0 && (
            <SelectItem value={String(currentVersion)}>
              v{currentVersion}
            </SelectItem>
          )}
        </SelectContent>
      </Select>
    </div>
  )
}

// --- Helpers ---

function extractGlobalValueRefs(payload: Record<string, unknown>): string[] {
  const refs = new Set<string>()
  const walk = (v: unknown) => {
    if (typeof v === "string") {
      const matches = v.matchAll(/\$\{(\w+)\.\w+\}/g)
      for (const m of matches) refs.add(m[1])
    } else if (v && typeof v === "object") {
      for (const child of Object.values(v as Record<string, unknown>)) {
        walk(child)
      }
    }
  }
  walk(payload)
  return Array.from(refs)
}

function hasTemplateChanges(result: TemplateRenderResult): boolean {
  if (result.previous_output == null) return true
  return result.rendered_output !== result.previous_output
}
