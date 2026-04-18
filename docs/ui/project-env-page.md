# Project Environment Page

**Route:** `/projects/:projectName/env/:envName`

Shows all config values for a specific `(project, environment)` pair, organized by template. This page displays the **current live state** (read-only). To edit values, use the **Workspace** (see workspace-page). Deployments are initiated from this page.

```
┌─────────────────────────────────────────────────────────────────────┐
│  ← billing-service / staging                                        │
│                                                                     │
│  Last deployed: v5 — 3h ago by alice         [Deployment History]   │
│                                              [Review Deployment]    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  Template: [ app.yaml ▾ ]                    Values version: v13    │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  Key              │  Value                        │ Mode    │    │
│  ├───────────────────┼───────────────────────────────┼─────────┤    │
│  │  service_name     │  [billing               ]     │ [Text]  │    │
│  │  env              │  [staging               ]     │ [Text]  │    │
│  │  db_host          │  [test_db_values ▾] [host ▾]  │ [Ref]   │    │
│  │  db_port          │  [test_db_values ▾] [port ▾]  │ [Ref]   │    │
│  │  db_user          │  [test_db_values ▾] [user…▾]  │ [Ref]   │    │
│  │  db_password      │  [test_db_values ▾] [pass…▾]  │ [Ref]   │    │
│  │  feature_flags    │  (nested object — expand ▸)   │         │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                     │
│                                                                     │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 1. Template Selector

Dropdown at the top to switch between templates owned by the project. Changing the selection loads the value set for the new `(template, environment)` pair.

A badge on the dropdown indicates if any values have unsaved changes.

---

## 2. Values Table

Each key from the Project Config Value JSON is displayed as a row.

### Row Structure
| Element | Description |
|---|---|
| **Key** | The JSON key name (read-only, derived from the value structure) |
| **Value input** | Either a text field or a reference selector, depending on mode |
| **Mode toggle** | Switch between `Text` and `Ref` mode |

### Text Mode (default)
A plain text input field. The user types a literal value (string, number, boolean).

### Reference Mode
Two cascading dropdowns:
1. **Global Values group** — lists all available Global Values entries (e.g. `test_db_values`, `shared_secrets`)
2. **Key within group** — lists all keys in the selected Global Values entry (e.g. `host`, `port`, `username`, `password`)

The stored value becomes `${group.key}` (e.g. `${test_db_values.host}`).

### Nested Objects
For nested JSON values (like `feature_flags`), the row is expandable. Clicking the expand arrow reveals the nested keys as indented sub-rows, each with its own text/ref toggle. Nesting is supported to arbitrary depth.

### Adding / Removing Keys
- **"+ Add Key"** button at the bottom of the table to add a new top-level key
- Each row has a delete icon to remove a key
- These structural changes are part of the value version — they create a new snapshot on save

---

## 3. Editing

This page is read-only. Values are displayed but cannot be edited directly. To make changes, navigate to the Workspace via the sidebar or the banner shown when the user has an active draft/PR for this project.

Editing happens on the **Workspace Environment Page** (`/workspace/:projectName/env/:envName`), which has the same layout but is editable and saves to the user's draft PR.

---

## 4. Deployment Section

### Header Info
- **Last deployed version** — shows the deployment ID, timestamp, and author
- **Current latest version** vs deployed version indicator — highlights if there are undeployed changes

### "Review Deployment" Button
Navigates to the **deployment-review-page** for this `(project, environment)`. Pins the current latest versions of all templates, values, and referenced global values.

### "Deployment History" Button
Opens a panel or page listing all past deployments for this `(project, environment)`:

```
┌───────────────────────────────────────────────────────────┐
│  Deployment History — billing-service / staging            │
├───────────────────────────────────────────────────────────┤
│  #5  succeeded   3h ago   alice   "Update DB creds"       │
│  #4  succeeded   2d ago   bob     "Add feature flags"     │
│  #3  rolled_back 5d ago   carol   "Nginx update"          │
│  #2  succeeded   1w ago   alice   "Initial staging"       │
│  #1  failed      1w ago   alice   "Initial staging"       │
├───────────────────────────────────────────────────────────┤
│  Click a deployment to inspect its pinned versions and    │
│  rendered outputs. Succeeded deployments show a           │
│  [Rollback to this] button.                               │
└───────────────────────────────────────────────────────────┘
```

Each succeeded deployment shows a **"Rollback to this"** button, which opens the deployment-review-page in rollback mode (pinned to the versions from that deployment instead of latest).

---

## 5. Validation

- No empty values are allowed. The save button is disabled if any value field is blank.
- Reference mode validates that the selected Global Values group and key exist.
- If a referenced Global Values entry is deleted or a key removed, the row shows a warning indicator.
