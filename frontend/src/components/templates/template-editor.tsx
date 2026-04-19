import { useState, useRef, useEffect, useCallback, useMemo } from "react"
import { EditorView, keymap } from "@codemirror/view"
import { EditorState, Compartment } from "@codemirror/state"
import { basicSetup } from "codemirror"
import { javascript } from "@codemirror/lang-javascript"
import { oneDark } from "@codemirror/theme-one-dark"
import { toast } from "sonner"
import { useTemplate } from "@/hooks/use-templates"
import { useStageChange, useActiveDraft } from "@/hooks/use-pull-requests"
import { templatesApi } from "@/api/templates"
import { VersionHistory } from "./version-history"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"

interface TemplateEditorProps {
  projectName: string
  templateName: string
  onClose: () => void
}

export function TemplateEditor({
  projectName,
  templateName,
  onClose,
}: TemplateEditorProps) {
  const editorRef = useRef<HTMLDivElement>(null)
  const viewRef = useRef<EditorView | null>(null)
  const [selectedVersion, setSelectedVersion] = useState<number | null>(null)
  const [isReadOnly, setIsReadOnly] = useState(false)
  const readOnlyCompartment = useRef(new Compartment())

  const { data: template } = useTemplate(projectName, templateName)
  const { data: draft } = useActiveDraft(projectName)
  const stageChange = useStageChange(projectName)

  // Find staged change for this template in the draft
  const stagedChange = useMemo(
    () =>
      draft?.changes?.find(
        (c) => c.object_type === "template" && c.template_name === templateName,
      ),
    [draft, templateName],
  )

  // Use staged body if available, otherwise DB body
  const initialBody = stagedChange?.proposed_payload ?? template?.body ?? null
  const isNewTemplate = !template
  const latestVersionId = template?.version_id ?? null

  const createEditorState = useCallback(
    (doc: string, readOnly: boolean) => {
      return EditorState.create({
        doc,
        extensions: [
          basicSetup,
          javascript(),
          oneDark,
          EditorView.lineWrapping,
          readOnlyCompartment.current.of(EditorState.readOnly.of(readOnly)),
          keymap.of([
            {
              key: "Mod-s",
              run: () => {
                if (!readOnly) {
                  document
                    .getElementById("tmpl-save-btn")
                    ?.click()
                }
                return true
              },
            },
          ]),
        ],
      })
    },
    [],
  )

  // Initialize editor when body is available
  useEffect(() => {
    if (!editorRef.current || initialBody === null) return
    if (viewRef.current) {
      viewRef.current.destroy()
    }
    const state = createEditorState(initialBody, false)
    const view = new EditorView({ state, parent: editorRef.current })
    viewRef.current = view
    if (!isNewTemplate && template) {
      setSelectedVersion(template.version_id)
    }
    return () => view.destroy()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [template?.id, stagedChange?.id, initialBody === null])

  // Handle version selection
  async function handleSelectVersion(versionId: number) {
    setSelectedVersion(versionId)
    if (versionId === latestVersionId) {
      setIsReadOnly(false)
      if (viewRef.current) {
        const body = stagedChange?.proposed_payload ?? template?.body ?? ""
        viewRef.current.dispatch({
          changes: {
            from: 0,
            to: viewRef.current.state.doc.length,
            insert: body,
          },
          effects: readOnlyCompartment.current.reconfigure(
            EditorState.readOnly.of(false),
          ),
        })
      }
      return
    }
    try {
      const ver = await templatesApi.getVersion(
        projectName,
        templateName,
        versionId,
      )
      setIsReadOnly(true)
      if (viewRef.current) {
        viewRef.current.dispatch({
          changes: {
            from: 0,
            to: viewRef.current.state.doc.length,
            insert: ver.body,
          },
          effects: readOnlyCompartment.current.reconfigure(
            EditorState.readOnly.of(true),
          ),
        })
      }
    } catch {
      toast.error("Failed to load version")
    }
  }

  function handleSave() {
    if (!viewRef.current || isReadOnly) return
    const body = viewRef.current.state.doc.toString()
    stageChange.mutate(
      {
        object_type: "template",
        template_name: templateName,
        proposed_payload: body,
      },
      {
        onSuccess: () => {
          toast.success("Change staged in draft")
        },
        onError: (err) => {
          toast.error("Failed to save", {
            description: (err as Error).message,
          })
        },
      },
    )
  }

  return (
    <div className="space-y-4 rounded-lg border p-4">
      <div className="flex items-center justify-between">
        <h3 className="font-medium">{templateName}</h3>
        <Button variant="ghost" size="sm" onClick={onClose}>
          Close
        </Button>
      </div>

      <div className="flex gap-4">
        <div className="flex-1 space-y-3">
          <div
            ref={editorRef}
            className="min-h-[300px] overflow-hidden rounded border [&_.cm-editor]:max-h-[500px] [&_.cm-editor]:min-h-[300px]"
          />
          {isReadOnly && (
            <p className="text-sm text-muted-foreground">
              Viewing v{selectedVersion} (read-only).{" "}
              <button
                className="underline"
                onClick={() =>
                  latestVersionId && handleSelectVersion(latestVersionId)
                }
              >
                Return to latest
              </button>
            </p>
          )}
          {!isReadOnly && (
            <div className="flex justify-end">
              <Button
                id="tmpl-save-btn"
                onClick={handleSave}
                disabled={stageChange.isPending}
                size="sm"
              >
                {stageChange.isPending ? "Saving..." : "Save to Draft"}
              </Button>
            </div>
          )}
        </div>

        {!isNewTemplate && (
          <>
            <Separator orientation="vertical" className="h-auto" />
            <div className="w-48 shrink-0">
              <VersionHistory
                projectName={projectName}
                templateName={templateName}
                selectedVersion={selectedVersion}
                onSelectVersion={handleSelectVersion}
              />
            </div>
          </>
        )}
      </div>
    </div>
  )
}
