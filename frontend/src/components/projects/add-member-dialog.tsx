import { useEffect, useState } from "react"
import { toast } from "sonner"
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
import { UserPlus } from "lucide-react"
import { useAddProjectMember } from "@/hooks/use-projects"
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

export function AddMemberDialog({
  projectName,
  existingMemberIds,
}: Readonly<{
  projectName: string
  existingMemberIds: number[]
}>) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState("")
  const debouncedSearch = useDebouncedValue(search, 250)
  const addMember = useAddProjectMember(projectName)

  const { data, isLoading, error } = useUserSearch(debouncedSearch, open)
  const existing = new Set(existingMemberIds)
  const results = (data?.items ?? []).filter((u) => !existing.has(u.id))

  function reset() {
    setSearch("")
  }

  function handleAdd(user: User) {
    addMember.mutate(user.id, {
      onSuccess: () => {
        setOpen(false)
        reset()
        toast.success(`Added ${user.display_name || user.username}`)
      },
      onError: (err) => {
        toast.error("Failed to add member", {
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
        if (!next) reset()
      }}
    >
      <DialogTrigger asChild>
        <Button size="sm">
          <UserPlus className="mr-2 h-4 w-4" />
          Add Member
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add Member</DialogTitle>
          <DialogDescription>
            Grant a user read access to this project. They will see the project
            but not its templates or values until granted further permissions.
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
                disabled={addMember.isPending}
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
