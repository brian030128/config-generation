import { useState } from "react"
import { useNavigate } from "react-router-dom"
import { useEnvironments } from "@/hooks/use-environments"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Plus } from "lucide-react"

export function AddEnvironmentDialog({
  projectName,
}: {
  projectName: string
}) {
  const [open, setOpen] = useState(false)
  const [selectedEnv, setSelectedEnv] = useState("")
  const { data } = useEnvironments()
  const navigate = useNavigate()

  const environments = data?.items ?? []

  function handleAdd() {
    if (!selectedEnv) return
    setOpen(false)
    navigate(`/projects/${projectName}/env/${selectedEnv}`)
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button size="sm">
          <Plus className="mr-2 h-4 w-4" />
          Add Environment
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add Environment</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>Environment</Label>
            <Select value={selectedEnv} onValueChange={setSelectedEnv}>
              <SelectTrigger>
                <SelectValue placeholder="Select an environment" />
              </SelectTrigger>
              <SelectContent>
                {environments.map((env) => (
                  <SelectItem key={env.id} value={env.name}>
                    {env.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleAdd} disabled={!selectedEnv}>
              Continue
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
