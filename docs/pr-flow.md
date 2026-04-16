# Pull Request Flow — Specification

## 1. Overview

This document specifies the pull request (PR) workflow for proposing, reviewing, and merging changes to versioned objects in the config generation system. It covers PR creation, scoping, approval rules, and the merge action.

A PR is the mechanism by which changes to Project Config Templates, Project Config Values, and Global Values are reviewed and accepted before they become the new latest versions. PRs sit between authoring (editing a draft) and deployment (pushing rendered configs to a target environment).

---

## 2. PR Scope

A single PR may contain changes across **multiple versioned objects**, specifically:

- One or more **Project Config Templates** (within the same project).
- One or more **Project Config Values** (across any environments within the same project).
- One or more **Global Values** entries.

All changes within a PR are treated as an atomic unit — they are merged together or not at all. This allows authors to coordinate related changes (e.g. adding a new template key and updating the values that reference it) in a single reviewable unit.

### 2.1 Constraints

- A PR is scoped to a **single project**. Changes to Global Values may be included because project values reference them, but the PR belongs to the owning project.
- Each changed object in the PR contains a full-copy snapshot of its new content (consistent with the versioning strategy in the Version Control spec). The diff shown to reviewers is computed between the object's current latest version and the proposed snapshot.
- A PR cannot include changes to non-versioned objects (Projects, Environments).

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

- **draft -> open**: The author submits the PR for review.
- **open -> approved**: The approval condition is met (see section 5). This transition is automatic — no manual action is required.
- **approved -> open**: The author pushes additional changes to the PR after it was approved. All existing approvals are invalidated, and the PR returns to `open` for re-review.
- **approved -> merged**: The author clicks the **Merge** button (see section 6).
- **open -> closed** / **draft -> closed**: The author (or a project admin) closes the PR without merging.

---

## 4. Creating a PR

Any user with write permission on at least one of the objects being changed may create a PR. The system validates at creation time that the author holds the necessary write (or create) permissions for every object included in the PR.

A PR contains:

- `pr_id` — unique identifier.
- `project` — the owning project.
- `author` — the user who created the PR.
- `title` — short summary of the change.
- `description` — optional free-text body.
- `status` — current lifecycle status.
- `changes` — list of proposed object snapshots (see section 2).
- `created_at`, `updated_at` — timestamps.

---

## 5. Approval Condition

### 5.1 Definition

Each project has a configurable **approval condition** that governs when a PR is considered approved. The condition is a boolean expression composed of **role requirements** joined by `AND` and `OR` operators, with grouping via parentheses.

A **role requirement** has the form:

```
<count> x <role>
```

where `<count>` is a positive integer and `<role>` is any role defined within the project's scope (e.g. `project_admin`, `release_manager`, `project_developer`).

A role requirement is satisfied when at least `<count>` distinct users holding that role have approved the PR. The approving users must be different from the PR author — self-approval does not count.

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

When a project is created, the creator specifies the approval condition. If none is provided, the default is:

```
1 x project_admin
```

### 5.5 Modifying the Condition

A user with `grant(<project>)` permission may update the approval condition at any time. Changes take effect immediately for all `open` PRs — their approval status is re-evaluated against the new condition.

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

When the author clicks **Merge**:

1. For each changed object in the PR, a new version row is appended (per the full-copy versioning strategy). The new version's `created_by` is the PR author, and the `commit_message` references the PR (e.g. `"Merged from PR #42"`).
2. All version rows are written atomically — either all succeed or none do.
3. The PR status moves to `merged` with a timestamp.
4. The new versions become the **latest** versions of their respective objects, available for the next deployment review.

### 6.3 Post-Merge

Merging a PR does **not** trigger a deployment. The merged changes are now the latest versions, but deployment remains a separate, explicit action via the Deployment Review GUI (see the Version Control & Deployment spec).

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

| Action | Required Permission |
|---|---|
| Create a PR with template changes | `write:project_templates(project)` |
| Create a PR with value changes | `write:project_values(project, env)` for each affected env |
| Create a PR with global value changes | `write:global_values(name)` for each affected entry |
| Approve a PR | `read` permission on all objects in the PR, plus membership in a role referenced by the approval condition |
| Merge a PR | Must be the PR author; PR must be `approved` |
| Close a PR | PR author or `grant(project)` holder |
| Modify the approval condition | `grant(project)` |
