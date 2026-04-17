import { useParams } from "react-router-dom"
import { useProject } from "@/hooks/use-projects"
import { useTemplates } from "@/hooks/use-templates"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { TemplateList } from "@/components/templates/template-list"
import { EnvironmentList } from "@/components/environments/environment-list"

export default function ProjectPage() {
  const { name } = useParams<{ name: string }>()
  const { data: project, isLoading, error } = useProject(name!)
  const { data: templates } = useTemplates(name!)

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

  const templateCount = templates?.count ?? 0

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">{project.name}</h1>
        {project.description && (
          <p className="text-muted-foreground">{project.description}</p>
        )}
      </div>

      <Tabs defaultValue="templates">
        <TabsList>
          <TabsTrigger value="templates">Templates</TabsTrigger>
          <TabsTrigger value="environments">Environments</TabsTrigger>
        </TabsList>

        <TabsContent value="templates" className="mt-4">
          <TemplateList projectName={name!} />
        </TabsContent>

        <TabsContent value="environments" className="mt-4">
          <EnvironmentList
            projectName={name!}
            templateCount={templateCount}
          />
        </TabsContent>

      </Tabs>
    </div>
  )
}
