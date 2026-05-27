import type { EnvLevel, TemplateCaps } from "@/lib/role-permissions"
import { cn } from "@/lib/utils"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

const TEMPLATE_CAPS: {
  key: keyof TemplateCaps
  label: string
  description: string
}[] = [
  {
    key: "read_templates",
    label: "Read templates",
    description: "View templates and their version history.",
  },
  {
    key: "write_templates",
    label: "Write templates",
    description: "Create and edit templates (implies read).",
  },
  {
    key: "delete_templates",
    label: "Delete templates",
    description: "Delete templates and their history.",
  },
]

// Toggling a template capability. Write implies read, so enabling write also
// enables read; read stays locked-on while write is enabled.
function applyTemplateToggle(
  t: TemplateCaps,
  key: keyof TemplateCaps,
): TemplateCaps {
  const next = { ...t, [key]: !t[key] }
  if (key === "write_templates" && next.write_templates) {
    next.read_templates = true
  }
  return next
}

// CapabilityEditor renders the shared template + per-environment value access
// controls used by both the per-member permissions page and the role editor.
// It is fully controlled; `disabled` locks every control (e.g. until a role is
// granted project read).
export function CapabilityEditor({
  environments,
  templates,
  onTemplatesChange,
  envLevels,
  onEnvLevelChange,
  disabled = false,
}: {
  environments: { name: string }[]
  templates: TemplateCaps
  onTemplatesChange: (next: TemplateCaps) => void
  envLevels: Record<string, EnvLevel>
  onEnvLevelChange: (env: string, level: EnvLevel) => void
  disabled?: boolean
}) {
  const levelFor = (env: string): EnvLevel => envLevels[env] ?? "none"

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>Templates</CardTitle>
          <CardDescription>
            Project-wide. Templates are shared across every environment.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {TEMPLATE_CAPS.map((cap) => {
            // Read is implied by write: show it checked and locked while write is on.
            const lockedByWrite =
              cap.key === "read_templates" && templates.write_templates
            const checked =
              cap.key === "read_templates"
                ? templates.read_templates || templates.write_templates
                : templates[cap.key]
            const rowDisabled = disabled || lockedByWrite
            return (
              <label
                key={cap.key}
                className={cn(
                  "flex cursor-pointer items-start gap-3 rounded-md border px-3 py-2 transition-colors hover:bg-accent/50",
                  rowDisabled && "opacity-60",
                )}
              >
                <input
                  type="checkbox"
                  className="mt-1 h-4 w-4"
                  checked={checked}
                  disabled={rowDisabled}
                  onChange={() =>
                    onTemplatesChange(applyTemplateToggle(templates, cap.key))
                  }
                />
                <span className="space-y-0.5">
                  <Label className="cursor-pointer">{cap.label}</Label>
                  <span className="block text-xs text-muted-foreground">
                    {cap.description}
                  </span>
                </span>
              </label>
            )
          })}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Environment values</CardTitle>
          <CardDescription>
            Granted per environment, so access can be given to some environments
            and denied to others (e.g. staging but not production).
          </CardDescription>
        </CardHeader>
        <CardContent>
          {environments.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              This project has no environments yet.
            </p>
          ) : (
            <div className="divide-y rounded-md border">
              {environments.map((env) => (
                <div
                  key={env.name}
                  className="flex items-center justify-between gap-4 px-4 py-3"
                >
                  <span className="font-medium">{env.name}</span>
                  <Select
                    value={levelFor(env.name)}
                    disabled={disabled}
                    onValueChange={(v) => onEnvLevelChange(env.name, v as EnvLevel)}
                  >
                    <SelectTrigger className="w-44">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="none">No access</SelectItem>
                      <SelectItem value="read">Read only</SelectItem>
                      <SelectItem value="write">Read &amp; write</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </>
  )
}
