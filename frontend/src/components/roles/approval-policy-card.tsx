import { useEffect, useState } from "react"
import { toast } from "sonner"
import {
  useRoles,
  useUpdateGvApprovalCondition,
  useUpdateProjectApprovalCondition,
} from "@/hooks/use-roles"
import { validateApprovalCondition } from "@/lib/approval-condition"
import { getApiErrorMessage } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"

// ApprovalPolicyCard shows and edits a project's / global-values entry's PR
// approval condition. Roles are global; it validates the condition against the
// global role names so an un-approvable (dangling-role) condition can't be saved.
export function ApprovalPolicyCard({
  kind,
  name,
  currentCondition,
}: {
  kind: "project" | "global-values"
  name: string
  currentCondition: string
}) {
  const rolesQuery = useRoles()
  const knownRoleNames = (rolesQuery.data?.items ?? []).map((r) => r.name)
  const builtin =
    kind === "project" ? `${name}_project_admin` : `${name}_gv_group_admin`

  const [value, setValue] = useState(currentCondition)
  useEffect(() => setValue(currentCondition), [currentCondition])

  const projectUpdate = useUpdateProjectApprovalCondition(
    kind === "project" ? name : "",
  )
  const gvUpdate = useUpdateGvApprovalCondition(
    kind === "global-values" ? name : "",
  )
  const pending = projectUpdate.isPending || gvUpdate.isPending

  const validation = validateApprovalCondition(value, knownRoleNames, [builtin])
  const dirty = value.trim() !== currentCondition.trim()

  function handleSave() {
    if (!validation.valid) return
    const handlers = {
      onSuccess: () => toast.success("Approval condition updated"),
      onError: (err: unknown) =>
        toast.error("Failed to update approval condition", {
          description: getApiErrorMessage(err),
        }),
    }
    if (kind === "project") projectUpdate.mutate(value.trim(), handlers)
    else gvUpdate.mutate(value.trim(), handlers)
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Approval policy</CardTitle>
        <CardDescription>
          Pull requests here merge only once this condition is met. Reference
          roles by their global name, e.g.{" "}
          <code className="text-xs">
            1 x {builtin} AND 1 x release_manager
          </code>
          {"."}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-2">
        <Input
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder={`1 x ${builtin}`}
        />
        {!validation.valid && (
          <ul className="space-y-0.5 text-xs text-destructive">
            {validation.errors.map((e) => (
              <li key={e}>{e}</li>
            ))}
          </ul>
        )}
        <div className="flex justify-end">
          <Button
            size="sm"
            onClick={handleSave}
            disabled={pending || !validation.valid || !dirty}
          >
            {pending ? "Saving..." : "Save policy"}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
