# UI Flow Overview

## Navigation Structure

```
Sidebar (persistent)
├── Projects          → project-listpage
├── Global Values     → global-values-listpage
├── Pull Requests     → pull-requests-page
├── Workspace         → workspace-page
└── User Menu
    └── Role Management (admin only)

Top Bar
└── Breadcrumb trail (e.g. Projects > billing-service > staging)
```

## Page Map

```
project-listpage
  └── project-page (click a project) — read-only view of current live state
        └── project-env-page (click an environment) — read-only live values
              └── deployment-review-page (click "Review Deployment")

workspace-page — lists the user's active draft/PR per project
  └── workspace-project-page (click a project workspace)
        ├── template-editor (edit a template) — saves to draft PR
        └── workspace-env-page (edit environment values) — saves to draft PR

global-values-listpage
  └── global-values-detail-page (click an entry)

pull-requests-page (Open / Closed tabs)
  └── pr-detail-page (click a PR)

role-management-page (admin)
```

## Core User Journeys

### 1. Author a config change
workspace-page → select project (or start new workspace) → edit templates / environment values → changes accumulate in draft PR → submit PR for review

### 2. Review and merge a PR
pull-requests-page → pr-detail-page → inspect diffs → approve → author merges

### 3. Deploy to an environment
project-page → click environment → project-env-page → "Review Deployment" → deployment-review-page → review diffs → Deploy

### 4. Manage shared values
global-values-listpage → global-values-detail-page → edit key-value pairs → create PR

### 5. Rollback a deployment
project-env-page → deployment history → select prior deployment → "Rollback to this" → deployment-review-page (rollback mode)
