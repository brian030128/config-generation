# Role Management Page

**Route:** `/projects/:projectName/roles` (project-scoped), `/global-values/:name/roles` (entry-scoped), or `/admin/roles` (system-level)

Manages roles, their permissions, and user assignments. Accessible to users with `grant(project)` for project-scoped roles, or `grant(global_values, name)` for Global Values entry-scoped roles.

```
┌─────────────────────────────────────────────────────────────────┐
│  Roles — billing-service                     [+ Create Role]    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  project_admin:billing                    (auto-created)  │  │
│  │  5 permissions · 1 member                                 │  │
│  │                                                           │  │
│  │  Permissions:                                             │  │
│  │    write:project_templates(billing)                       │  │
│  │    create:env_values(billing)                             │  │
│  │    delete:project_values(billing, *)                      │  │
│  │    delete:project(billing)                                │  │
│  │    grant(billing)                                         │  │
│  │                                                           │  │
│  │  Members:                                                 │  │
│  │    alice (creator)                           [Remove]     │  │
│  │                                          [+ Add Member]   │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  billing_staging_operator                                 │  │
│  │  1 permission · 2 members                                 │  │
│  │                                                           │  │
│  │  Permissions:                                             │  │
│  │    write:project_values(billing, staging)                 │  │
│  │                                                           │  │
│  │  Members:                                                 │  │
│  │    bob                                       [Remove]     │  │
│  │    carol                                     [Remove]     │  │
│  │                                          [+ Add Member]   │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 1. Role List

Each role is displayed as an expandable card showing:
- **Role name** and auto-created badge if applicable
- **Permission count** and **member count**
- Expanded view: full permission list and member list

---

## 2. Create Role

The **"+ Create Role"** button opens a dialog:

- **Name** — text input (required, unique within project scope)
- **Permissions** — add one or more permission atoms:
  - **Action** dropdown: `read`, `write`, `create`, `delete`, `grant`
  - **Resource** dropdown: `project_templates`, `project_values`, `global_values`, `env_values`, `project`
  - **Key fields** — dynamic based on resource type:
    - `project_templates`: project (pre-filled)
    - `project_values`: project (pre-filled), environment (dropdown or `*`)
    - `global_values`: name (text or `*`)
    - `env_values`: project (pre-filled)
    - `project`: project (pre-filled)
  - **"+ Add Permission"** to add more atoms

---

## 3. Member Management

### "+ Add Member" Button
Opens a user search dialog. Select a user to assign them the role.

### "Remove" Button
Removes the user from the role. Confirmation required.

### Constraints
- Auto-created roles cannot be deleted (but members can be managed)
- Cannot remove the last member of a `project_admin` role if it would lock out the project

---

## 4. Edit / Delete Role

- **Edit** — modify the permission atoms in a custom role. Auto-created roles' permissions are read-only.
- **Delete** — remove a custom role entirely. All member assignments are revoked. Confirmation required.
