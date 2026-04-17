# Database Schema

## 1. Overview

This document defines the relational database schema for the config generation system. It covers all domain objects from the specification documents: projects, environments, config templates, config values, global values, versioning, deployments, pull requests, permissions, and roles.

All timestamps are stored as `TIMESTAMPTZ`. All primary keys use `BIGINT` auto-incrementing IDs unless otherwise noted.

---

## 2. Core Domain Tables

### 2.1 `projects`

| Column | Type | Constraints |
|---|---|---|
| `id` | `BIGINT` | PK, auto-increment |
| `name` | `TEXT` | UNIQUE, NOT NULL |
| `description` | `TEXT` | |
| `approval_condition` | `TEXT` | NOT NULL, DEFAULT `'1 x project_admin'` |
| `created_by` | `BIGINT` | FK → `users.id`, NOT NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |

### 2.2 `environments`

| Column | Type | Constraints |
|---|---|---|
| `id` | `BIGINT` | PK, auto-increment |
| `name` | `TEXT` | UNIQUE, NOT NULL |
| `description` | `TEXT` | |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |

### 2.3 `users`

| Column | Type | Constraints |
|---|---|---|
| `id` | `BIGINT` | PK, auto-increment |
| `username` | `TEXT` | UNIQUE, NOT NULL |
| `display_name` | `TEXT` | |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |

---

## 3. Versioned Object Tables

All versioned objects follow the full-copy snapshot strategy: each edit appends a new row with the complete payload. Versions are immutable once written.

### 3.1 `project_config_templates`

Stores every version of every template owned by a project.

| Column | Type | Constraints |
|---|---|---|
| `id` | `BIGINT` | PK, auto-increment |
| `project_id` | `BIGINT` | FK → `projects.id`, NOT NULL |
| `template_name` | `TEXT` | NOT NULL |
| `version_id` | `INTEGER` | NOT NULL |
| `body` | `TEXT` | NOT NULL |
| `commit_message` | `TEXT` | |
| `created_by` | `BIGINT` | FK → `users.id`, NOT NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |

- UNIQUE(`project_id`, `template_name`, `version_id`)
- `version_id` is monotonically increasing within each `(project_id, template_name)` scope.

### 3.2 `project_config_values`

Stores every version of the per-(project, template, environment) value set.

| Column | Type | Constraints |
|---|---|---|
| `id` | `BIGINT` | PK, auto-increment |
| `project_id` | `BIGINT` | FK → `projects.id`, NOT NULL |
| `template_name` | `TEXT` | NOT NULL |
| `environment_id` | `BIGINT` | FK → `environments.id`, NOT NULL |
| `version_id` | `INTEGER` | NOT NULL |
| `payload` | `JSONB` | NOT NULL |
| `commit_message` | `TEXT` | |
| `created_by` | `BIGINT` | FK → `users.id`, NOT NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |

- UNIQUE(`project_id`, `template_name`, `environment_id`, `version_id`)
- `version_id` is monotonically increasing within each `(project_id, template_name, environment_id)` scope.

### 3.3 `global_values`

Stores every version of each named Global Values entry.

| Column | Type | Constraints |
|---|---|---|
| `id` | `BIGINT` | PK, auto-increment |
| `name` | `TEXT` | NOT NULL |
| `version_id` | `INTEGER` | NOT NULL |
| `payload` | `JSONB` | NOT NULL |
| `commit_message` | `TEXT` | |
| `created_by` | `BIGINT` | FK → `users.id`, NOT NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |

- UNIQUE(`name`, `version_id`)
- `payload` must be a flat JSON object (single level of key-value pairs with scalar values).
- `version_id` is monotonically increasing within each `name` scope.

---

## 4. Deployment Tables

### 4.1 `deployments`

