import { useState } from "react"
import { Link } from "react-router-dom"
import { useGlobalValues } from "@/hooks/use-global-values"
import { CreateGlobalValuesDialog } from "@/components/global-values/create-gv-dialog"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { formatRelativeTime } from "@/lib/utils"
import { ChevronRight, Database, Search, SearchX } from "lucide-react"

export default function GlobalValuesListPage() {
  const { data, isLoading, error } = useGlobalValues()
  const [search, setSearch] = useState("")

  const filtered =
    data?.items.filter((gv) =>
      gv.name.toLowerCase().includes(search.toLowerCase()),
    ) ?? []

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold">Global Values</h1>
          <p className="text-sm text-muted-foreground">
            Manage shared configuration entries used across projects.
          </p>
        </div>
        <CreateGlobalValuesDialog />
      </div>

      <div className="flex flex-col gap-3 rounded-lg border bg-card p-3 shadow-xs sm:flex-row sm:items-center sm:justify-between">
        <div className="relative w-full sm:max-w-md">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="pl-9"
            placeholder="Search entries..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <p className="text-sm text-muted-foreground">
          {data?.count ?? 0} {data?.count === 1 ? "entry" : "entries"}
        </p>
      </div>

      {isLoading && (
        <p className="text-muted-foreground">Loading global values...</p>
      )}

      {error && (
        <p className="text-destructive">
          Failed to load global values: {(error as Error).message}
        </p>
      )}

      {!isLoading && filtered.length === 0 ? (
        <div className="flex min-h-72 items-center justify-center rounded-lg border border-dashed bg-card/40 p-8 text-center">
          <div className="mx-auto flex max-w-sm flex-col items-center gap-3">
            <div className="flex size-11 items-center justify-center rounded-lg border bg-background text-muted-foreground">
              {search ? (
                <SearchX className="h-5 w-5" />
              ) : (
                <Database className="h-5 w-5" />
              )}
            </div>
            <div className="space-y-1">
              <h2 className="text-base font-semibold">
                {search ? "No matching entries" : "No global values yet"}
              </h2>
              <p className="text-sm text-muted-foreground">
                {search
                  ? "Try a different search term or clear the search field."
                  : "Create a shared entry for values reused across projects."}
              </p>
            </div>
            {search && (
              <Button variant="outline" onClick={() => setSearch("")}>
                Clear search
              </Button>
            )}
          </div>
        </div>
      ) : (
        <div className="space-y-2">
          {filtered.map((gv) => (
            <Link
              key={gv.name}
              to={`/global-values/${gv.name}`}
              className="flex items-center justify-between rounded-lg border bg-card px-4 py-3 transition-colors hover:bg-accent/50"
            >
              <div className="flex flex-wrap items-center gap-4">
                <span className="font-mono text-sm font-medium">{gv.name}</span>
                <Badge variant="outline" className="text-xs">
                  group
                </Badge>
                <span className="text-xs text-muted-foreground">
                  Updated {formatRelativeTime(gv.created_at)}
                </span>
              </div>
              <ChevronRight className="h-4 w-4 text-muted-foreground" />
            </Link>
          ))}
        </div>
      )}
    </div>
  )
}
