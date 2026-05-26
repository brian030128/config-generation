# API Endpoint Permission Matrix

## Purpose

This document enumerates every HTTP endpoint exposed by the backend and, for each
one, states:

- **Expected enforcement** — the permission that *should* gate the endpoint, derived
  from [`permission-roles.md`](./permission-roles.md) and [`pr-flow.md`](./pr-flow.md).
- **Current implementation** — what the code in `backend/handlers/router.go` and the
  handler packages actually checks today.
- **Status** — whether the two agree.

It is an audit of the gap between the permission spec and the running code, intended
to drive the remaining permission design work. Routes are defined in
`backend/handlers/router.go`; enforcement primitives live in
`backend/middleware/permissions.go`.

## Design: every project mutation goes through a workspace

This matrix reflects the **target design** in which a project's live state changes
**only** through a workspace → PR → merge flow:

- Every project mutation — creating/deleting an environment, creating/editing/deleting
  a template, creating/editing/deleting an environment's values — is **staged** into the
  caller's **workspace** (their single active draft/open/approved Project PR; one per user
  per project, see [`pr-flow.md`](./pr-flow.md)).
- The **direct-mutation endpoints have been removed.** A merge is the *only* operation that
  appends new versions to or removes objects from live state.
- The `GET /api/projects/{p}/…` endpoints return the **published base**, identical for every
  user and changed only by a merge. The `GET /api/workspace/{p}/…` endpoints return the base
  **overlaid with the caller's own staged changes** — providing per-user isolation (a user
  sees only their own in-progress edits, never anyone else's workspace).
- Because the *stage* endpoints enforce the relevant `write`/`create`/`delete` permission,
  **merge re-checks only PR authorship and the approval condition** — it does not re-check
  write permission per object.

Rows below for removed endpoints are retained for traceability and marked **Removed**.
Global Values keep their existing separate flow and are unchanged by this redesign.

## How enforcement works

- **Route MW** — `RequirePermission(action, resource, projectKey, envKey, nameKey)`
  middleware wraps the route in `router.go`. Key slots are resolved from chi URL
  params at request time. This is the primary, declarative enforcement path.
- **In-handler** — the handler resolves the object's scope first (e.g. looks up a
  role's owning project), then calls `middleware.CheckPermission(...)`, or performs a
  direct author-ID comparison.
- **Composition rules** applied by `satisfies()` in `middleware/permissions.go`:
  - `*` in any key slot is a wildcard, matched per axis.
  - `write` implies `read` on the same resource.
  - `create:env_values(P)` implies `write:project_values(P, *)` (and thus read).
- **Superuser bypass** — users with `users.superuser = true` (seeded admin and
  `OIDC_SUPERUSER_EMAILS`) pass every Route-MW and `CheckPermission` check. They are
  **not** exempt from direct author-ID checks (PR merge/submit).
- **Auth + CSRF** — all `/api/*` routes outside `/api/auth` require a valid session
  (cookie or bearer JWT); cookie-authenticated unsafe methods also require a matching
  `X-CSRF-Token`.

## Status legend

| Symbol | Meaning |
|---|---|
| ✅ | Implementation matches the expected enforcement. |
| ⚠️ | Partial / weaker than spec, or the spec is silent and the endpoint is currently open. |
| ❌ | Expected permission is not enforced at all (or the gating endpoint does not exist). |

---

## Public endpoints (no authentication)

| Method | Path | Notes |
|---|---|---|
| GET | `/healthz` | Liveness probe. |
| GET | `/readyz` | Readiness probe (checks DB ping). |
| GET | `/metrics` | Prometheus scrape. |
| GET | `/api/auth/config` | Frontend feature flags. |
| GET | `/api/auth/me` | Returns current user; `401` if unauthenticated. |
| POST | `/api/auth/session` | Bearer-JWT → session-cookie bridge. |
| POST | `/api/auth/logout` | Requires CSRF token when a session cookie is present. |
| POST | `/api/auth/register` | Available only if `REGISTRATION_ENABLED`. |
| POST | `/api/auth/login` | Available only if `PASSWORD_LOGIN_ENABLED`. |
| GET | `/api/auth/oidc/login` | OIDC redirect start. |
| GET | `/api/auth/oidc/callback` | OIDC redirect return. |

