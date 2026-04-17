# Project List Page

**Route:** `/projects`

## Layout

A simple list view of all projects the current user has read access to.

```
┌─────────────────────────────────────────────────────────┐
│  Projects                            [+ New Project]    │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │  billing-service                                │    │
│  │  Payment processing service configs             │    │
│  │  3 templates · 4 environments · Updated 2h ago  │    │
│  └─────────────────────────────────────────────────┘    │
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │  auth-service                                   │    │
│  │  Authentication and authorization configs       │    │
│  │  2 templates · 3 environments · Updated 1d ago  │    │
│  └─────────────────────────────────────────────────┘    │
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │  api-gateway                                    │    │
│  │  (no description)                               │    │
│  │  5 templates · 4 environments · Updated 5m ago  │    │
│  └─────────────────────────────────────────────────┘    │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

## Elements

### Project Card
Each project is displayed as a clickable card showing:
- **Project name** (bold)
- **Description** (muted, or "no description" if empty)
- **Summary line:** template count, environment count, last updated timestamp

Clicking a card navigates to the **project-page** for that project.

### "+ New Project" Button
Visible only to users with `create:project` permission. Opens a dialog:
- **Name** — text input (required, must be unique)
- **Description** — text area (optional)
- **Approval condition** — text input, pre-filled with `1 x project_admin`

On creation, the system auto-creates the `project_admin:<name>` role and assigns it to the creator.

## Sorting and Filtering
- Default sort: last updated (most recent first)
- Search bar filters projects by name (client-side substring match)
