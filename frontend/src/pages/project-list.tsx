import { useState } from "react"
import { useProjects } from "@/hooks/use-projects"
import { ProjectCard } from "@/components/projects/project-card"
import { CreateProjectDialog } from "@/components/projects/create-project-dialog"
import { Input } from "@/components/ui/input"
import { Search } from "lucide-react"

export default function ProjectListPage() {
  const { data, isLoading, error } = useProjects()
  const [search, setSearch] = useState("")

  const filtered = data?.items.filter((p) =>
    p.name.toLowerCase().includes(search.toLowerCase()),
  ) ?? []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Projects</h1>
        <CreateProjectDialog />
      </div>

      <div className="relative max-w-sm">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          className="pl-9"
          placeholder="Search projects..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {isLoading && (
        <p className="text-muted-foreground">Loading projects...</p>
      )}

      {error && (
        <p className="text-destructive">
          Failed to load projects: {(error as Error).message}
        </p>
      )}

      {!isLoading && filtered.length === 0 && (
        <p className="text-muted-foreground">
          {search ? "No projects match your search." : "No projects yet. Create one to get started."}
        </p>
      )}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {filtered.map((project) => (
          <ProjectCard key={project.id} project={project} />
        ))}
      </div>
    </div>
  )
}
