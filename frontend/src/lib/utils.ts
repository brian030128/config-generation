import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"
import { AxiosError } from "axios"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// getApiErrorMessage extracts the backend's `error` message from an axios error,
// falling back to the generic error message.
export function getApiErrorMessage(err: unknown): string {
  if (err instanceof AxiosError && err.response?.data?.error) {
    return err.response.data.error as string
  }
  if (err instanceof Error) return err.message
  return "Something went wrong"
}

export function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSecs = Math.floor(diffMs / 1000)
  const diffMins = Math.floor(diffSecs / 60)
  const diffHours = Math.floor(diffMins / 60)
  const diffDays = Math.floor(diffHours / 24)

  if (diffSecs < 60) return "just now"
  if (diffMins < 60) return `${diffMins}m ago`
  if (diffHours < 24) return `${diffHours}h ago`
  if (diffDays < 7) return `${diffDays}d ago`
  if (diffDays < 30) return `${Math.floor(diffDays / 7)}w ago`
  return date.toLocaleDateString()
}

// safeString coerces an unknown value to a string display, avoiding the
// useless "[object Object]" output that `String(x)` produces for plain
// objects. Objects/arrays are serialized as JSON.
export function safeString(v: unknown): string {
  if (v == null) return ""
  if (typeof v === "string") return v
  if (
    typeof v === "number" ||
    typeof v === "boolean" ||
    typeof v === "bigint"
  ) {
    return String(v)
  }
  if (typeof v === "object") {
    try {
      return JSON.stringify(v)
    } catch {
      return ""
    }
  }
  // functions / symbols — not expected in our JSON payloads
  return ""
}
