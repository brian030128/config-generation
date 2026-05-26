import { useNavigate, useParams } from "react-router-dom"
import { useProject } from "@/hooks/use-projects"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Button } from "@/components/ui/button"
import { TemplateList } from "@/components/templates/template-list"
import { EnvironmentList } from "@/components/environments/environment-list"
import { MemberList } from "@/components/projects/member-list"
import { ArrowUpRight } from "lucide-react"

export default function ProjectPage() {
  const { name } = useParams<{ name: string }>()
  const { data: project, isLoading, error } = useProject(name!)
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

      <Tabs defaultValue="templates">
        <TabsList>
          <TabsTrigger value="templates">Templates</TabsTrigger>
          <TabsTrigger value="environments">Environments</TabsTrigger>
          <TabsTrigger value="members">Members</TabsTrigger>
        </TabsList>

        <TabsContent value="templates" className="mt-4">
          <TemplateList projectName={name!} />
        </TabsContent>

        <TabsContent value="environments" className="mt-4">
          <EnvironmentList projectName={name!} />
        </TabsContent>

        <TabsContent value="members" className="mt-4">
          <MemberList projectName={name!} />
        </TabsContent>
      </Tabs>
    </div>
  )
}
