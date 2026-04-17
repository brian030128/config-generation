# Implementation Tasks

## Phase 1 — Database & Core Models

The foundation. Everything else depends on this.

### 1.1 Database Setup
- Set up PostgreSQL database and migrations tooling
- Create migration for `users` and `environments` tables
- Create migration for `projects` table (including `approval_condition`)

### 1.2 Versioned Object Tables
- Create migration for `project_config_templates` (with full-copy versioning)
- Create migration for `project_config_values` (with JSONB payload)
- Create migration for `global_values` (with flat-JSON constraint)
- Implement version ID auto-increment logic (scoped per object, `SELECT MAX + 1 FOR UPDATE`)

### 1.3 Core CRUD — Projects & Environments
- API: create, read, list, delete projects
- API: create, read, list environments
- On project create: auto-create `project_admin:<name>` role and assign to creator

### 1.4 Core CRUD — Templates
- API: create template (first version)
- API: append new version to existing template
- API: get latest version of a template
- API: get specific version of a template
- API: list templates for a project
- API: list version history for a template

### 1.5 Core CRUD — Project Config Values
- API: create value set for a `(template, env)` pair (v1)
- API: append new version to existing value set
- API: get latest version for `(template, env)`
- API: get specific version
- API: list all value sets for a `(project, env)` pair
- Validate: no empty values on write

### 1.6 Core CRUD — Global Values
- API: create named Global Values entry
- API: append new version
- API: get latest version by name
- API: get specific version
- API: list all entries
- Validate: payload must be flat (no nested objects/arrays, scalars only)

---

## Phase 2 — Config Generation Engine

Depends on: Phase 1

### 2.1 Reference Resolution
- Implement `${name.key}` parser to extract references from a JSON value tree
- Walk the JSON at any depth, replacing reference strings with resolved scalars
- Error on unknown Global Values name (`ErrUnknownGlobalValues`)
- Error on unknown key within a Global Values entry (`ErrUnknownKey`)

### 2.2 Template Rendering
- Parse Go template from template body
- Execute template with resolved JSON as dot context
- Return rendered text
- Error on parse failure (`ErrTemplateParse`)
- Error on execution failure (`ErrTemplateExec`)

### 2.3 Generation Orchestration
- Given `(project, environment)`: generate all templates
- Per template: lookup template → lookup values → resolve → render
- Error if no values exist for a `(template, env)` pair (`ErrMissingValues`)
- Return list of `(template_name, rendered_output)` pairs

---

## Phase 3 — Permissions & Roles

Depends on: Phase 1

### 3.1 Roles & Permissions Tables
- Create migration for `roles`, `role_permissions`, `user_roles`
- API: create role (with permission atoms)
- API: edit role permissions (custom roles only)
- API: delete role (custom only, with member revocation)
- API: assign user to role
- API: remove user from role
- Constraint: cannot remove last `project_admin` member

### 3.2 Permission Checking
- Implement permission resolver: given a user, compute effective permissions (union of all role permissions)
- Implement wildcard matching on key slots (`*` matches anything)
- Implement `write implies read` logic
- Implement `create:env_values implies write:project_values(project, *)` logic

### 3.3 Permission Middleware
- Add permission checks to all existing APIs:
  - `create:project` for project creation
  - `write:project_templates(project)` for template writes
  - `write:project_values(project, env)` for value writes
  - `create:env_values(project)` for new value set creation
  - `delete:project(project)`, `delete:project_values(project, env)` for deletes
  - `write:global_values(name)` for global values writes
  - `grant(project)` for role/approval-condition management
  - Read permissions for all read endpoints

---

## Phase 4 — Deployments

Depends on: Phase 2 (generation engine), Phase 1

### 4.1 Deployment Tables
- Create migration for `deployments`, `deployment_entries`, `deployment_entry_global_refs`

### 4.2 Deployment Creation
- API: create deployment for `(project, environment)`
- Pin candidate version set (latest versions of all templates, values, referenced globals)
- Render all templates using pinned versions
- Write deployment record with status `pending`, all pinned version IDs, and rendered outputs
- Atomic write of all entries

### 4.3 Deployment Status
- API: update deployment status (`succeeded` / `failed`)
- API: get last successful deployment for `(project, environment)`
- API: list deployment history for `(project, environment)`

### 4.4 Diff Computation
- Given a candidate version set and a last-deployed version set: compute diffs for each input (template body, values JSON, global values JSON)
- Compute rendered output diff (last deployed rendered output vs new rendered output)
- Detect which Global Values entries are added/removed between deployments

### 4.5 Rollback
- API: create rollback deployment from a prior successful deployment
- Reuse the prior deployment's pinned version IDs
- Set `rolled_back_from` field on new deployment record
- Render using the prior versions and capture output

---

## Phase 5 — Pull Requests

Depends on: Phase 1, Phase 3 (permissions)

### 5.1 PR Tables
- Create migration for `pull_requests`, `pr_changes`, `pr_approvals`
- Add unique constraint: one active (draft/open/approved) PR per user per project

### 5.2 Project PR Lifecycle
- API: create PR (validate author has write permission on all included objects)
- API: update PR title/description
- API: submit for review (draft → open)
- API: close PR (author or `grant(project)` holder)
- API: add/update change to PR (replace snapshot if same object already in PR)
- On change added to approved PR: reset all approvals, status → open

### 5.3 Global Values PR Lifecycle
- Same lifecycle as project PRs but not scoped to a project
- One active Global Values PR per user
- Can bundle changes across multiple Global Values entries
- Cannot include templates or project config values

### 5.4 Conflict Detection
- Record `base_version_id` per change when added to PR
- Periodically (and at merge time): compare base version against current latest
- If diverged: mark `is_conflicted`, reset approvals, return to open, disable merge

