# Pull Request Flow — Specification

## 1. Overview

This document specifies the pull request (PR) workflow for proposing, reviewing, and merging changes to versioned objects in the config generation system. It covers PR creation, scoping, approval rules, and the merge action.

A PR is the mechanism by which changes to Project Config Templates, Project Config Values, and Global Values are reviewed and accepted before they become the new latest versions. PRs sit between authoring (editing a draft) and deployment (pushing rendered configs to a target environment).

---

## 2. PR Scope

There are two distinct PR types:

### 2.1 Project PRs

A Project PR is the merge-time view of a user's **workspace** for a single project (§4.1). It
may contain any mix of the following changes within that project, each tagged with an
**operation** (`create`, `update`, or `delete`):

- **Project Config Templates** — create, edit, or delete a template.
- **Project Config Values** — create, edit, or delete an environment's value set.
- **Environments** — create or delete an environment (a non-versioned, structural change).

All changes within a PR are treated as an atomic unit — they are merged together or not at all. This allows authors to coordinate related changes (e.g. adding a new template key and updating the values that reference it) in a single reviewable unit.

A Project PR **cannot** include Global Values changes.

### 2.2 Global Values PRs

A Global Values PR proposes changes to a single **Global Values entry**. It is scoped to that entry and governed by the entry's own approval condition and roles (see §5).

A Global Values PR **cannot** include project templates or project config values.

### 2.3 Constraints

