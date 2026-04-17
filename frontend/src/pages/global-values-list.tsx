import { useState } from "react"
import { Link } from "react-router-dom"
import { useGlobalValues } from "@/hooks/use-global-values"
import { CreateGlobalValuesDialog } from "@/components/global-values/create-gv-dialog"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { formatRelativeTime } from "@/lib/utils"
import { Search, ChevronRight } from "lucide-react"

export default function GlobalValuesListPage() {
  const { data, isLoading, error } = useGlobalValues()
  const [search, setSearch] = useState("")

  const filtered =
    data?.items.filter((gv) =>
      gv.name.toLowerCase().includes(search.toLowerCase()),
    ) ?? []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Global Values</h1>
        <CreateGlobalValuesDialog />
      </div>

      <div className="relative max-w-sm">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          className="pl-9"
          placeholder="Search entries..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {isLoading && (
        <p className="text-muted-foreground">Loading global values...</p>
      )}

      {error && (
        <p className="text-destructive">
          Failed to load global values: {(error as Error).message}
        </p>
      )}

      {!isLoading && filtered.length === 0 && (
        <p className="text-muted-foreground">
          {search
            ? "No entries match your search."
            : "No global values yet. Create one to get started."}
        </p>
      )}

      <div className="space-y-2">
        {filtered.map((gv) => (
          <Link
            key={gv.name}
            to={`/global-values/${gv.name}`}
            className="flex items-center justify-between rounded-lg border px-4 py-3 transition-colors hover:bg-accent/50"
          >
            <div className="flex items-center gap-4">
              <span className="font-mono text-sm font-medium">{gv.name}</span>
              <span className="text-xs text-muted-foreground">
                {Object.keys(gv.payload).length} keys
              </span>
              <Badge variant="outline" className="text-xs">
                v{gv.version_id}
              </Badge>
              <span className="text-xs text-muted-foreground">
                {formatRelativeTime(gv.created_at)}
              </span>
            </div>
            <ChevronRight className="h-4 w-4 text-muted-foreground" />
          </Link>
        ))}
      </div>
    </div>
  )
}
