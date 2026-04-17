import { useState } from "react"
import { toast } from "sonner"
import { useCreateTemplate } from "@/hooks/use-templates"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Plus } from "lucide-react"

export function CreateTemplateDialog({
  projectName,
}: {
  projectName: string
}) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [body, setBody] = useState("")
  const [commitMsg, setCommitMsg] = useState("")
  const createTemplate = useCreateTemplate(projectName)

  function reset() {
    setName("")
    setBody("")
    setCommitMsg("")
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim() || !body.trim()) return
    createTemplate.mutate(
      {
        template_name: name.trim(),
        body: body,
        commit_message: commitMsg.trim() || undefined,
      },
      {
        onSuccess: () => {
          toast.success(`Template "${name.trim()}" created`)
          setOpen(false)
          reset()
        },
        onError: (err) => {
          toast.error("Failed to create template", {
            description: (err as Error).message,
          })
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button size="sm">
          <Plus className="mr-2 h-4 w-4" />
          New Template
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Create Template</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="tmpl-name">Template Name</Label>
            <Input
              id="tmpl-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="app.yaml"
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="tmpl-body">Body</Label>
            <Textarea
              id="tmpl-body"
              value={body}
              onChange={(e) => setBody(e.target.value)}
              placeholder="Go template content..."
              rows={8}
              className="font-mono text-sm"
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="tmpl-commit">Commit Message</Label>
            <Input
              id="tmpl-commit"
              value={commitMsg}
              onChange={(e) => setCommitMsg(e.target.value)}
              placeholder="Optional"
            />
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={createTemplate.isPending}>
              {createTemplate.isPending ? "Creating..." : "Create"}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
