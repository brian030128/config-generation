import { useEffect, useState } from "react"
import { toast } from "sonner"
import { UserPlus } from "lucide-react"
import type { User } from "@/api/types"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { useAddEnvAdmin } from "@/hooks/use-environments"
import { useUserSearch } from "@/hooks/use-users"
import { getApiErrorMessage } from "@/lib/utils"

function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), delayMs)
    return () => clearTimeout(id)
  }, [value, delayMs])
  return debounced
}

export function AddEnvAdminDialog({
  projectName,
  envName,
  existingAdminIds,
}: {
  projectName: string
  envName: string
  existingAdminIds: number[]
}) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState("")
  const debouncedSearch = useDebouncedValue(search, 250)
  const addAdmin = useAddEnvAdmin(projectName, envName)

  const { data, isLoading, error } = useUserSearch(debouncedSearch, open)
  const existing = new Set(existingAdminIds)
  const results = (data?.items ?? []).filter((u) => !existing.has(u.id))

  function handleAdd(user: User) {
    addAdmin.mutate(user.id, {
      onSuccess: () => {
        setOpen(false)
        setSearch("")
        toast.success(`Added ${user.display_name || user.username} as env admin`)
      },
      onError: (err) => {
        toast.error("Failed to add env admin", {
          description: getApiErrorMessage(err),
        })
      },
    })
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (!next) setSearch("")
      }}
    >
      <DialogTrigger asChild>
        <Button size="sm">
          <UserPlus className="mr-2 h-4 w-4" />
          Add Admin
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add Environment Admin</DialogTitle>
          <DialogDescription>
            Env admins can manage this environment's values, delete it, and grant
            env-admin to other users.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <Input
            autoFocus
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by name or username"
          />

          <div className="min-h-40 max-h-72 overflow-y-auto rounded-md border">
            {isLoading && (
              <p className="p-3 text-sm text-muted-foreground">Searching...</p>
            )}

            {error && (
              <p className="p-3 text-sm text-destructive">
                {getApiErrorMessage(error)}
              </p>
            )}

            {!isLoading && !error && results.length === 0 && (
              <p className="p-3 text-sm text-muted-foreground">
                {search.trim()
                  ? "No matching users."
                  : "No users available to add."}
              </p>
            )}

            {results.map((user) => (
              <button
                key={user.id}
                type="button"
                disabled={addAdmin.isPending}
                onClick={() => handleAdd(user)}
                className="flex w-full items-center gap-3 px-3 py-2 text-left transition-colors hover:bg-accent/50 disabled:opacity-50"
              >
                <span className="font-medium">
                  {user.display_name || user.username}
                </span>
                <span className="text-xs text-muted-foreground">
                  @{user.username}
                </span>
              </button>
            ))}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
