// Client-side validation for the PR approval-condition DSL, mirroring the
// backend parser/matcher. The condition is a set of "N x role_name" requirements
// joined by AND/OR. It is valid when it parses and every referenced role exists
// in scope (or is a built-in auto-created role). This blocks creating an
// un-approvable (dangling-role) condition.

export interface ApprovalRequirement {
  count: number
  role: string
}

export interface ApprovalValidation {
  valid: boolean
  errors: string[]
  requirements: ApprovalRequirement[]
}

// Quantifiers are bounded to keep the regex linear (no catastrophic
// backtracking) for any input length.
const REQ_RE = /(\d{1,9})[ \t]{0,8}x[ \t]{0,8}(\S{1,256})/g

export function parseApprovalCondition(condition: string): ApprovalRequirement[] {
  const reqs: ApprovalRequirement[] = []
  for (const m of condition.matchAll(REQ_RE)) {
    reqs.push({ count: Number.parseInt(m[1], 10), role: m[2] })
  }
  return reqs
}

// A single, complete requirement: "N x role" (role is any run of non-space chars).
// Bounded quantifiers prevent ReDoS.
const FULL_REQ_RE = /^\d{1,9}[ \t]{0,8}x[ \t]{0,8}\S{1,256}$/i

// isWellFormed checks the whole condition is a sequence of complete "N x role"
// requirements joined by AND/OR — with no dangling token (e.g. a trailing
// "1 x " with no role, or a stray "AND"). The extraction regex alone ignores
// such fragments, so this guards against silently accepting them.
export function isWellFormed(condition: string): boolean {
  const trimmed = condition.trim()
  if (!trimmed) return false
  const parts = trimmed.split(/\s+(?:AND|OR)\s+/i)
  return parts.every((p) => FULL_REQ_RE.test(p.trim()))
}

// A requirement role is satisfiable if a known/built-in role name equals it or
// has it as a "name:" prefix (e.g. "project_admin" matches "project_admin:billing").
function satisfiable(role: string, known: string[]): boolean {
  return known.some((n) => n === role || n.startsWith(role + ":"))
}

export function validateApprovalCondition(
  condition: string,
  knownRoleNames: string[],
  builtins: string[] = [],
): ApprovalValidation {
  const errors: string[] = []
  const trimmed = condition.trim()
  const requirements = parseApprovalCondition(trimmed)

  if (!trimmed) {
    errors.push("Approval condition is required.")
    return { valid: false, errors, requirements }
  }
  if (requirements.length === 0 || !isWellFormed(trimmed)) {
    errors.push(
      'Could not parse — use e.g. "1 x project_admin AND 1 x release_manager".',
    )
    return { valid: false, errors, requirements }
  }

  const known = [...knownRoleNames, ...builtins]
  for (const req of requirements) {
    if (req.count < 1) {
      errors.push(`"${req.role}": count must be at least 1.`)
    } else if (!satisfiable(req.role, known)) {
      errors.push(`Role "${req.role}" does not exist yet — create it first.`)
    }
  }

  return { valid: errors.length === 0, errors, requirements }
}
