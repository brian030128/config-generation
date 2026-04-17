# Global Values List Page

**Route:** `/global-values`

Lists all Global Values entries in the system. Global Values are shared across projects and contain reusable key-value data (e.g. database credentials, shared endpoints).

```
┌─────────────────────────────────────────────────────────────────┐
│  Global Values                           [+ New Entry]          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  test_db_values              4 keys   v4   Updated 2h ago       │
│  prod_db_values              4 keys   v7   Updated 1d ago       │
│  shared_secrets              2 keys   v9   Updated 5m ago       │
│  monitoring_endpoints        3 keys   v2   Updated 1w ago       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Elements

### Entry Row
- **Name** — the globally unique identifier
- **Key count** — number of keys in the entry
- **Latest version** number
- **Last updated** timestamp

Clicking a row navigates to the **global-values-detail-page**.

### "+ New Entry" Button
Opens a dialog:
- **Name** — text input (required, must be globally unique)

Creates the entry with an empty payload (v1). Navigates to the detail page to add keys.

### Search
Search bar filters entries by name (substring match).

---

## Pull Requests

A separate section (or tab) lists **Global Values PRs** — PRs that contain only Global Values changes, independent of any project.

```
┌─────────────────────────────────────────────────────────────────┐
│  Global Values Pull Requests                                    │
├─────────────────────────────────────────────────────────────────┤
│  #7  Rotate all staging DB credentials       approved           │
│      alice · 3 changes · Updated 1h ago                         │
│                                                                 │
│  #6  Add monitoring_endpoints entry          open               │
│      bob · 1 change · Updated 3h ago                            │
└─────────────────────────────────────────────────────────────────┘
```

These follow the same lifecycle as project PRs (draft → open → approved → merged / closed) but are not scoped to a project. Approval condition for Global Values PRs is a system-level setting.
