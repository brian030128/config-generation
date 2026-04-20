import { useMemo } from "react"
import { diffLines } from "diff"
import { cn } from "@/lib/utils"

interface DiffViewerProps {
  oldText: string
  newText: string
  className?: string
}

interface DiffLine {
  type: "added" | "removed" | "context"
  content: string
  oldLineNo?: number
  newLineNo?: number
}

export function DiffViewer({ oldText, newText, className }: DiffViewerProps) {
  const { lines, additions, deletions } = useMemo(() => {
    const parts = diffLines(oldText, newText)
    const result: DiffLine[] = []
    let oldLine = 1
    let newLine = 1
    let adds = 0
    let dels = 0

    for (const part of parts) {
      const partLines = part.value.split("\n")
      if (partLines[partLines.length - 1] === "") partLines.pop()

      for (const content of partLines) {
        if (part.added) {
          result.push({ type: "added", content, newLineNo: newLine })
          newLine++
          adds++
        } else if (part.removed) {
          result.push({ type: "removed", content, oldLineNo: oldLine })
          oldLine++
          dels++
        } else {
          result.push({ type: "context", content, oldLineNo: oldLine, newLineNo: newLine })
          oldLine++
          newLine++
        }
      }
    }

    return { lines: result, additions: adds, deletions: dels }
  }, [oldText, newText])

  if (additions === 0 && deletions === 0) {
    return (
      <div className={cn("text-sm text-muted-foreground italic p-3", className)}>
        No changes
      </div>
    )
  }

  return (
    <div className={cn("text-sm font-mono", className)}>
      <div className="px-3 py-1.5 text-xs text-muted-foreground border-b bg-muted/30 flex items-center gap-3">
        <span className="text-green-400">+{additions}</span>
        <span className="text-red-400">-{deletions}</span>
      </div>
      <pre className="overflow-x-auto p-0 m-0">
        {lines.map((line, i) => (
          <div
            key={i}
            className={cn(
              "flex leading-5 whitespace-pre",
              line.type === "added" && "bg-green-500/15",
              line.type === "removed" && "bg-red-500/15",
            )}
          >
            <span className="inline-block w-10 shrink-0 text-right pr-1 text-muted-foreground/50 select-none border-r border-border/50">
              {line.oldLineNo ?? ""}
            </span>
            <span className="inline-block w-10 shrink-0 text-right pr-1 text-muted-foreground/50 select-none border-r border-border/50">
              {line.newLineNo ?? ""}
            </span>
            <span
              className={cn(
                "inline-block w-4 shrink-0 text-center select-none",
                line.type === "added" && "text-green-400",
                line.type === "removed" && "text-red-400",
              )}
            >
              {line.type === "added" ? "+" : line.type === "removed" ? "-" : " "}
            </span>
            <span
              className={cn(
                "flex-1 pl-1",
                line.type === "added" && "text-green-400",
                line.type === "removed" && "text-red-400",
              )}
            >
              {line.content}
            </span>
          </div>
        ))}
      </pre>
    </div>
  )
}

interface TextViewerProps {
  text: string
  className?: string
}

export function TextViewer({ text, className }: TextViewerProps) {
  const lines = text.split("\n")
  if (lines[lines.length - 1] === "") lines.pop()

  return (
    <div className={cn("text-sm font-mono", className)}>
      <pre className="overflow-x-auto p-0 m-0">
        {lines.map((line, i) => (
          <div key={i} className="flex leading-5 whitespace-pre">
            <span className="inline-block w-10 shrink-0 text-right pr-1 text-muted-foreground/50 select-none border-r border-border/50">
              {i + 1}
            </span>
            <span className="pl-3 flex-1">{line}</span>
          </div>
        ))}
      </pre>
    </div>
  )
}
