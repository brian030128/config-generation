# Workspace Page

**Route:** `/workspace`

The Workspace is the user's personal editing area. It shows all active draft/open/approved PRs the user has across projects, and is the entry point for making changes to project config. The normal project pages (`/projects/:name`) show the **current live state** (read-only); the Workspace shows the user's **in-progress changes**.

```
┌─────────────────────────────────────────────────────────────────┐
│  Workspace                                                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  billing-service                              [draft]     │  │
│  │  3 changes · created 2h ago                               │  │
│  │  Templates: app.yaml, nginx.conf                          │  │
│  │  Environments: staging                                    │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  auth-service                                 [open]      │  │
│  │  PR #12: "Update auth config" · 1 change · 1d ago        │  │
│  │  Templates: auth.yaml                                     │  │
│  │  1/1 approvals                                            │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                 │
│  Start new workspace: [ Select a project ▾ ]   [Start]          │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 1. Workspace List

Shows all projects where the current user has an active (draft/open/approved) PR. Each card displays:

| Element | Description |
|---|---|
| **Project name** | The project this workspace targets |
| **Status badge** | `draft`, `open`, or `approved` |
| **PR title** | Shown for open/approved PRs (drafts may not have a title yet) |
| **Change count** | Number of changed objects in the PR |
| **Changed objects summary** | Which templates and environments have been modified |
| **Approval progress** | For open/approved PRs: how many approvals vs. required |
| **Timestamp** | When the PR was created or last updated |

Clicking a card navigates to the **workspace project page** for that project.

---

## 2. Start New Workspace

At the bottom of the page, a dropdown lists all projects the user has write access to (excluding projects where they already have an active PR). Selecting a project and clicking **Start** navigates to the workspace project page, ready to begin editing. The draft PR is auto-created on the first save.

---

## 3. Workspace Actions

Each workspace card has a context menu with:
- **Submit for Review** (draft only) — opens a dialog to provide title and description, then transitions the PR from `draft` to `open`.
- **View PR** (open/approved only) — navigates to the PR detail page.
- **Discard** — closes/deletes the draft PR and all its staged changes. Confirmation required.

---

# Workspace Project Page

**Route:** `/workspace/:projectName`

The editing interface for a specific project within the workspace. Structured similarly to the project page but always shows the **draft state** (the latest live version overlaid with any staged changes from the PR).

```
┌─────────────────────────────────────────────────────────────────┐
│  ← Workspace / billing-service                     [draft]      │
│                                                                 │
│  [ Templates ]  [ Environments ]                                │
│                                                [Submit PR]      │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  (tab content — same structure as project-page but editable)    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 1. Templates Tab

Same layout as the project page Templates tab, but every action stages a change rather than
applying it directly. The list is the **overlay view** (`GET /api/workspace/{p}/templates`):
the published base plus this user's staged changes.

- **+ New Template** stages a create (`POST /api/workspace/{p}/templates`); the new template appears in the list with a **"new"** badge.
- **Edit** opens the template editor in workspace mode. Saving stages the change (`PUT /api/workspace/{p}/templates/{t}`), not directly to the latest version.
- **Delete** (row menu) stages a deletion (`DELETE /api/workspace/{p}/templates/{t}`); the template shows a **"deleted"** badge and is struck through until merge.
- Templates with a staged edit show a **"modified"** badge.
- The editor shows the **draft snapshot** if one exists for this template; otherwise, it loads the current live version as a starting point.

---

## 2. Environments Tab

Same layout as the project page Environments tab, overlaid with staged changes
(`GET /api/workspace/{p}/environments`):

- **+ Add Environment** stages an environment create (`POST /api/workspace/{p}/environments`); it appears with a **"new"** badge.
- Clicking an environment navigates to the **workspace environment page** (`/workspace/:projectName/env/:envName`).
- **Delete** (row menu) stages an environment delete (`DELETE /api/workspace/{p}/environments/{e}`), shown struck through with a **"deleted"** badge until merge.
- Environments with pending value changes show a **"modified"** badge.

---

## 3. Workspace Environment Page

**Route:** `/workspace/:projectName/env/:envName`

Same layout as the project-env-page, but:

- The values shown are the **draft snapshot** (overlay, `GET /api/workspace/{p}/envs/{e}/values`) if one exists for this `(template, environment)` pair; otherwise, the current live values are loaded as a starting point.
- **Save** stages the change (`PUT /api/workspace/{p}/envs/{e}/values`). If the PR is `approved`, saving resets all approvals and returns it to `open`.
- **Delete values** stages removal of this env's value set (`DELETE /api/workspace/{p}/envs/{e}/values`).
- No deployment section — deployments operate on the live version, not the draft.

---

## 4. Submit PR

The **"Submit PR"** button (visible in `draft` status) opens a dialog:
- **Title** — text input (required)
- **Description** — text area (optional)

On submit, the PR transitions from `draft` to `open` and becomes visible to reviewers on the Pull Requests page. The user can continue editing in the workspace; further changes go to the now-open PR.

---

## 5. Change Summary

A collapsible panel (or a dedicated tab) showing all changes accumulated in the PR
(`GET /api/workspace/{p}/changes`). Each row labels its **operation**:

```
┌───────────────────────────────────────────────────────────────┐
│  Changes (4)                                                  │
│                                                               │
│  edit    Template: app.yaml                      v8 → draft   │
│  delete  Template: legacy.conf                   v3 → ✕       │
│  create  Environment: eu-prod                    —            │
│  edit    Values: app.yaml / staging              v13 → draft  │
│                                                               │
│  Click a change to view the diff/edit, or remove it (×) to     │
│  unstage and revert that object to the published base.         │
└───────────────────────────────────────────────────────────────┘
```

Each change is clickable — navigates to the relevant editor with the draft snapshot loaded.
Removing a change calls `DELETE /api/workspace/{p}/changes/{changeID}`; discarding the whole
workspace (§3) calls `DELETE /api/workspace/{p}`.
