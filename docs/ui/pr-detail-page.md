# Pull Request Detail Page

**Route:** `/projects/:projectName/prs/:prId`

The page for viewing, reviewing, and managing a single pull request.

```
┌─────────────────────────────────────────────────────────────────────┐
│  ← billing-service / Pull Requests                                  │
│                                                                     │
│  PR #42: Update staging DB credentials                   [approved] │
│  alice · opened 2h ago · updated 1h ago                             │
│                                                                     │
│  Description:                                                       │
│  Rotating the staging database credentials to match the new         │
│  test-db cluster provisioned by infra team.                         │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  Changes (2)                                                        │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  Values: app.yaml / staging                                  │  │
│  │  v12 → proposed                                              │  │
│  │  ┌─────────────────────────────────────────────────────────┐  │  │
│  │  │  @@ -3,5 +3,5 @@                                      │  │  │
│  │  │   "env": "staging",                                    │  │  │
│  │  │  -"db_host": "${old_db_values.host}",                  │  │  │
│  │  │  +"db_host": "${test_db_values.host}",                 │  │  │
│  │  │   "db_port": "${test_db_values.port}",                 │  │  │
│  │  └─────────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  Global Values: test_db_values                               │  │
│  │  v3 → proposed                                               │  │
│  │  ┌─────────────────────────────────────────────────────────┐  │  │
│  │  │  @@ -1,4 +1,4 @@                                      │  │  │
│  │  │  -"host": "old-db.internal",                           │  │  │
│  │  │  +"host": "test-db.internal",                          │  │  │
│  │  │   "port": 5432,                                        │  │  │
│  │  └─────────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  Approvals                                                          │
│  Condition: 1 x project_admin                                       │
│  ✓ bob (project_admin) — approved 30m ago                           │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  [Close PR]                           [Approve]  [Merge]            │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 1. Header

- **PR number and title**
- **Status badge** — `draft`, `open`, `approved`, `merged`, `closed`
- **Conflict indicator** — if conflicted, shows a warning: *"This PR has conflicts. Close it and create a new one."*
- **Author**, timestamps

### Editable Fields (author or admin only)
- **Title** — click to edit inline
- **Description** — click to edit inline

---

## 2. Changes Section

Lists all proposed object changes in the PR, each as an expandable diff panel.

### Change Card
Each change shows:
- **Object type and identifier** — e.g. "Values: app.yaml / staging", "Template: nginx.conf", "Global Values: test_db_values"
- **Version transition** — `current_latest → proposed`
- **Diff** — git-style diff between the object's current latest version and the proposed snapshot

### Change Types
| Object Type | Diff Source |
|---|---|
| Template | Text diff of template body |
| Values | JSON diff, pretty-printed with stable key ordering |
| Global Values | JSON diff, same as values |

### Adding Changes (author only, draft/open)
Only the **PR author** can add or update changes. A user may only have **one active (draft/open/approved) PR per project**, so there is no ambiguity about which PR a change belongs to. Other users cannot modify someone else's PR — they can only review and approve. The **"+ Add Change"** button (visible only to the author) offers:
- **Edit a template** — opens the template editor, saves to this PR instead of directly
- **Edit values** — opens the values editor for a (template, env) pair, saves to this PR

Note: Global Values changes are handled in separate **Global Values PRs** (see global-values-listpage). Project PRs cannot include Global Values changes.

If the PR already contains a change for the same object, the new snapshot replaces the previous one.

Adding changes to an `approved` PR **resets all approvals** and returns it to `open`.

---

## 3. Approvals Section

Shows the approval condition and current approval status.

### Approval Condition Display
The project's approval condition is shown as text (e.g. `1 x project_admin AND 1 x project_developer`). Each sub-condition shows whether it's satisfied or not.

### Approval List
Each approval shows:
- **Approver name**
- **Roles held** at the time of approval
- **Timestamp**
- **Withdraw** link (visible to the approver only, while PR is open/approved)

### Approval Progress
A visual indicator showing how many approvals are still needed. For example: `1/1 project_admin ✓, 0/1 project_developer ✗`

---

## 4. Actions

### For Reviewers
- **Approve** — submit an approval. Visible to users who hold a role referenced in the approval condition and have read access to all objects in the PR. Disabled for the PR author (no self-approval).

### For the Author
- **Merge** — visible when status is `approved` and no conflicts. Creates new version rows atomically for all changes. Post-merge, the page shows a "merged" state with a summary.
- **Close PR** — available in `draft`, `open`, or `approved` states. Moves to `closed`.

### For Project Admins
- **Close PR** — admins with `grant(project)` can close any PR.

---

## 5. Status Transitions on This Page

```
[Submit for Review]  draft → open        (author only, on draft PRs)
[Approve]            open → approved     (when condition met, automatic)
[Push Changes]       approved → open     (approvals reset)
[Merge]              approved → merged   (author only)
[Close PR]           any → closed        (author or admin)
```

---

## 6. Conflict State

When a conflict is detected (an object's latest version has changed since the PR was created):
- Banner at the top: *"This PR has conflicts with the latest version. Close this PR and create a new one incorporating the latest changes."*
- All approvals are reset
- Merge button is disabled
- The diff is updated to show the divergence, highlighting which objects have moved forward
