# Project Page

**Route:** `/projects/:projectName`

The main hub for a project, showing the **current live state** (latest merged versions). This page is read-only — to make changes, use the **Workspace** (see workspace-page). If the current user has an active draft/PR for this project, a banner links to their workspace.

Two tabbed sections: **Templates** and **Environments**.

```
┌─────────────────────────────────────────────────────────────────┐
│  ← Projects / billing-service                                   │
│  Payment processing service configs                             │
│                                                                 │
│  [ Templates ]  [ Environments ]                                │
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
│  app.yaml            v8    Updated 2h ago   [Edit in Workspace]  │
│  database.conf       v3    Updated 1d ago   [Edit in Workspace]  │
│  nginx.conf          v12   Updated 5m ago   [Edit in Workspace]  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Template Row
- **Template name**
- **Current version** (latest version number)
- **Last updated** timestamp
- **Edit in Workspace** button — navigates to the Workspace for this project

### "+ New Template" Button
Visible to users with `write:project_templates(project)`. Because the project page is read-only,
this navigates to the **Workspace** for the project, where the new template is staged into the
caller's draft PR (`POST /api/workspace/{p}/templates`). It is not created directly.

### Template Viewer
Clicking a template opens it in a read-only code viewer showing the current live version.

- **Code viewer** with the template body (read-only)
- **Version history** sidebar — lists all versions with author and timestamp; clicking a version loads that snapshot

To edit, use the Workspace.

### Delete Template
Available via a menu on each template row. Requires `delete:project_templates(project)`. Like every other mutation, deletion is **staged in the Workspace** (`DELETE /api/workspace/{p}/templates/{t}`) and only takes effect when the PR is merged. Confirmation dialog warns that all versions and associated values will be affected.

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
Visible to users with `create:env_values(project)`. Navigates to the **Workspace**, where the
new environment is staged (`POST /api/workspace/{p}/environments`) and the user fills in values
for each template (staged as value changes). Nothing is created directly; it all merges together.

### Delete Environment
Available via a menu on each row. Requires `delete:project_values(project, *)`. Staged in the
Workspace (`DELETE /api/workspace/{p}/environments/{e}`) and applied at merge — removes the
environment and all its value sets. Confirmation dialog required.

---

## 3. Project Settings (admin)

Accessible via a gear icon in the page header. Visible to users with `grant(project)`.

- **Edit description**
- **Edit approval condition** — text input showing the current expression
- **Manage roles** — link to the role management page scoped to this project
- **Delete project** — requires `delete:project(project)`, confirmation dialog