| Column | Type | Constraints |
|---|---|---|
| `id` | `BIGINT` | PK, auto-increment |
| `project_id` | `BIGINT` | FK → `projects.id`, NOT NULL |
| `environment_id` | `BIGINT` | FK → `environments.id`, NOT NULL |
| `status` | `TEXT` | NOT NULL, CHECK IN (`'pending'`, `'succeeded'`, `'failed'`, `'rolled_back'`) |
| `rolled_back_from` | `BIGINT` | FK → `deployments.id`, nullable |
| `commit_message` | `TEXT` | |
| `created_by` | `BIGINT` | FK → `users.id`, NOT NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |

- INDEX on (`project_id`, `environment_id`, `status`, `created_at` DESC) for "last successful deployment" lookups.

### 4.2 `deployment_entries`

One row per template in a deployment, capturing the pinned versions and rendered output.

| Column | Type | Constraints |
|---|---|---|
| `id` | `BIGINT` | PK, auto-increment |
| `deployment_id` | `BIGINT` | FK → `deployments.id`, NOT NULL |
| `template_name` | `TEXT` | NOT NULL |
| `template_version_id` | `INTEGER` | NOT NULL |
| `values_version_id` | `INTEGER` | NOT NULL |
| `rendered_output` | `TEXT` | NOT NULL |

- UNIQUE(`deployment_id`, `template_name`)

### 4.3 `deployment_entry_global_refs`

Tracks which Global Values versions were used per deployment entry.

| Column | Type | Constraints |
|---|---|---|
| `id` | `BIGINT` | PK, auto-increment |
| `deployment_entry_id` | `BIGINT` | FK → `deployment_entries.id`, NOT NULL |
| `global_values_name` | `TEXT` | NOT NULL |
| `global_values_version_id` | `INTEGER` | NOT NULL |

- UNIQUE(`deployment_entry_id`, `global_values_name`)

---

## 5. Pull Request Tables

### 5.1 `pull_requests`

| Column | Type | Constraints |
|---|---|---|
| `id` | `BIGINT` | PK, auto-increment |
| `project_id` | `BIGINT` | FK → `projects.id`, NOT NULL |
| `author_id` | `BIGINT` | FK → `users.id`, NOT NULL |
| `title` | `TEXT` | NOT NULL |
| `description` | `TEXT` | |
| `status` | `TEXT` | NOT NULL, CHECK IN (`'draft'`, `'open'`, `'approved'`, `'merged'`, `'closed'`) |
| `is_conflicted` | `BOOLEAN` | NOT NULL, DEFAULT false |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |
| `merged_at` | `TIMESTAMPTZ` | |
| `closed_at` | `TIMESTAMPTZ` | |

### 5.2 `pr_changes`

Each row is a proposed full-copy snapshot of one versioned object within a PR.

| Column | Type | Constraints |
|---|---|---|
| `id` | `BIGINT` | PK, auto-increment |
| `pr_id` | `BIGINT` | FK → `pull_requests.id`, NOT NULL |
| `object_type` | `TEXT` | NOT NULL, CHECK IN (`'template'`, `'values'`, `'global_values'`) |
| `project_id` | `BIGINT` | FK → `projects.id`, nullable (null for global_values) |
| `template_name` | `TEXT` | nullable (null for global_values) |
| `environment_id` | `BIGINT` | FK → `environments.id`, nullable (only for values) |
| `global_values_name` | `TEXT` | nullable (only for global_values) |
| `base_version_id` | `INTEGER` | NOT NULL — latest version when change was added to PR |
| `proposed_payload` | `TEXT` | NOT NULL — full content of proposed new version |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |

### 5.3 `pr_approvals`

