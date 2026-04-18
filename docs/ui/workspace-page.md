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

Same layout as the project page Templates tab, but:

- **Edit** opens the template editor in workspace mode. Saves stage the change in the draft PR (not directly to the latest version).
- Templates with pending changes in the PR show a **"modified"** badge.
- The editor shows the **draft snapshot** if one exists for this template; otherwise, it loads the current live version as a starting point.

---

## 2. Environments Tab

Same layout as the project page Environments tab, but:

- Clicking an environment navigates to the **workspace environment page** (`/workspace/:projectName/env/:envName`).
- Environments with pending value changes show a **"modified"** badge.

---

## 3. Workspace Environment Page

**Route:** `/workspace/:projectName/env/:envName`

Same layout as the project-env-page, but:

- The values shown are the **draft snapshot** from the PR if one exists for this `(template, environment)` pair; otherwise, the current live values are loaded as a starting point.
- **Save** stages the change in the draft PR. If the PR is `approved`, saving resets all approvals and returns it to `open`.
- No deployment section — deployments operate on the live version, not the draft.

---

## 4. Submit PR

The **"Submit PR"** button (visible in `draft` status) opens a dialog:
- **Title** — text input (required)
- **Description** — text area (optional)

On submit, the PR transitions from `draft` to `open` and becomes visible to reviewers on the Pull Requests page. The user can continue editing in the workspace; further changes go to the now-open PR.

---

## 5. Change Summary

A collapsible panel (or a dedicated tab) showing all changes accumulated in the PR:

```
┌───────────────────────────────────────────────────────────────┐
│  Changes (3)                                                  │
│                                                               │
│  Template: app.yaml                              v8 → draft   │
│  Template: nginx.conf                            v12 → draft  │
│  Values: app.yaml / staging                      v13 → draft  │
│                                                               │
│  Click a change to view the diff or edit.                     │
└───────────────────────────────────────────────────────────────┘
```

Each change is clickable — navigates to the relevant editor with the draft snapshot loaded.