Authentication only proves identity; it grants no resource permissions.

---

## Projects

| Method | Path | Expected enforcement | Current implementation | Status |
|---|---|---|---|---|
| POST | `/api/projects` | `create:project` | Route MW: `create:project` | ✅ |
| GET | `/api/projects` | Authenticated (no scoping) | Authenticated — returns all projects | ✅ |
| GET | `/api/projects/{p}` | `read:project(p)` | None — any authenticated user | ❌ |
| DELETE | `/api/projects/{p}` | `delete:project(p)` | Route MW: `delete:project(p)` | ✅ |

> `GET /api/projects/{p}` requires a `read:project(p)` atom that **does not currently
> exist** in `models/permissions.go` (the only read atoms are `read:project_templates`
> and `read:project_values`). Enforcing this needs a new atom.

Note: project creation auto-creates the `project_admin:<p>` role and assigns it to the
creator (`handlers/projects.go`).

---

## Project Config Templates

All GETs read the **published base** (read-only). Authoring happens in the workspace
(see *Workspace — the project write surface*).

| Method | Path | Expected enforcement | Current implementation | Status |
|---|---|---|---|---|
| GET | `/api/projects/{p}/templates` | `read:project_templates(p)` | Route MW | ✅ |
| ~~POST~~ | ~~`/api/projects/{p}/templates`~~ | **Removed** — stage via `POST /api/workspace/{p}/templates` | Route MW `write` (direct write) | ❌ must remove |
| GET | `/api/projects/{p}/templates/{t}` | `read:project_templates(p)` | Route MW | ✅ |
| GET | `/api/projects/{p}/templates/{t}/variables` | `read:project_templates(p)` | Route MW | ✅ |
| GET | `/api/projects/{p}/templates/{t}/versions` | `read:project_templates(p)` | Route MW | ✅ |
| ~~POST~~ | ~~`/api/projects/{p}/templates/{t}/versions`~~ | **Removed** — stage via `PUT /api/workspace/{p}/templates/{t}` | Route MW `write` (direct write) | ❌ must remove |
| GET | `/api/projects/{p}/templates/{t}/versions/{v}` | `read:project_templates(p)` | Route MW | ✅ |
| GET | `/api/projects/{p}/variables` | `read:project_templates(p)` | Route MW | ✅ |

---

## Project Config Values (per environment)

GETs read the **published base**. Creating/editing/deleting values happens in the workspace
(`PUT`/`DELETE /api/workspace/{p}/envs/{e}/values`).

| Method | Path | Expected enforcement | Current implementation | Status |
|---|---|---|---|---|
| ~~POST~~ | ~~`/api/projects/{p}/values`~~ | **Removed** — stage via `PUT /api/workspace/{p}/envs/{e}/values` | Route MW `create:env_values` (direct write) | ❌ must remove |
| GET | `/api/projects/{p}/envs/{e}/values` | `read:project_values(p, e)` | Route MW | ✅ |
| ~~POST~~ | ~~`/api/projects/{p}/envs/{e}/values/versions`~~ | **Removed** — stage via `PUT /api/workspace/{p}/envs/{e}/values` | Route MW `write` (direct write) | ❌ must remove |
| GET | `/api/projects/{p}/envs/{e}/values/versions/{v}` | `read:project_values(p, e)` | Route MW | ✅ |

---

## Deployments

