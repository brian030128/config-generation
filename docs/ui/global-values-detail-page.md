# Global Values Detail Page

**Route:** `/global-values/:name`

Editor for a single Global Values entry. Displays the flat key-value pairs and allows editing.

```
┌─────────────────────────────────────────────────────────────────┐
│  ← Global Values / test_db_values                               │
│  Version: v4 · Updated 2h ago by alice                          │
│                                                                 │
│  Referenced by: billing-service (3), auth-service (1)           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────────┬────────────────────────────────────────┐  │
│  │  Key             │  Value                                 │  │
│  ├──────────────────┼────────────────────────────────────────┤  │
│  │  host            │  [test-db.internal              ]      │  │
│  │  port            │  [5432                          ]      │  │
│  │  username        │  [app                           ]      │  │
│  │  password        │  [••••••••                      ]  👁   │  │
│  └──────────────────┴────────────────────────────────────────┘  │
│                                                  [+ Add Key]    │
│                                                                 │
│  Commit message: [___________________________]                  │
│                                              [Save]  [Save to PR] │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│  Version History                                                │
│  v4  alice  2h ago   "Rotate credentials"                       │
│  v3  bob    3d ago   "Update host to test-db"                   │
│  v2  alice  1w ago   "Add password field"                       │
│  v1  alice  2w ago   "Initial creation"                         │
└─────────────────────────────────────────────────────────────────┘
```

---

## 1. Key-Value Editor

A simple table of key-value pairs. Global Values are always flat (no nesting).

### Row Structure
| Element | Description |
|---|---|
| **Key** | Text input for the key name |
| **Value** | Text input for the scalar value (string, number, or boolean) |
| **Delete** | Icon button to remove the key |

### "+ Add Key" Button
Appends a new empty row at the bottom of the table.

### Sensitive Values
Values that appear to be secrets (keys containing `password`, `secret`, `token`, `key`) are masked by default. A visibility toggle reveals the value.

---

## 2. Save Actions

### "Save" Button
Appends a new version with the current key-value state. Requires `write:global_values(name)`.

### "Save to PR" Button
Stages the change into a **Global Values PR** — a separate PR type that is not scoped to any project. A user may only have **one active Global Values PR** at a time.

- If the user **has no active Global Values PR** → opens a dialog to create one (title + description), then stages the change.
- If the user **already has an active Global Values PR** → the change is added to it directly.

Global Values PRs can bundle changes across multiple Global Values entries but cannot include project templates or project config values. If the PR already contains a change for this same entry, the new snapshot replaces the previous one.

---

## 3. "Referenced by" Section

Shows which projects reference this Global Values entry in their config values. Each reference is a link to the relevant project-env-page. Helps the author understand the blast radius of a change.

---

## 4. Version History

Lists all versions of this entry, most recent first.

| Column | Description |
|---|---|
| **Version** | Version number |
| **Author** | Who created this version |
| **Timestamp** | When it was created |
| **Commit message** | Optional note from the author |

Clicking a version loads that snapshot into the editor in **read-only mode**, allowing inspection of historical values. Only the latest version is editable.
