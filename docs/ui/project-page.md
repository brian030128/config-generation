# Project Page

**Route:** `/projects/:projectName`

The main hub for a project. Three tabbed sections: **Templates**, **Environments**, and **Pull Requests**.

```
┌─────────────────────────────────────────────────────────────────┐
│  ← Projects / billing-service                                   │
│  Payment processing service configs                             │
│                                                                 │
│  [ Templates ]  [ Environments ]  [ Pull Requests ]             │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  (tab content below)                                            │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 1. Templates Tab

Lists all config templates owned by this project.

```
┌─────────────────────────────────────────────────────────────────┐
│  Templates                                [+ New Template]      │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  app.yaml                     v8    Updated 2h ago     [Edit]   │
│  database.conf                v3    Updated 1d ago     [Edit]   │
│  nginx.conf                   v12   Updated 5m ago     [Edit]   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Template Row
- **Template name**
- **Current version** (latest version number)
- **Last updated** timestamp
- **Edit button** — opens the template editor (inline or full-page)

### "+ New Template" Button
Visible to users with `write:project_templates(project)`.
Opens a dialog:
- **Template name** — text input (required, unique within project)
- **Body** — code editor (the Go template text)
- **Commit message** — text input (optional)

### Template Editor
Clicking Edit opens the template body in a code editor with syntax highlighting. Save creates a new version (full-copy).

- **Code editor** with the template body
- **Commit message** input
- **Save** button — appends a new version
- **Version history** sidebar — lists all versions with author and timestamp; clicking a version loads that snapshot (read-only for non-latest)

Requires `write:project_templates(project)` to save.

### Delete Template
Available via a menu on each template row. Requires `write:project_templates(project)`. Confirmation dialog warns that all versions and associated values will be affected.

---

## 2. Environments Tab

Lists all environments that have value sets defined for this project.

```
┌─────────────────────────────────────────────────────────────────┐
│  Environments                          [+ Add Environment]      │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  dev         3/3 templates configured    Last deployed 1h ago   │
│  staging     3/3 templates configured    Last deployed 3h ago   │
│  prod        3/3 templates configured    Last deployed 2d ago   │
│  eu-prod     1/3 templates configured    Never deployed         │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Environment Row
- **Environment name**
- **Configuration coverage** — how many templates have value sets for this environment vs total templates
- **Last deployed** — timestamp of the last successful deployment, or "Never deployed"

Clicking a row navigates to the **project-env-page** for that `(project, environment)`.

### "+ Add Environment" Button
Visible to users with `create:env_values(project)`.
Opens a dialog:
- **Environment** — dropdown of all global environments
- Once selected, the user is navigated to the **project-env-page** to fill in values for each template (values are required — no empty values allowed).

### Delete Environment
Available via a menu on each row. Requires `delete:project_values(project, env)`. Removes all value sets for this `(project, environment)` pair. Confirmation dialog required.

---

## 3. Pull Requests Tab

Lists all PRs for this project.

```
┌─────────────────────────────────────────────────────────────────┐
│  Pull Requests                              [+ New PR]          │
│  [ Open ▾ ]                                                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  #42  Update staging DB credentials          approved           │
│       alice · 2 changes · 1 approval · Updated 1h ago           │
│                                                                 │
│  #41  Add eu-prod environment values         open               │
│       bob · 5 changes · 0 approvals · Updated 3h ago            │
│                                                                 │
│  #39  Refactor nginx template                open  ⚠ conflict   │
│       carol · 1 change · 0 approvals · Updated 1d ago           │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### PR Row
- **PR number and title**
- **Status badge** — `draft`, `open`, `approved`, `merged`, `closed`
- **Conflict indicator** if conflicted
- **Author**, change count, approval count, last updated

Clicking a row navigates to the **pr-detail-page**.

### Status Filter
Dropdown to filter by status. Default: `open` (shows `open` + `approved`).

### "+ New PR" Button
Opens the PR creation flow (see pr-detail-page for details).

---

## 4. Project Settings (admin)

Accessible via a gear icon in the page header. Visible to users with `grant(project)`.

- **Edit description**
- **Edit approval condition** — text input showing the current expression
- **Manage roles** — link to the role management page scoped to this project
- **Delete project** — requires `delete:project(project)`, confirmation dialog
