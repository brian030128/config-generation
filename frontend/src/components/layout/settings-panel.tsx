import { useEffect, useState } from "react"
import { Check, Monitor, Moon, Settings, Sun } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { cn } from "@/lib/utils"

type BackgroundMode = "light" | "dark" | "system"
type SettingsSection = "background"

const backgroundOptions: Array<{
  value: BackgroundMode
  label: string
  description: string
  icon: typeof Sun
}> = [
  {
    value: "light",
    label: "Light",
    description: "Use the light interface.",
    icon: Sun,
  },
  {
    value: "dark",
    label: "Dark",
    description: "Use the dark interface.",
    icon: Moon,
  },
  {
    value: "system",
    label: "System",
    description: "Follow your device setting.",
    icon: Monitor,
  },
]

function getStoredBackgroundMode(): BackgroundMode {
  const stored = localStorage.getItem("background_mode")
  if (stored === "light" || stored === "dark" || stored === "system") {
    return stored
  }
  return "light"
}

function applyBackgroundMode(mode: BackgroundMode) {
  const prefersDark = globalThis.matchMedia("(prefers-color-scheme: dark)").matches
  const shouldUseDark = mode === "dark" || (mode === "system" && prefersDark)

  document.documentElement.classList.toggle("dark", shouldUseDark)
}

export function SettingsPanel() {
  const [section, setSection] = useState<SettingsSection>("background")
  const [backgroundMode, setBackgroundMode] = useState<BackgroundMode>(() =>
    getStoredBackgroundMode(),
  )

  useEffect(() => {
    applyBackgroundMode(backgroundMode)
    localStorage.setItem("background_mode", backgroundMode)
  }, [backgroundMode])

  useEffect(() => {
    if (backgroundMode !== "system") return

    const media = globalThis.matchMedia("(prefers-color-scheme: dark)")
    const syncSystemMode = () => applyBackgroundMode("system")

    media.addEventListener("change", syncSystemMode)
    return () => media.removeEventListener("change", syncSystemMode)
  }, [backgroundMode])

  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button
          className="fixed bottom-5 right-5 z-40 h-11 rounded-lg bg-primary px-3 text-primary-foreground shadow-lg hover:bg-primary/90"
          aria-label="Open settings"
        >
          <Settings className="h-4 w-4" />
        </Button>
      </DialogTrigger>
      <DialogContent className="gap-0 p-0 sm:max-w-2xl">
        <DialogHeader className="border-b px-6 py-4">
          <DialogTitle>Settings</DialogTitle>
          <DialogDescription>
            Adjust workspace preferences for this browser.
          </DialogDescription>
        </DialogHeader>

        <div className="grid min-h-80 grid-cols-[13rem_1fr]">
          <aside className="border-r bg-muted/30 p-3">
            <button
              type="button"
              onClick={() => setSection("background")}
              className={cn(
                "flex w-full items-center justify-between rounded-md px-3 py-2 text-left text-sm transition-colors",
                section === "background"
                  ? "bg-background font-medium text-foreground shadow-xs"
                  : "text-muted-foreground hover:bg-background/70 hover:text-foreground",
              )}
            >
              Background
              {section === "background" && <Check className="h-3.5 w-3.5" />}
            </button>
          </aside>

          <section className="space-y-4 p-6">
            <div className="space-y-1">
              <h3 className="text-base font-semibold">Background</h3>
              <p className="text-sm text-muted-foreground">
                Choose how the interface background should appear.
              </p>
            </div>

            <div className="grid gap-3">
              {backgroundOptions.map((option) => {
                const Icon = option.icon
                const selected = backgroundMode === option.value

                return (
                  <button
                    key={option.value}
                    type="button"
                    onClick={() => setBackgroundMode(option.value)}
                    className={cn(
                      "flex items-center justify-between rounded-lg border p-4 text-left transition-colors hover:bg-accent/50",
                      selected && "border-foreground/30 bg-accent/50",
                    )}
                  >
                    <span className="flex items-center gap-3">
                      <span className="flex size-9 items-center justify-center rounded-md border bg-background text-muted-foreground">
                        <Icon className="h-4 w-4" />
                      </span>
                      <span>
                        <span className="block text-sm font-medium">
                          {option.label}
                        </span>
                        <span className="block text-xs text-muted-foreground">
                          {option.description}
                        </span>
                      </span>
                    </span>
                    {selected && <Check className="h-4 w-4" />}
                  </button>
                )
              })}
            </div>
          </section>
        </div>
      </DialogContent>
    </Dialog>
  )
}
