# Deployment Review Page

**Route:** `/projects/:projectName/env/:envName/deploy`

The split-pane review interface where users inspect exactly what will change before deploying. All version IDs are pinned at page load and remain fixed throughout the review session.

```
┌─────────────────────────────────────────────────────────────────────────┐
│  Deploy: billing-service → staging                                      │
│  Template: [ app.yaml ▾ ]  [database.conf]  [nginx.conf●]              │
│                                                   ● = has changes       │
│  ⓘ Newer versions available — [Refresh]                                 │
├────────────────────────────────┬────────────────────────────────────────┤
│  INPUTS                        │  RENDERED OUTPUT                       │
│                                │                                        │
│  ▸ Template: app.yaml          │                                        │
│    v7 → v8                     │   service: billing                     │
│  ┌──────────────────────────┐  │   environment: staging                 │
│  │  @@ -3,4 +3,5 @@        │  │   database:                            │
│  │   service: {{ .svc }}    │  │  -  host: old-db.internal              │
│  │  +environment: {{ .env }}│  │  +  host: test-db.internal             │
│  │   database:              │  │     port: 5432                         │
│  └──────────────────────────┘  │     user: app                          │
│                                │  -  password: old_pass                  │
│  ▸ Values: app.yaml/staging    │  +  password: s3cret                   │
│    v12 → v13                   │   features:                            │
│  ┌──────────────────────────┐  │     new_checkout: true                 │
│  │  @@ -2,3 +2,4 @@        │  │  +  legacy_invoices: false             │
│  │   "env": "staging",     │  │                                        │
│  │  +"db_host": "${test_db… │  │                                        │
│  │   "db_port": "${test_db… │  │                                        │
│  └──────────────────────────┘  │                                        │
│                                │                                        │
│  ▸ Global: test_db_values      │                                        │
│    v3 → v4                     │                                        │
│  ┌──────────────────────────┐  │                                        │
│  │  "host": "old-db…"      │  │                                        │
│  │  "host": "test-db…"     │  │                                        │
│  └──────────────────────────┘  │                                        │
│                                │                                        │
│  ▾ Global: shared_secrets      │                                        │
│    v9 (unchanged)              │                                        │
│                                │                                        │
├────────────────────────────────┴────────────────────────────────────────┤
│  Commit message: [___________________________________]                  │
│                                             [Cancel]  [Deploy]          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 1. Version Pinning

When the page loads, the system captures the **candidate version set** — the latest version IDs of:
- All templates in the project
- All values for the selected environment
- All Global Values entries referenced by those values

These version IDs are **frozen** for the entire review session. Concurrent edits by other users do not affect this session.

### "Refresh" Button
If newer versions exist for any pinned input, a banner appears: *"Newer versions available."* Clicking **Refresh** re-captures a new candidate version set and reloads all panes.

### Rollback Mode
When opened from "Rollback to this" in the deployment history, the candidate version set uses the versions from the selected prior deployment instead of the current latest. The header shows: *"Rollback to deployment #N"* and the Deploy button reads **"Rollback"**.

---

## 2. Template Selector

Horizontal tabs or dropdown listing all templates in the project. Both panes update together when switching templates.

### Change Badges
Each template tab shows a dot indicator if **any** of its inputs (template body, values, or referenced globals) differ between the pinned candidate and the last deployment. Lets the user quickly scan which templates will produce different output.

---

## 3. Left Pane — Inputs

Shows every source input that contributes to rendering the selected template, in order:

### 3.1 Template Section
- **Header:** template name, version transition (e.g. `v7 → v8`), or version number + "unchanged" tag
- **Body:** git-style diff of the template text between last-deployed version and pinned candidate version

### 3.2 Values Section
- **Header:** values identifier (template/env), version transition
- **Body:** diff of the JSON, pretty-printed with stable key ordering

### 3.3 Global Values Sections
One section per Global Values entry referenced by the pinned values. Entries are listed with:
- **Header:** global values name, version transition
- **Added/removed tags** if the entry is newly referenced or no longer referenced compared to the last deployment
- **Body:** diff of the flat JSON

### Collapse Behavior
- Sections with changes are **expanded** by default
- Unchanged sections are **collapsed** by default, showing only the header with an "unchanged" tag
- All sections are manually expandable/collapsible

---

## 4. Right Pane — Rendered Output

Shows the rendered config text as a diff between:
- **Old:** the `rendered_output` from the last successful deployment for this `(project, environment, template)`
- **New:** the output rendered using the pinned candidate version set

If no prior deployment exists, the entire output is shown as additions.

If rendering fails (missing values, bad template syntax, unknown references), the pane shows the error message and the **Deploy** button is disabled.

---

## 5. Diff Rendering

Both panes use the same conventions:
- **Unified or split view**, user-toggleable (persisted per user preference)
- Standard line-level coloring: additions (green), deletions (red)
- Syntax highlighting based on format (Go template for inputs, detected format for output)
- Summary line at the top of each diff: `+N −M lines`

---

## 6. Deploy Action

### Preconditions
- No rendering errors
- User has deploy permission (when implemented)

### Flow
1. User fills in optional **commit message**
2. User clicks **Deploy**
3. Confirmation dialog: *"Deploy billing-service to staging? This will use the exact versions shown in this review."*
4. System creates a Deployment record with status `pending`, capturing all pinned version IDs and rendered outputs
5. On success → status becomes `succeeded`, this becomes the new "last deployment"
6. On failure → status becomes `failed`, user sees error, prior deployment remains the baseline

### Cancel
Returns to the project-env-page. No state is persisted from the review session.