| Method | Path | Expected enforcement | Current implementation | Status |
|---|---|---|---|---|
| POST | `/api/projects/{p}/envs/{e}/deploy/preview` | `deploy(p, e)` (proposed, spec §6 Q1) | None — any authenticated user | ❌ |
| POST | `/api/projects/{p}/envs/{e}/deploy` | `deploy(p, e)` (proposed, spec §6 Q1) | None — **any authenticated user can deploy to any env, including prod** | ❌ |
| GET | `/api/projects/{p}/envs/{e}/deployments/latest` | `read:project_values(p, e)` (or read-scoped) | None — any authenticated user | ❌ |

`deploy`/`rollback` permissions are explicitly deferred in the spec (§6 Q1) and not
modeled in `models/permissions.go`. The deploy routes carry no permission middleware.
This is the highest-risk gap.

---

## Environments (project-scoped, non-versioned)

These GETs read the **published base**. Creating and deleting environments are now **staged
in the workspace** and applied at merge (`POST`/`DELETE /api/workspace/{p}/environments[/{e}]`),
not performed directly — environments are no longer excluded from PRs.

| Method | Path | Expected enforcement | Current implementation | Status |
|---|---|---|---|---|
| GET | `/api/projects/{p}/environments` | `read:project_templates(p)` (project read) | None — any authenticated user | ⚠️ |
| GET | `/api/projects/{p}/environments/{e}` | `read:project_templates(p)` (project read) | None — any authenticated user | ⚠️ |