| Column | Type | Constraints |
|---|---|---|
| `id` | `BIGINT` | PK, auto-increment |
| `pr_id` | `BIGINT` | FK → `pull_requests.id`, NOT NULL |
| `user_id` | `BIGINT` | FK → `users.id`, NOT NULL |
| `approved_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |
| `withdrawn_at` | `TIMESTAMPTZ` | |

- UNIQUE(`pr_id`, `user_id`) — one active approval per user per PR.
- The roles held by the approver at approval time are resolved by joining against `user_roles` at query time (or snapshotted if needed).

---

## 6. Permissions & Roles Tables

### 6.1 `roles`

| Column | Type | Constraints |
|---|---|---|
| `id` | `BIGINT` | PK, auto-increment |
| `name` | `TEXT` | NOT NULL |
| `project_id` | `BIGINT` | FK → `projects.id`, nullable (null for system-level roles) |
| `is_auto_created` | `BOOLEAN` | NOT NULL, DEFAULT false |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |

- UNIQUE(`name`, `project_id`) — role names are unique within a project (or globally if `project_id` is null).

### 6.2 `role_permissions`

Each row is one permission atom assigned to a role.

| Column | Type | Constraints |
|---|---|---|
| `id` | `BIGINT` | PK, auto-increment |
| `role_id` | `BIGINT` | FK → `roles.id`, NOT NULL |
| `action` | `TEXT` | NOT NULL — e.g. `'read'`, `'write'`, `'create'`, `'delete'`, `'grant'` |
| `resource` | `TEXT` | NOT NULL — e.g. `'project_templates'`, `'project_values'`, `'global_values'`, `'project'`, `'env_values'` |
| `key_project` | `TEXT` | nullable — project name or `'*'` |
| `key_env` | `TEXT` | nullable — environment name or `'*'` |
| `key_name` | `TEXT` | nullable — global values name or `'*'` |

- UNIQUE(`role_id`, `action`, `resource`, `key_project`, `key_env`, `key_name`)
- Wildcard `'*'` in a key slot means "all."
- `write` implies `read` — enforced at the application layer, not as duplicate rows.

### 6.3 `user_roles`

| Column | Type | Constraints |
|---|---|---|
| `id` | `BIGINT` | PK, auto-increment |
| `user_id` | `BIGINT` | FK → `users.id`, NOT NULL |
| `role_id` | `BIGINT` | FK → `roles.id`, NOT NULL |
| `granted_by` | `BIGINT` | FK → `users.id`, NOT NULL |
| `granted_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |

- UNIQUE(`user_id`, `role_id`)

---

## 7. Key Indexes

| Table | Index | Purpose |
|---|---|---|
| `project_config_templates` | (`project_id`, `template_name`, `version_id` DESC) | Fetch latest version of a template |
| `project_config_values` | (`project_id`, `template_name`, `environment_id`, `version_id` DESC) | Fetch latest values for a (template, env) pair |
| `global_values` | (`name`, `version_id` DESC) | Fetch latest version of a global values entry |
| `deployments` | (`project_id`, `environment_id`, `status`, `created_at` DESC) | Find last successful deployment |
| `pull_requests` | (`project_id`, `status`) | List open PRs for a project |
| `user_roles` | (`user_id`) | Look up all roles for a user |
| `role_permissions` | (`role_id`) | Look up all permissions for a role |

---


## 9. Design Notes

1. **Full-copy versioning.** Each versioned table stores complete payloads per version. Diffs are computed at read time by comparing two version rows. This matches the spec's "no deltas stored" requirement.

2. **Version ID scoping.** `version_id` is scoped to the object's natural key (e.g. per template name within a project), not globally. Application logic increments it atomically using `SELECT MAX(version_id) + 1 ... FOR UPDATE` or a sequence per scope.

3. **Approval condition storage.** The `approval_condition` on `projects` is stored as a text expression (e.g. `'1 x project_admin AND 1 x release_manager'`). The application parses and evaluates it at runtime.

4. **PR conflict detection.** `pr_changes.base_version_id` records the latest version at the time a change was added. At merge time, the app compares this against the current latest version to detect conflicts.

5. **Implied permissions.** `write` implying `read` and `create:env_values` implying `write:project_values(project, *)` are enforced in application-level permission checks, not duplicated in the `role_permissions` table.

6. **Global Values flatness.** The `global_values.payload` column stores a flat JSON object. The application validates on write that no nested objects or arrays are present.
