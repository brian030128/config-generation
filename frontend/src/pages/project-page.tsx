import { useNavigate, useParams } from "react-router-dom"
import { useProject, useProjectMembers } from "@/hooks/use-projects"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Button } from "@/components/ui/button"
import { TemplateList } from "@/components/templates/template-list"
import { EnvironmentList } from "@/components/environments/environment-list"
import { MemberList } from "@/components/projects/member-list"
import { ApprovalPolicyCard } from "@/components/roles/approval-policy-card"
import { ArrowUpRight } from "lucide-react"

type ProjectTab = "templates" | "environments" | "members" | "approval"

export default function ProjectPage({ tab = "templates" }: Readonly<{ tab?: ProjectTab }>) {
  const { name = "" } = useParams<{ name: string }>()
  const { data: project, isLoading, error } = useProject(name)
  const membersQuery = useProjectMembers(name)
  const canManage = membersQuery.data?.viewer_can_manage ?? false
  const navigate = useNavigate()

  if (isLoading) {
    return <p className="text-muted-foreground">Loading project...</p>
  }

  if (error || !project) {
    return (
      <p className="text-destructive">
        Failed to load project: {(error as Error)?.message ?? "Not found"}
      </p>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold">{project.name}</h1>
          <p className="text-sm text-muted-foreground">
            {project.description || "No description provided"}
          </p>
        </div>
        <Button onClick={() => navigate(`/workspace/${project.name}`)}>
          <ArrowUpRight />
          Open Workspace
        </Button>
      </div>

      <Tabs
        value={tab}
        onValueChange={(v) =>
          navigate(
            v === "templates"
              ? `/projects/${name}`
              : `/projects/${name}/${v}`,
          )
        }
      >
        <TabsList>
          <TabsTrigger value="templates">Templates</TabsTrigger>
          <TabsTrigger value="environments">Environments</TabsTrigger>
          <TabsTrigger value="members">Members</TabsTrigger>
          {canManage && <TabsTrigger value="approval">Approval</TabsTrigger>}
        </TabsList>

        <TabsContent value="templates" className="mt-4">
          <TemplateList projectName={name} />
        </TabsContent>

        <TabsContent value="environments" className="mt-4">
          <EnvironmentList projectName={name} />
        </TabsContent>

        <TabsContent value="members" className="mt-4">
          <MemberList projectName={name} />
        </TabsContent>

        {canManage && (
          <TabsContent value="approval" className="mt-4">
            <div className="max-w-2xl space-y-3">
              <ApprovalPolicyCard
                kind="project"
                name={name}
                currentCondition={project.approval_condition}
              />
              <p className="text-sm text-muted-foreground">
                Roles are managed globally on the{" "}
                <button
                  className="underline"
                  onClick={() => navigate("/roles")}
                >
                  Roles
                </button>{" "}
                page. Create approver roles there, then reference them by name
                above.
              </p>
            </div>
          </TabsContent>
        )}
      </Tabs>
    </div>
  )
}