- A PR is scoped to either a **single project** (Project PR) or a **single Global Values entry** (Global Values PR).
- Each changed *versioned* object (template, values) carries a full-copy snapshot of its new content (consistent with the versioning strategy in the Version Control spec). The diff shown to reviewers is computed between the object's current latest version and the proposed snapshot. A **delete** change carries no payload — it records the intent to remove the object at merge.
- **Environments** are non-versioned; a PR may include their **create**/**delete** as structural changes applied at merge (env create/delete is what stands up or tears down the value sets that hang off the environment). The **Project** itself is still never modified through a PR.

---

## 3. PR Lifecycle

A PR moves through the following statuses:

```
draft ──> open ──> approved ──> merged
            │          │
            │    open ◄─┘  (approval invalidated by new changes or conflict)
            ▼
          closed
```

| Status | Meaning |
|---|---|
| `draft` | Author is still composing changes. Not visible to reviewers. |
| `open` | Submitted for review. Reviewers can inspect diffs and approve. |
| `approved` | The approval condition (see section 5) is satisfied. The author may now merge. |
| `merged` | All changes have been committed as new versions of their respective objects. Terminal state. |
| `closed` | Abandoned without merging. Terminal state. |

### 3.1 Status Transitions

- **draft -> open**: The author submits the PR for review by providing a title and optional description. For Project PRs, the draft is auto-created when the user first saves a change; the user explicitly submits it when ready.
- **open -> approved**: The approval condition is met (see section 5). This transition is automatic — no manual action is required.
- **approved -> open**: The author pushes additional changes to the PR after it was approved. All existing approvals are invalidated, and the PR returns to `open` for re-review.
- **approved -> merged**: The author clicks the **Merge** button (see section 6).
- **open -> closed** / **draft -> closed**: The author (or a project admin) closes the PR without merging.

---

## 4. Creating and Editing PRs

### 4.1 Project PRs — the Workspace is the only write path

There are **no direct-apply endpoints** for a project's objects. Every modification —
creating/deleting an environment, creating/editing/deleting a template, creating/editing/deleting
an environment's values — is **staged** into the author's **workspace** and only becomes part of
the live project state when the PR is merged. The workspace **is** the user's active draft/open/
approved Project PR.

- Each staged change is one of the typed workspace operations (see the API spec's *Workspace —
  the project write surface*): a template `create`/`edit`/`delete`, an environment
  `create`/`delete`, or a values `create-or-update`/`delete`. Each change records its **operation**
  alongside the proposed snapshot (deletes carry no snapshot).
- The first staged change **auto-creates** the draft PR if none exists.
- A user may have at most **one active** (draft/open/approved) **PR per project** at a time. All edits within the project go to that single PR.
- The user can continue making edits across multiple templates, environments, and value sets; each change is added to (or updates/replaces an existing change for the same object in) the same PR. A single staged change can be **unstaged**, reverting that object to the published base within the workspace.
- **Isolation:** while editing, the workspace shows the published base **overlaid with the
  author's own staged changes** only. Other users' workspaces are invisible, and the published
  state every user reads is unaffected until a merge.
- When the user is satisfied, they submit the draft for review (transition from `draft` to `open`), at which point they provide a title and description.
- After submitting, the user may **continue editing** the PR in the same way — further changes are added to the open/approved PR.
- If the PR is already `approved` when the user pushes new changes, **all approvals are reset** and the PR returns to `open` for re-review (see §3.1).

This approach avoids branching and merge strategies — a limitation that may be revisited in future updates.

### 4.2 Global Values PRs

Global Values PRs are created explicitly when the user clicks "Save to PR" on the Global Values detail page (see the Global Values Detail Page spec). They are scoped to a single entry.

A user may have at most **one active** (draft/open/approved) **Global Values PR per entry**.

### 4.3 PR Data Model

A PR contains:

- `pr_id` — unique identifier.
- `project` — the owning project (Project PRs only; null for Global Values PRs).
- `global_values_name` — the target Global Values entry (Global Values PRs only; null for Project PRs).
- `author` — the user who created the PR.
- `title` — short summary of the change (set when submitting draft for review).
- `description` — optional free-text body.
- `status` — current lifecycle status.
- `changes` — list of staged changes. Each carries an **object type** (template / values / environment), an **operation** (`create` / `update` / `delete`), the target identity, a `base_version_id` (for conflict detection on versioned objects), and a proposed snapshot for create/update (none for delete). See section 2.
- `created_at`, `updated_at` — timestamps.

---

## 5. Approval Condition

### 5.1 Definition

Each project and each Global Values entry has a configurable **approval condition** that governs when a PR is considered approved. For Project PRs, the project's condition applies; for Global Values PRs, the entry's condition applies. The condition is a boolean expression composed of **role requirements** joined by `AND` and `OR` operators, with grouping via parentheses.

A **role requirement** has the form:

```
<count> x <role>
```

where `<count>` is a positive integer and `<role>` is any role defined within the project's scope (e.g. `project_admin`, `release_manager`, `project_developer`).

A role requirement is satisfied when at least `<count>` distinct users holding that role have approved the PR.

### 5.2 Operators

- **OR**: At least one of the operands must be satisfied.
- **AND**: All operands must be satisfied.
- **Parentheses**: Group sub-expressions to control precedence. `AND` binds tighter than `OR` when parentheses are absent.

### 5.3 Examples

| Condition | Meaning |
|---|---|
| `1 x release_manager` | One release manager must approve. |
| `2 x project_developer` | Two distinct project developers must approve. |
| `1 x project_admin AND 1 x project_developer` | One project admin and one project developer must both approve. |
| `1 x release_manager OR (1 x project_admin AND 1 x project_developer)` | Either a release manager approves alone, or both a project admin and a project developer approve. |

### 5.4 Initial Condition

When a project is created, the creator specifies the approval condition. If none is provided, the default is `1 x project_admin`.

When a Global Values entry is created, the creator specifies the approval condition. If none is provided, the default is:

```
1 x gv_group_admin
```

### 5.5 Modifying the Condition

A user with `grant(<project>)` permission may update a project's approval condition at any time. A user with `grant(global_values, <name>)` may update a Global Values entry's approval condition. Changes take effect immediately for all `open` PRs — their approval status is re-evaluated against the new condition.

### 5.6 Approval Mechanics

- A user approves a PR by submitting an **approval review**. The system records which role(s) the approver holds at the time of approval.
- If a user holds multiple roles (e.g. both `project_admin` and `project_developer`), a single approval counts toward all roles they hold.
- An approver may withdraw their approval at any time while the PR is `open` or `approved`, which triggers re-evaluation of the condition.
- When the author pushes new changes to an `approved` PR, **all approvals are reset** and the PR returns to `open`. Reviewers must re-approve the updated content.

---

## 6. Merge

### 6.1 Who Can Merge

Only the **PR author** may merge. The merge button is available when:

1. The PR status is `approved`.
2. The PR is not conflicted (see section 7).

### 6.2 Merge Action

When the author clicks **Merge**, the staged changes are applied to live state according to
their operation:

1. **Create / update** of a versioned object (template, values): a new version row is appended (per the full-copy versioning strategy). The new version's `created_by` is the PR author, and the `commit_message` references the PR (e.g. `"Merged from PR #42"`).
2. **Delete** of a template or value set: the logical object and its entire version history are removed.
3. **Environment create / delete**: the environment row is created, or deleted along with its value sets (a structural change — no version row).
4. All of the above are applied **atomically** within a single transaction — either every change succeeds or none do.
5. The PR status moves to `merged` with a timestamp.
6. The resulting state becomes the **latest** for each affected object, available for the next deployment review.

Because permission was already enforced when each change was **staged** (§8), merge re-checks
only PR authorship, `approved` status, and conflict-freedom — it does not re-check write
permission per object.

### 6.3 Post-Merge

Merging a PR does **not** trigger a deployment. The merged changes are now the latest versions, but deployment remains a separate, explicit action via the Deployment Review GUI (see the Version Control & Deployment spec).

### 6.4 Post-Merge: Global Values PRs

When a Global Values PR is merged, all other unmerged (draft, open, or approved) PRs targeting the **same Global Values entry** are automatically closed. The system sets their status to `closed` with a message referencing the merged PR (e.g. `"Auto-closed: superseded by PR #N"`).

Rationale: Global Values use full-copy versioning. Once a PR is merged, every other PR's `base_version_id` for that entry is stale. Rather than marking them conflicted and requiring manual close, the system auto-closes them. Authors may create new PRs incorporating the latest version.

---

## 7. Conflicts

A conflict occurs when an object included in the PR has had its latest version updated (by another merged PR or direct edit) since the PR was created.

### 7.1 Detection

Each PR records a **base version** per included object — the latest version at the time the object was added to the PR. At merge time (and periodically while the PR is open), the system compares each base version against the current latest version. If any have diverged, the PR is marked **conflicted**.

### 7.2 Resolution

Conflicts cannot be automatically resolved. A conflicted PR is **invalidated**: all approvals are reset, the PR returns to `open`, and the merge button is disabled. The author must **close** the conflicted PR and create a new PR that incorporates the current latest versions.

---

## 8. Permissions

PR operations interact with the existing permission model as follows:

### 8.1 Project PRs

Permission is enforced when a change is **staged** into the workspace (per object/operation),
not at merge time. The merge action itself requires only authorship + `approved` status.

| Action (staged in the workspace) | Required Permission |
|---|---|
| Stage a template create or edit | `write:project_templates(project)` |
| Stage a template delete | `delete:project_templates(project)` *(new atom)* |
| Stage an environment create | `create:env_values(project)` |
| Stage an environment delete | `delete:project_values(project, *)` |
| Stage a value set create (first set for an env) | `create:env_values(project)` |
| Stage a value set edit | `write:project_values(project, env)` |
| Stage a value set delete | `delete:project_values(project, env)` |
| Unstage a change / discard the workspace | the same permission as the staged op (author only for discard) |
| Approve a PR | `read` permission on all objects in the PR, plus membership in a role referenced by the project's approval condition |
| Merge a PR | Must be the PR author; PR must be `approved`; not conflicted |
| Close a PR | PR author or `grant(project)` holder |
| Modify the approval condition | `grant(project)` |

### 8.2 Global Values PRs

| Action | Required Permission |
|---|---|
| Create a Global Values PR | `write:global_values(name)` for the target entry |
| Approve a Global Values PR | `read:global_values(name)` plus membership in a role referenced by the entry's approval condition |
| Merge a Global Values PR | Must be the PR author; PR must be `approved` |
| Close a Global Values PR | PR author or `grant(global_values, name)` holder |
| Modify the approval condition | `grant(global_values, name)` |
