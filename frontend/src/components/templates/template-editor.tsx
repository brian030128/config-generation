import { useState, useRef, useEffect, useCallback } from "react"
import { EditorView, keymap } from "@codemirror/view"
import { EditorState, Compartment } from "@codemirror/state"
import { basicSetup } from "codemirror"
import { javascript } from "@codemirror/lang-javascript"
import { oneDark } from "@codemirror/theme-one-dark"
import { toast } from "sonner"
import {
  useTemplate,
  useAppendTemplateVersion,
} from "@/hooks/use-templates"
import { templatesApi } from "@/api/templates"
import { VersionHistory } from "./version-history"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
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
  const [commitMsg, setCommitMsg] = useState("")
  const [selectedVersion, setSelectedVersion] = useState<number | null>(null)
  const [isReadOnly, setIsReadOnly] = useState(false)
  const readOnlyCompartment = useRef(new Compartment())

  const { data: template } = useTemplate(projectName, templateName)
  const appendVersion = useAppendTemplateVersion(projectName, templateName)

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

  // Initialize editor
  useEffect(() => {
    if (!editorRef.current || !template) return
    if (viewRef.current) {
      viewRef.current.destroy()
    }
    const state = createEditorState(template.body, false)
    const view = new EditorView({ state, parent: editorRef.current })
    viewRef.current = view
    setSelectedVersion(template.version_id)
    return () => view.destroy()
    // Only run on mount and when template identity changes
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [template?.id])

  // Handle version selection
  async function handleSelectVersion(versionId: number) {
    setSelectedVersion(versionId)
    if (versionId === latestVersionId) {
      setIsReadOnly(false)
      if (viewRef.current && template) {
        viewRef.current.dispatch({
          changes: {
            from: 0,
            to: viewRef.current.state.doc.length,
            insert: template.body,
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
    appendVersion.mutate(
      { body, commit_message: commitMsg.trim() || undefined },
      {
        onSuccess: () => {
          toast.success("Version saved")
          setCommitMsg("")
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
            <div className="flex items-end gap-3">
              <div className="flex-1 space-y-1">
                <Label htmlFor="commit-msg" className="text-xs">
                  Commit Message
                </Label>
                <Input
                  id="commit-msg"
                  value={commitMsg}
                  onChange={(e) => setCommitMsg(e.target.value)}
                  placeholder="Optional commit message"
                  className="text-sm"
                />
              </div>
              <Button
                id="tmpl-save-btn"
                onClick={handleSave}
                disabled={appendVersion.isPending}
                size="sm"
              >
                {appendVersion.isPending ? "Saving..." : "Save"}
              </Button>
            </div>
          )}
        </div>

        <Separator orientation="vertical" className="h-auto" />

        <div className="w-48 shrink-0">
          <VersionHistory
            projectName={projectName}
            templateName={templateName}
            selectedVersion={selectedVersion}
            onSelectVersion={handleSelectVersion}
          />
        </div>
      </div>
    </div>
  )
}
