# Deploy Page

**Route:** `/deploy`
**Sidebar:** Top-level nav item (Rocket icon)

Standalone page for deploying rendered config files. Users select a project, environment, and pin specific versions of all inputs. The page shows a split-pane diff review and, on deploy, saves the deployment record and downloads rendered configs as a zip file.

```
┌─────────────────────────────────────────────────────────────────────────┐
│  🚀 Deploy                                                             │
│                                                                         │
│  Project: [ billing-service ▾ ]   Environment: [ staging ▾ ]            │
│                                                   [Refresh Preview]     │
│                                                                         │
│  ▸ Version Pinning (3 templates, values v13, 2 global value groups)     │
│                                                                         │
│  Template: [ app.yaml ]  [database.conf]  [nginx.conf●]                │
│                                                   ● = has changes       │
├────────────────────────────────┬────────────────────────────────────────┤
│  INPUTS                        │  RENDERED OUTPUT                       │
│                                │                                        │
│  ▸ Template: app.yaml v8       │                                        │
│  ┌──────────────────────────┐  │   service: billing                     │
│  │  @@ -3,4 +3,5 @@        │  │   environment: staging                 │
│  │   service: {{ .svc }}    │  │  -  host: old-db.internal              │
│  │  +environment: {{ .env }}│  │  +  host: test-db.internal             │
│  │   database:              │  │     port: 5432                         │
│  └──────────────────────────┘  │                                        │
│                                │                                        │
│  ▸ Values (v13)                │                                        │
│  ┌──────────────────────────┐  │                                        │
│  │   "env": "staging",     │  │                                        │
│  │  +"db_host": "${test_db… │  │                                        │
│  └──────────────────────────┘  │                                        │
│                                │                                        │
│  ▸ Global: test_db_values (v4) │                                        │
│  ┌──────────────────────────┐  │                                        │
│  │  "host": "test-db…"     │  │                                        │
│  └──────────────────────────┘  │                                        │
│                                │                                        │
│  ▾ Global: shared_secrets (v9) │                                        │
│    [unchanged]                 │                                        │
│                                │                                        │
├────────────────────────────────┴────────────────────────────────────────┤
│  Commit message: [___________________________________]                  │
│                                                          [Deploy]       │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 1. Project & Environment Selection

Two cascading dropdowns at the top of the page:
- **Project** — lists all projects
- **Environment** — lists environments for the selected project (disabled until project is chosen)

Once both are selected, the page loads version defaults and triggers a preview render.

---

## 2. Version Pinning

A collapsible "Version Pinning" panel lets users control which exact versions are used:

- **Template versions** — per-template dropdown listing all available versions
- **Values version** — the values version for the selected (project, environment)
- **Global value group versions** — per referenced global values entry, dropdown of all versions

### Default Versions

- If a previous successful deployment exists for this (project, environment): version selectors default to the versions from that deployment.
- If no prior deployment exists: version selectors default to the latest versions of all inputs.

### Refresh

Clicking **Refresh Preview** re-renders all templates with the currently pinned versions and updates both panes.

---

## 3. Template Selector

Horizontal tabs listing all templates in the project. Both panes update together when switching templates.

### Indicators
- **Red alert icon** — template has a rendering error
- **Blue dot** — template output has changes compared to last deployment
- No indicator — template output is unchanged

---

## 4. Left Pane — Inputs

Shows every source input that contributes to rendering the selected template, in order:

### 4.1 Template Section
- **Header:** template name, pinned version
- **Body:** unified diff of the template text between last-deployed version and pinned version

### 4.2 Values Section
- **Header:** pinned version
- **Body:** diff of the JSON payload, pretty-printed

### 4.3 Global Values Sections
One collapsible section per Global Values entry referenced by the pinned values:
- **Header:** global values name, pinned version
- **Body:** diff of the flat JSON payload

### Collapse Behavior
- Sections with changes are **expanded** by default
- Unchanged sections are **collapsed** by default, showing an "unchanged" badge
- All sections are manually expandable/collapsible

---

## 5. Right Pane — Rendered Output

Shows the rendered config text as a diff between:
- **Old:** the `rendered_output` from the last successful deployment for this `(project, environment, template)`
- **New:** the output rendered using the pinned version set

If no prior deployment exists, the full rendered output is shown without diff markers.

### Error Display

If rendering fails (missing values, bad template syntax, unknown global values references, unknown keys), the right pane shows:
- A red error panel with the error message
- The error kind badge (e.g. `template_exec`, `unknown_key`, `unknown_global_values`)
- The **Deploy** button is disabled with a tooltip: "Fix rendering errors before deploying"

Error examples:
- *"template execution error in "app.yaml": template: app.yaml:3:12: executing "app.yaml" at <.db_host>: map has no entry for key "db_host""*
- *"unknown key "bad_key" in global values "test_db_values""*
- *"unknown global values entry "nonexistent_db""*

---

## 6. Diff Rendering

Both panes use unified diff conventions:
- Standard line-level coloring: additions (green), deletions (red)
- Summary line at the top of each diff: `+N −M lines`
- "No changes" message when old and new are identical

---

## 7. Deploy Action

### Preconditions
- No rendering errors across any template (Deploy button disabled if any errors exist)
- User has deploy permission (when implemented)

### Flow
1. User fills in optional **commit message**
2. User clicks **Deploy**
3. Confirmation dialog: *"Deploy [project] to [environment]? This will use the exact versions shown in this review and download the rendered configs as a zip file."*
4. System creates a Deployment record with status `succeeded`, capturing all pinned version IDs and rendered outputs
5. Rendered configs are packaged into a zip file and downloaded automatically
6. The pinned versions become the new "last deployment" baseline — next time this (project, environment) is opened, version selectors default to these versions

### Zip Download
The zip file is named `{project}-{environment}-deploy.zip` and contains one file per template, named by template_name, containing the rendered output.
