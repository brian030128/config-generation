// Shared classname helpers for KV / text diff rendering. Centralising these
// removes the nested ternaries that were flagged by SonarCloud (S3358) in
// create-pr.tsx, pr-detail-page.tsx and deploy-page.tsx.

export interface KvChangeFlags {
  isAdded: boolean
  isRemoved: boolean
  isChanged: boolean
}

// Row background based on add/remove/change state.
export function kvRowBgClass({ isAdded, isRemoved, isChanged }: KvChangeFlags): string {
  if (isAdded) return "bg-green-50 dark:bg-green-950/20"
  if (isRemoved) return "bg-red-50 dark:bg-red-950/20"
  if (isChanged) return "bg-yellow-50 dark:bg-yellow-950/20"
  return ""
}

// Text colour for the "proposed" cell.
export function kvProposedTextClass({ isAdded, isChanged }: { isAdded: boolean; isChanged: boolean }): string {
  if (isAdded) return "text-green-600"
  if (isChanged) return "text-yellow-600"
  return ""
}

// For unified-line text diffs (template diffs).
export type DiffLineType = "added" | "removed" | "context" | "changed"

export function diffLineBgClass(type: DiffLineType): string {
  if (type === "added") return "bg-green-50 dark:bg-green-950/20"
  if (type === "removed") return "bg-red-50 dark:bg-red-950/20"
  return ""
}

export function diffLinePrefix(type: DiffLineType): string {
  if (type === "added") return "+ "
  if (type === "removed") return "- "
  return "  "
}

export function diffLineTextClass(type: DiffLineType): string {
  if (type === "added") return "text-green-700 dark:text-green-400"
  if (type === "removed") return "text-red-700 dark:text-red-400"
  return ""
}