### 5.5 Approval System
- API: approve PR (validate: not the author, holds required role, has read access to all objects)
- API: withdraw approval
- Parse approval condition expression (`<count> x <role>`, AND, OR, parentheses)
- Evaluate condition against current approvals (user's roles at approval time)
- Auto-transition open → approved when condition met
- Re-evaluate on: new approval, withdrawal, condition change, approval reset

### 5.6 Merge
- API: merge PR (validate: author only, status = approved, not conflicted)
- For each change: append new version row atomically
- New version `commit_message` = "Merged from PR #N"
- All version rows written in single transaction
- PR status → merged
- Merging does NOT trigger deployment

---

## Phase 6 — Frontend: Core Pages

Depends on: Phase 1, Phase 3

### 6.1 App Shell
- Sidebar navigation (Projects, Global Values, User Menu)
- Top bar with breadcrumb trail
- Authentication/session management

### 6.2 Project List Page
- Fetch and display project cards (name, description, template count, env count, last updated)
- Search bar (client-side filter)
- Sort by last updated
- "New Project" dialog (name, description, approval condition)
- Permission-gated: only show "New Project" if user has `create:project`

### 6.3 Project Page — Templates Tab
- List templates (name, version, last updated)
- Template editor: code editor with syntax highlighting, commit message, save
- Version history sidebar (click to view read-only snapshot)
- "New Template" dialog
- Delete template (with confirmation)
- "Save to PR" flow

### 6.4 Project Page — Environments Tab
- List environments with config coverage and last deployed timestamp
- "Add Environment" dialog (select from global environments)
- Delete environment (with confirmation)
- Click to navigate to project-env-page

### 6.5 Project Env Page — Values Editor
- Template selector dropdown
- Values table with text input / reference mode toggle per row
- Reference mode: two cascading dropdowns (Global Values group → key)
- Nested object expansion
- Add/remove keys
- "Save" button (direct version append)
- "Save to PR" button (create new or add to existing active PR)
- Validation: no empty values

### 6.6 Global Values List Page
- List all entries (name, key count, version, last updated)
- Search bar
- "New Entry" dialog
- Global Values PR list section

### 6.7 Global Values Detail Page
- Key-value editor table
- Sensitive value masking (password/secret/token/key fields)
- "Save" / "Save to PR" buttons
- "Referenced by" section (which projects use this entry)
- Version history list (click to view read-only)

---

## Phase 7 — Frontend: Deployment Review

Depends on: Phase 4, Phase 6

### 7.1 Deployment Review Page
- Pin candidate version set on page load
- Template selector tabs with change badges
- "Newer versions available" banner + Refresh button

### 7.2 Left Pane — Input Diffs
- Template diff section (collapsible, git-style text diff)
- Values diff section (collapsible, pretty-printed JSON diff)
- Global Values diff sections (one per referenced entry, with added/removed tags)
- Collapse unchanged sections by default

### 7.3 Right Pane — Rendered Output Diff
- Git-style diff of rendered config (old deployed vs new candidate)
- Show error if rendering fails, disable Deploy button
- "All added" mode if no prior deployment

### 7.4 Diff Rendering
- Unified/split view toggle (persisted per user)
- Line-level additions (green) / deletions (red)
- Syntax highlighting
- Summary line (`+N −M lines`)

### 7.5 Deploy / Rollback Actions
- Deploy button with confirmation dialog and commit message input
- Status feedback (pending → succeeded/failed)
- Deployment history panel
- "Rollback to this" button on succeeded deployments (opens review page in rollback mode)

---

## Phase 8 — Frontend: Pull Requests

Depends on: Phase 5, Phase 6

### 8.1 Project Page — PRs Tab
- List PRs (number, title, status badge, conflict indicator, author, change/approval counts)
- Status filter dropdown (default: open + approved)
- "New PR" button

### 8.2 PR Detail Page — Changes
- List change cards with diffs (template text diff, values/global JSON diff)
- "+ Add Change" button (author only, draft/open)
- Snapshot replacement when same object updated

### 8.3 PR Detail Page — Approvals
- Display approval condition and progress
- Approve button (validate role, not author)
- Withdraw approval
- Auto-transition on condition met

### 8.4 PR Detail Page — Actions
- Submit for review (draft → open)
- Merge (approved + no conflict → merged)
- Close PR

### 8.5 Global Values PR List & Detail
- PR list on global values list page
- Same detail page layout but without project scope

---

## Phase 9 — Frontend: Role Management

Depends on: Phase 3, Phase 6

### 9.1 Role Management Page
- List roles as expandable cards (permissions + members)
- "Create Role" dialog with permission atom builder (action, resource, key fields)
- Edit permissions on custom roles
- Delete custom roles (with confirmation)

### 9.2 Member Management
- "Add Member" with user search
- "Remove Member" with confirmation
- Constraint: prevent removing last `project_admin` member

### 9.3 Project Settings
- Edit description and approval condition
- Link to role management
- Delete project

---

## Dependency Graph

```
Phase 1 (DB & Models)
  ├── Phase 2 (Generation Engine)
  │     └── Phase 4 (Deployments) ──── Phase 7 (UI: Deployment Review)
  ├── Phase 3 (Permissions & Roles)
  │     ├── Phase 5 (Pull Requests) ── Phase 8 (UI: Pull Requests)
  │     └── Phase 9 (UI: Roles)
  └── Phase 6 (UI: Core Pages)
        ├── Phase 7
        ├── Phase 8
        └── Phase 9
```

Phases 2 and 3 can be built in parallel after Phase 1.
Phases 4 and 5 can be built in parallel.
Phases 6 through 9 depend on their backend counterparts but frontend scaffolding (Phase 6) can start early.
