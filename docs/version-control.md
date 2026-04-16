# Config Version Control & Deployment GUI — Specification

## 1. Overview

This document specifies the version control and deployment review system that sits on top of the [Config Generation System](#). It defines:

1. How versioned snapshots are taken of the three versionable domain objects.
2. How a "deployment" is recorded as a reference point for future diffs.
3. The deployment review GUI, which presents a side-by-side view of source inputs and rendered output, each annotated with a diff against the last deployment.

The goal is to give operators a clear, auditable view of *what is about to change* — both in the inputs they author and in the generated output that will actually hit production — before they commit to a deployment.

---

## 2. Versioned Objects

Three domain objects are versioned. All other objects (Project, Environment) are not versioned because they are pure identifiers / namespacing.

| Object                    | Versioned? | Version key                          |
| ------------------------- | ---------- | ------------------------------------ |
| Project                   | No         | —                                    |
| Environment               | No         | —                                    |
| **Project Config Template** | Yes      | `(project, template_name)`           |
| **Project Config Value**    | Yes      | `(project, template_name, environment)` |
| **Global Values**           | Yes      | `(global_values_name)`               |

### 2.1 Versioning Strategy: Full Copy

For simplicity, every edit produces a **full copy** of the object as a new version row. No diffs or deltas are stored — diffs are computed on read by comparing two full snapshots.

Each version row contains:

- `version_id` — monotonically increasing integer, unique within the object's version key scope.
- `created_at` — timestamp.
- `created_by` — author identity.
- `payload` — full content of the object at this version (template text, values JSON, or global values JSON).
- `commit_message` — optional free-text note from the author.

A version is **immutable** once written. Edits always append a new version row.

### 2.2 "Latest" vs "Deployed"

For each versioned object, two pointers are tracked **per environment** (where applicable):

- **Latest version** — the most recently authored version. This is what the editor screen shows and edits.
- **Last deployed version** — the version that was active in the most recent successful deployment to a given environment.

Note that templates and global values are not inherently scoped to an environment, but their *deployment pointer* is — the same template version may be the "last deployed" version in `staging` while a newer version is "last deployed" in `dev`.

---

## 3. Deployments

### 3.1 Definition

A **Deployment** is an immutable record that snapshots, for a single `(project, environment)` pair, the exact set of versions used to render every config file produced for that project at that moment in time.

A Deployment record contains:

- `deployment_id` — unique ID.
- `project`, `environment` — the target.
- `created_at`, `created_by`, `commit_message`.
- `status` — `pending`, `succeeded`, `failed`, `rolled_back`.
- `entries` — one entry per template owned by the project, each containing:
    - `template_name`
    - `template_version_id` — the Project Config Template version used.
    - `values_version_id` — the Project Config Value version used.
    - `global_values_refs` — list of `(global_values_name, version_id)` pairs for every Global Values entry referenced by the resolved values.
    - `rendered_output` — the final rendered config text.

Because the Deployment captures every input version *and* the rendered output, it is fully reproducible and auditable without re-running the generation pipeline.

### 3.2 "Last Deployment" Semantics

The **last deployment** for a `(project, environment)` pair is the most recent Deployment with status `succeeded`. This is the baseline against which all diffs in the deployment GUI are computed.

If no prior successful deployment exists, the diff baseline is empty — every input and the rendered output are shown as "all added."

---

## 4. Deployment Review GUI

### 4.1 Entry Point and Version Pinning

The user selects a `(project, environment)` pair and clicks **Review Deployment**. The system:

1. Resolves the **latest** version IDs of the project's templates, their corresponding values for the chosen environment, and every Global Values entry referenced by those values. **These version IDs are captured immediately and pinned for the lifetime of the review session.** Call this set the **candidate version set**.
2. Loads the **last successful deployment** for the same `(project, environment)` as the diff baseline.
3. Renders all templates using the pinned candidate version set to produce the *proposed* output.
4. Opens the Review GUI.

#### 4.1.1 Pinning Semantics

Once the candidate version set is captured, the entire review session — every input shown in the left pane, every rendered output in the right pane, and the eventual deployment — operates on those exact version IDs. Any edits authors make to templates, values, or global values *after* the GUI loads are invisible to this session and have no effect on what gets deployed.

This means:

- The GUI displays exact version numbers (e.g. `template v8`, `values v13`, `test_db_values v4`) for every input, not "latest."
- Refreshing the review session is an explicit user action (a **Refresh** button) that re-captures a new candidate version set against current latest versions.
- If a newer version of any pinned input exists at the moment of refresh or deploy, the GUI may show a subtle indicator ("newer version available — refresh to include") but does not silently pull it in.

### 4.2 Layout

The GUI is split vertically into two panes of equal width. A template selector at the top lets the user switch between the project's templates; both panes update together.

```
┌─────────────────────────────────────────────────────────────────────┐
│  Project: billing-service     Environment: staging                  │
│  Template: [ app.yaml ▾ ]   [database.conf] [nginx.conf]            │
├──────────────────────────────────┬──────────────────────────────────┤
│  LEFT: Inputs (with diffs)       │  RIGHT: Rendered Output (diff)   │
│                                  │                                  │
│  ▸ Template: app.yaml      v7→v8 │                                  │
│  ▸ Values:   app.yaml/stg  v12→v13│   <unified or split diff of      │
│  ▸ Global:   test_db_values v3→v4│    rendered config text>         │
│  ▸ Global:   shared_secrets v9   │                                  │
│    (unchanged)                   │                                  │
│                                  │                                  │
│  [collapsible diff for each]     │                                  │
└──────────────────────────────────┴──────────────────────────────────┘
                  [ Cancel ]      [ Deploy ]
```

### 4.3 Left Pane — Inputs

The left pane shows every **source input** that contributed to rendering the currently selected template, in this order:

1. The **Project Config Template** itself.
2. The **Project Config Value** for `(template, environment)`.
3. Each **Global Values** entry referenced by the values JSON.

Each input is rendered as a collapsible section with a header showing:

- The object's name and type.
- The version transition: `last_deployed_version → pinned_candidate_version`. If unchanged, show only the version number and a muted "unchanged" tag.
- An "added in this deployment" tag if the input did not exist in the last deployment (e.g. a newly referenced Global Values entry).

When expanded, each section shows a **git-style diff** between the last-deployed version and the pinned candidate version of that input:

- For templates → text diff of the template body.
- For values → diff of the JSON, pretty-printed with stable key ordering so the diff is meaningful.
- For global values → same as values.

If an input is unchanged, the section is collapsed by default and the diff view shows the full content with no diff markers.

#### 4.3.1 Determining Referenced Global Values

The "Global Values entries referenced" list is computed by scanning the **pinned candidate** Project Config Value for `${name.key}` references and taking the union of `name`s. The set is also computed for the last-deployed values; any entry that appears in one set but not the other is marked **added** or **removed** accordingly.

### 4.4 Right Pane — Rendered Output

The right pane shows the **rendered config text** for the currently selected template, displayed as a git-style diff between:

- **Old side:** the `rendered_output` stored in the last successful Deployment for this `(project, environment, template)`.
- **New side:** the output rendered using the pinned candidate version set.

If no prior deployment exists, the entire output is shown as added.

If rendering the pinned candidate version set fails (per §6 of the generation spec), the right pane shows the error and the **Deploy** button is disabled.

### 4.5 Diff Rendering

Both panes use the same diff conventions:

- Unified or split view, user-toggleable; the choice is persisted per user.
- Standard line-level additions (green) and deletions (red).
- Syntax highlighting based on the file's apparent format (template body and rendered output are highlighted; JSON inputs use JSON highlighting).
- A summary line at the top of each diff: `+N -M lines`.

### 4.6 Cross-Template Navigation

The template selector at the top shows a badge next to each template indicating whether *anything* feeding into it has changed since the last deployment (template body, values, or any referenced global values). This lets the user quickly scan which templates will produce different output.

---

## 5. Deploy Action

When the user clicks **Deploy**:

1. The system constructs a new Deployment record using the **pinned candidate version set** captured at GUI load (or last refresh). No re-resolution against "latest" occurs — the deployment uses exactly the version IDs the user reviewed.
2. Each template is rendered using its pinned template version, pinned values version, and the pinned versions of all referenced Global Values entries. The rendered outputs match what was shown in the right pane.
3. The Deployment record is written with status `pending`, capturing every pinned version ID and its rendered output.
4. The deployment is executed (mechanism out of scope for this spec).
5. On success, status is set to `succeeded` and this becomes the new "last deployment" for future diffs. On failure, status is set to `failed` and the prior successful deployment remains the diff baseline.

Because version IDs are pinned end-to-end, what the user reviewed is bit-for-bit what gets deployed, regardless of any concurrent edits by other authors.

---

## 6. Rollback

A rollback is modeled as a new Deployment whose entries point to the version IDs from a prior successful Deployment, rather than to the latest versions. The Deployment record carries a `rolled_back_from` field referencing the deployment that was undone, and the GUI labels it as a rollback in deployment history.

After a successful rollback, the rollback deployment becomes the new "last deployment" — subsequent diffs are computed against it, not against the pre-rollback state.

---

## 7. Out of Scope

The following are intentionally not specified here:

- Branching, merging, or pull-request-style review workflows on the versioned objects.
- Approval / multi-party sign-off before deploy.
- Storage backend for version rows and deployment records.
- The actual mechanism by which a successful deployment delivers rendered configs to runtime systems.
- Garbage collection or retention policy for old versions.