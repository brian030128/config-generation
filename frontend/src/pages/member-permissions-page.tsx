import { useEffect, useState } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { toast } from "sonner"
import { ArrowLeft } from "lucide-react"
import type { MemberPermissions } from "@/api/types"
import type { EnvLevel, TemplateCaps } from "@/lib/role-permissions"
import {
  useMemberPermissions,
  useProjectMembers,
  useSetMemberPermissions,
} from "@/hooks/use-projects"
import { useEnvironments } from "@/hooks/use-environments"
import { getApiErrorMessage } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { CapabilityEditor } from "@/components/permissions/capability-editor"

export default function MemberPermissionsPage() {
  const { name: projectName, userId } = useParams<{
    name: string
    userId: string
  }>()
  const userIdNum = Number(userId)
  const navigate = useNavigate()

  const membersQuery = useProjectMembers(projectName!)
  const member = membersQuery.data?.items.find((m) => m.user_id === userIdNum)
  const displayName = member
    ? member.display_name || member.username
    : `user ${userIdNum}`

  const envsQuery = useEnvironments(projectName!)
  const environments = envsQuery.data?.items ?? []

  const permsQuery = useMemberPermissions(projectName!, userIdNum)
  const setPermissions = useSetMemberPermissions(projectName!, userIdNum)

  const [templates, setTemplates] = useState<TemplateCaps>({
    read_templates: false,
    write_templates: false,
    delete_templates: false,
  })
  const [envLevels, setEnvLevels] = useState<Record<string, EnvLevel>>({})

  // Seed the editable draft once the current permissions arrive.
  useEffect(() => {
    const perms = permsQuery.data
    if (!perms) return
    setTemplates({
      read_templates: perms.read_templates,
      write_templates: perms.write_templates,
      delete_templates: perms.delete_templates,
    })
    const levels: Record<string, EnvLevel> = {}
    for (const e of perms.environments ?? []) {
      levels[e.env] = e.write ? "write" : e.read ? "read" : "none"
    }
    setEnvLevels(levels)
  }, [permsQuery.data])

  function levelFor(env: string): EnvLevel {
    return envLevels[env] ?? "none"
  }

  function handleSave() {
    const payload: MemberPermissions = {
      ...templates,
      environments: environments
        .map((e) => ({ env: e.name, level: levelFor(e.name) }))
        .filter((e) => e.level !== "none")
        .map((e) => ({
          env: e.env,
          read: true,
          write: e.level === "write",
        })),
    }
    setPermissions.mutate(payload, {
      onSuccess: () => {
        toast.success(`Updated permissions for ${displayName}`)
        navigate(`/projects/${projectName}`)
      },
      onError: (err) => {
        toast.error("Failed to update permissions", {
          description: getApiErrorMessage(err),
        })
      },
    })
  }

  const loading = permsQuery.isLoading || envsQuery.isLoading
  const loadError = permsQuery.error || envsQuery.error

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div className="space-y-2">
        <button
          onClick={() => navigate(`/projects/${projectName}`)}
          className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="h-4 w-4" />
          {projectName} / Members
        </button>
        <div>
          <h1 className="text-2xl font-semibold">Permissions · {displayName}</h1>
          {member && (
            <p className="text-sm text-muted-foreground">@{member.username}</p>
          )}
        </div>
      </div>

      {loading && (
        <p className="text-sm text-muted-foreground">Loading permissions...</p>
      )}

      {loadError && !loading && (
        <p className="text-sm text-destructive">
          {getApiErrorMessage(loadError)}
        </p>
      )}

      {!loading && !loadError && (
        <>
          <CapabilityEditor
            environments={environments}
            templates={templates}
            onTemplatesChange={setTemplates}
            envLevels={envLevels}
            onEnvLevelChange={(env, level) =>
              setEnvLevels((m) => ({ ...m, [env]: level }))
            }
          />

          <div className="flex justify-end gap-3">
            <Button
              variant="outline"
              onClick={() => navigate(`/projects/${projectName}`)}
              disabled={setPermissions.isPending}
            >
              Cancel
            </Button>
            <Button onClick={handleSave} disabled={setPermissions.isPending}>
              {setPermissions.isPending ? "Saving..." : "Save changes"}
            </Button>
          </div>
        </>
      )}
    </div>
  )
}