Environments are non-versioned. Creation requires `create:env_values(p)`; deletion (which
cascades to the env's value sets) requires `delete:project_values(p, *)` — both staged in the
workspace. See the Workspace section.

---

## Roles — project-scoped

| Method | Path | Expected enforcement | Current implementation | Status |
|---|---|---|---|---|
| POST | `/api/projects/{p}/roles` | `grant(p)` | Route MW: `grant(p)` | ✅ |
| GET | `/api/projects/{p}/roles` | `grant(p)` | Route MW: `grant(p)` | ✅ |

---

## Roles — by ID (scope resolved in-handler)

These resolve the role's owning scope (project or Global Values entry) before checking
permission, in `handlers/roles.go::checkGrantPermission`.

| Method | Path | Expected enforcement | Current implementation | Status |
|---|---|---|---|---|
| PUT | `/api/roles/{id}/permissions` | `grant(scope)`; auto-created roles immutable | In-handler `grant(scope)`; rejects `is_auto_created` | ✅ |
| DELETE | `/api/roles/{id}` | `grant(scope)`; auto-created roles undeletable | In-handler `grant(scope)`; rejects `is_auto_created` | ✅ |
| POST | `/api/roles/{id}/members` | `grant(scope)` | In-handler `grant(scope)` | ✅ |
| DELETE | `/api/roles/{id}/members/{userID}` | `grant(scope)`; cannot remove last project_admin | In-handler `grant(scope)` + last-member guard | ✅ |

**Open items (spec §6 Q4):**
- Enforcement is **grant-anything-in-scope** — a `grant` holder can author a role with
  any permission within the scope, not only permissions they themselves hold. The
  "grant-only-what-you-hold" alternative is not implemented.
- **System-level roles** (neither `project_id` nor `global_values_name` set) are
  rejected: `"system-level role management not yet supported"`. As a result
  `create:project` and Global-Values-entry creation cannot be delegated through roles —
  only superusers control them.

---

## Global Values

| Method | Path | Expected enforcement | Current implementation | Status |
|---|---|---|---|---|
| GET | `/api/global-values` | Scoped to readable entries | None — lists all entries to any authenticated user | ⚠️ (spec undefined) |
| POST | `/api/global-values` | System-level create permission (spec §6 Q2) | None — **any authenticated user can create a GV entry** | ❌ |
| GET | `/api/global-values/{n}` | `read:global_values(n)` | Route MW | ✅ |
| GET | `/api/global-values/{n}/versions` | `read:global_values(n)` | Route MW | ✅ |
| POST | `/api/global-values/{n}/versions` | `write:global_values(n)` | Route MW | ✅ |
| GET | `/api/global-values/{n}/versions/{v}` | `read:global_values(n)` | Route MW | ✅ |
| GET | `/api/global-values/{n}/roles` | `grant(global_values, n)` | Route MW | ✅ |

Notes:
- Creating a GV entry auto-creates `gv_group_admin:<n>` and assigns the creator, but the
  create endpoint itself is ungated.
- There is **no** GV-scoped role *create* endpoint — only listing
  (`ListForGlobalValues`). Custom roles scoped to a GV entry cannot be authored via the
  API today.

---

## Pull Requests

Per spec §8. A **project PR is the workspace** in its review/merge phase — it is created
implicitly by the workspace (first stage), not via `POST /api/pull-requests`; that endpoint
now serves **Global Values PRs only**. These endpoints handle the post-draft lifecycle and
must be gated (today only `merge`/`submit` perform a direct author-ID check).

| Method | Path | Expected enforcement | Current implementation | Status |
|---|---|---|---|---|
| POST | `/api/pull-requests` | GV PRs only: `write:global_values(n)` | None — any authenticated user | ❌ |
| GET | `/api/pull-requests` | Scoped to PRs whose objects the user can read | None — any authenticated user | ❌ |
| GET | `/api/pull-requests/{id}` | `read` on the PR's objects | None — any authenticated user | ❌ |
| POST | `/api/pull-requests/{id}/submit` | PR author | In-handler: author-ID check | ✅ |
| POST | `/api/pull-requests/{id}/approve` | `read` on all objects **and** membership in a role named in the approval condition | None at call time. Approval is recorded for anyone; only approvals from role-holders are *counted* during condition evaluation. No `read` check. | ⚠️ |
| POST | `/api/pull-requests/{id}/withdraw-approval` | Self (withdraw own approval) | In-handler: scoped to caller's own approval via `user_id` | ✅ |
| POST | `/api/pull-requests/{id}/merge` | PR author; status `approved`; not conflicted | In-handler: author-ID check + status check | ✅ |
| POST | `/api/pull-requests/{id}/close` | PR author **or** `grant(scope)` holder | None — any authenticated user can close any PR | ❌ |

Approval-condition evaluation (`checkApprovalConditionMet`, `parseApprovalCondition` in
`handlers/pull_requests.go`) is itself only **partial**: it handles a pure-AND or
pure-OR list of `<count> x <role>` requirements (`isAnd := contains("AND") || len==1`)
and does **not** support the parentheses / precedence grammar described in
`pr-flow.md` §5.2.

---

## Workspace — the project write surface

The workspace is the **only** way to mutate a project's objects. It is the caller's single
active draft/open/approved Project PR (one per user per project). There is **one endpoint per
object type and operation** — each with a single typed request/response body so OpenAPI/Swagger
codegen produces clean, distinct models (no polymorphic `object_type` discriminator). The write
tree mirrors the published read tree under `/api/projects/{p}`.

A staged change requires the same permission the equivalent direct write used to require
(checked at stage time). Merge then re-checks only authorship + approval.

### Lifecycle

| Method | Path | Expected enforcement | Notes |
|---|---|---|---|
| GET | `/api/workspace/{p}` | `read:project_templates(p)` (project read) | The active workspace + a summary of its staged change set; empty if none. Supersedes `GET /api/workspace/{p}/draft`. |
| DELETE | `/api/workspace/{p}` | PR author | Discard the workspace (draft only) — the "until deleted" path. |

### Stage changes (typed per object)

| Method | Path | Expected enforcement | Operation |
|---|---|---|---|
| POST | `/api/workspace/{p}/templates` | `write:project_templates(p)` | Stage a new template. Body `{ template_name, body, commit_message? }`. |
| PUT | `/api/workspace/{p}/templates/{t}` | `write:project_templates(p)` | Stage an edit to a template body. Body `{ body, commit_message? }`. |
| DELETE | `/api/workspace/{p}/templates/{t}` | `delete:project_templates(p)` *(new atom)* | Stage deletion of a template. |
| POST | `/api/workspace/{p}/environments` | `create:env_values(p)` | Stage a new environment. Body `{ name, description? }`. |
| DELETE | `/api/workspace/{p}/environments/{e}` | `delete:project_values(p, *)` | Stage deletion of an environment (cascades to its value sets). |
| PUT | `/api/workspace/{p}/envs/{e}/values` | `create:env_values(p)` if no live set exists, else `write:project_values(p, e)` | Stage create-or-update of an env's value set. Body `{ payload, commit_message? }`. |
| DELETE | `/api/workspace/{p}/envs/{e}/values` | `delete:project_values(p, e)` | Stage deletion of an env's value set. |

### Change set / unstage

Kept separate from the per-object endpoints so "discard my pending edit" is never conflated
with "stage a deletion of the live object."

| Method | Path | Expected enforcement | Notes |
|---|---|---|---|
| GET | `/api/workspace/{p}/changes` | `read:project_templates(p)` (project read) | List the staged changes. |
| DELETE | `/api/workspace/{p}/changes/{changeID}` | same permission as the staged op being dropped | Unstage one pending change; reverts that object to the published base within the workspace. |

### Overlay reads (base + the caller's own staged changes)

| Method | Path | Expected enforcement | Notes |
|---|---|---|---|
| GET | `/api/workspace/{p}/templates` | `read:project_templates(p)` | Created templates appear, deleted ones are hidden, edited ones show the proposed body. |
| GET | `/api/workspace/{p}/templates/{t}` | `read:project_templates(p)` | Proposed body if staged, else the live body. |
| GET | `/api/workspace/{p}/environments` | `read:project_templates(p)` | Overlaid env list. |
| GET | `/api/workspace/{p}/envs/{e}/values` | `read:project_values(p, e)` | Overlaid values for the env. |

---

## Approval-condition modification

`pr-flow.md` §5.5 specifies that a `grant(project)` / `grant(global_values, name)`
holder may update a scope's `approval_condition` at any time.

| Capability | Expected enforcement | Current implementation | Status |
|---|---|---|---|
| Update a project's `approval_condition` | `grant(p)` | No endpoint — only set at project creation | ❌ |
| Update a GV entry's `approval_condition` | `grant(global_values, n)` | No endpoint — only set at entry creation | ❌ |

---

## Summary of work to reach the target design

The redesign establishes a single write path (workspace → PR → merge) for project objects.
The remaining work, in rough priority order:

1. **Remove the direct-mutation endpoints** — `POST /templates`, `POST /templates/{t}/versions`,
   `POST /values`, `POST /envs/{e}/values/versions`. After removal, the only way to change live
   project state is a merge. *(Core of the redesign.)*
2. **Build out the typed workspace write surface** — the per-object `POST`/`PUT`/`DELETE`
   endpoints above, each gated with its `write`/`create`/`delete` atom. This replaces the single
   polymorphic `/stage` endpoint and adds the previously-missing **delete** operations and
   environment create/delete.
3. **Gate the PR lifecycle endpoints** — `create` (GV-only), `close` (author or `grant`),
   `approve` (read + role membership), and scope `list`/`get` to readable objects. Today only
   `merge`/`submit` check authorship.
4. **Add the one new permission atom** — `delete:project_templates(p)` — and assign it to the
   auto-created `project_admin:<p>` role (see `permission-roles.md`). Environment create/delete
   reuse `create:env_values(p)` / `delete:project_values(p, *)`.
5. **Deployments** (`/deploy`, `/deploy/preview`, `/deployments/latest`) — still completely
   ungated; no `deploy` atom exists. Out of scope for this redesign but remains the highest-risk
   gap. *(Unchanged.)*
6. **Global Values** keep their existing separate flow (direct append + per-entry PR);
   `POST /global-values` and the GV/PR list endpoints are still open and unchanged here.
7. **Approval condition** — no endpoint to modify it after creation, and the evaluator does not
   implement the full AND/OR + parentheses grammar. *(Unchanged; tracked separately.)*
