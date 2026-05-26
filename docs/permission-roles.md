# Permissions — Specification

## 1. Overview

This document specifies the permission model for the config generation and deployment system. It defines the atomic permissions that can be granted, how they compose (write implies read; wildcards on key slots), and how roles bundle permissions for assignment to users.

This spec covers permissions over the domain objects defined in the Config Generation System spec (Projects, Environments, Project Config Templates, Project Config Values, Global Values) and the Deployment records defined in the Config Version Control & Deployment GUI spec.

---

## 2. Permission Model

### 2.1 Shape

A permission is a triple of `(action, resource, key)`, written as `action:resource(key)`. The key identifies *which instance* of the resource the permission applies to, and varies by resource type.

### 2.2 Composition Rules

- **Write implies read.** Granting `write:X(k)` also grants `read:X(k)`. The reverse does not hold.
- **Wildcards.** The `*` character may appear in any slot of a permission key. `write:project_values(billing, *)` means "write on billing's values for any environment." `write:project_values(*, staging)` means "write on staging values across all projects." Wildcards compose on every axis independently.

### 2.3 Versioning Interaction

Since Project Config Templates, Project Config Values, and Global Values are versioned (full-copy snapshots per the deployment spec), write permissions authorize appending new versions. Reads authorize inspecting any historical version. Deletes operate on the logical object and its entire version history.

For **project** objects, these permissions are enforced when a change is **staged into a
workspace** (the author's active draft PR — see [`pr-flow.md`](./pr-flow.md) §4.1), which is the
only write path. Staging a template/value change requires the corresponding `write`/`create`;
staging a delete requires the corresponding `delete`. The **merge** is what actually appends the
version (or removes the object); it is gated separately by PR authorship and the project's
approval condition, and does **not** re-check these atoms. (Global Values remain on their
existing direct-append + per-entry-PR flow.)

---

## 3. Permission Atoms

### 3.1 Resource Read/Write

| Permission | Key | Meaning |
|---|---|---|
| `read:project(project)` | project | Read the project's own metadata (name, description, approval condition) and see it listed. Granted **only** by project membership (see §4.3) — it is **not** implied by, and does **not** imply, any other project-scoped permission. In particular it does *not* grant read on templates, environments, or values. |
| `read:project_templates(project)` | project | Read any template owned by the project, at any version. |
| `write:project_templates(project)` | project | Append new versions of any template owned by the project. Includes creating new templates in the project. Implies read. |
| `read:project_values(project, env)` | project, env | Read the value set for any template in the project at the given env, at any version. |
| `write:project_values(project, env)` | project, env | Append a new version to an **existing** value set for any template in the project at the given env. Fails if the record does not exist. Implies read. |
| `read:global_values(name)` | name | Read the named Global Values entry, at any version. |
| `write:global_values(name)` | name | Append a new version of the named Global Values entry. Implies read. |

### 3.2 Authoring & Lifecycle

| Permission | Key | Meaning |
|---|---|---|
| `create:project` | — | Create a new project. System-level; no scope key. |
| `create:env_values(project)` | project | Create a new value set for any `(template, env)` pair in the project. Payload-bearing: the caller supplies the initial value JSON, which becomes v1 of the record. Fails if a record for that `(template, env)` already exists. Also authorizes **staging the creation of a new environment** (which exists precisely to hold value sets). **Implies `write:project_values(project, *)`** (and therefore read). |
| `delete:project_templates(project)` | project | Delete a template owned by the project, including its entire version history. Symmetric with `write:project_templates` (which covers create + append). |
| `delete:project(project)` | project | Delete the project. Scope of cascade (templates, values, deployments) is out of scope for this spec. |
| `delete:project_values(project, env)` | project, env | Delete the value set(s) for the given `(project, env)`. The wildcard form `delete:project_values(project, *)` additionally authorizes **deleting an environment** (which cascades to all its value sets). |

### 3.3 Administration

| Permission | Key | Meaning |
|---|---|---|
| `grant(project)` | project | Modify role assignments and role definitions within the scope of the project. Exact semantics (grant-only-what-you-hold vs. grant-anything-in-scope) are **open** — see §6. |
| `grant(global_values, name)` | name | Modify role assignments and role definitions scoped to the named Global Values entry. Also allows updating the entry's approval condition. |

### 3.4 Deferred

The following are referenced in the system but **not yet specified** in this permission model:

- `deploy(project, env)` and/or `rollback(project, env)` — gating the Deploy action in the deployment review GUI. See §6.
- Transitive read of Global Values referenced by a Project Config Value. See §6.

---

## 4. Roles

A **role** is a named bundle of permission atoms. Users are granted permissions by being assigned roles; permissions are not granted directly to users.

Roles are themselves first-class objects and can be created, edited, and assigned. A user's effective permissions are the union of the permissions granted by all roles they hold.

### 4.1 Auto-Created Roles

When a project is created, the system automatically creates a **project admin** role scoped to that project and assigns it to the creator.

#### `project_admin:<P>`

Contains the following permissions:

- `write:project_templates(P)` — full authoring on all templates in the project (create + append versions).
- `delete:project_templates(P)` — delete any template in the project.
- `create:env_values(P)` — bootstrap new envs and (by implication) modify any existing env values in the project.
- `delete:project_values(P, *)` — tear down any env's values in the project, and delete environments themselves.
- `delete:project(P)` — delete the project itself.
- `grant(P)` — manage roles and assignments within the project, **and add/remove project members** (see §4.3).
- *(Deploy permission — TBD, see §6.)*

Note: `write:project_values(P, *)` is not listed separately because it is implied by `create:env_values(P)`. The creator is also automatically added as a **member** of the project (see §4.3), which is what gives them `read:project(P)`.

#### `gv_group_admin:<N>`

Created automatically when a Global Values entry `N` is created. Assigned to the creator. Contains the following permissions:

- `write:global_values(N)` — modify the entry (append new versions).
- `delete:global_values(N)` — delete the entry and its entire version history.
- `grant(global_values, N)` — manage roles and assignments scoped to this entry, and update the entry's approval condition.

### 4.2 Illustrative Custom Roles

The following are example role shapes the model supports. They are not auto-created.

- **Env-scoped operator** (e.g. `billing_staging_operator`):
  `write:project_values(billing, staging)`.
  Can edit existing staging values for billing only. Cannot touch prod, cannot bootstrap new envs, cannot modify templates.

- **Cross-project env provisioner**:
  `create:env_values(*)`.
  Can stand up new envs for any project, and — via implication — modify any existing env's values anywhere. Useful for infra-provisioning workflows.

- **Project read-only auditor** (e.g. `billing_auditor`):
  `read:project_templates(billing)` + `read:project_values(billing, *)`.
  Can inspect all templates and value sets (at any version) for billing. Cannot modify anything.

### 4.3 Project Membership

A project has a set of **members**. Membership is a first-class relationship (the
`project_members` table), distinct from roles:

- Being a member grants exactly one thing: `read:project(P)` — the member can read the
  project's metadata and see it in their project list. Membership grants **nothing** on the
  project's templates, environments, or values; those remain gated by their own atoms.
- `read:project(P)` is **synthesized from membership** at permission-load time; it is never
  stored as a `role_permissions` row and is not implied by any other permission. Equivalently:
  to read a project you must be a member of it (or a superuser).
- The project creator is auto-added as a member at creation. Other members are added/removed
  by a `grant(P)` holder via the members endpoints (`POST`/`DELETE /api/projects/{P}/members`).
- Removing a member also revokes that user's project-scoped role assignments, so no
  project permissions can outlive membership. Removing the sole member of `project_admin:<P>`
  is refused (lockout protection).

The typical flow is **add the user as a member first, then grant them permissions** (assign
roles). A bare member sees the project shell; an admin layers template/value/grant
permissions on top via roles.

---

## 5. Examples

### 5.1 Creating a new environment for an existing project

Alice wants to stand up `eu-prod` values for the `billing` project. The value set for `(app.yaml, eu-prod)` does not yet exist.

Required permission: `create:env_values(billing)` (or any wildcard that covers it).

`write:project_values(billing, eu-prod)` is **not sufficient** on its own, because the record does not exist — write fails when the record is absent. However, a user with `create:env_values(billing)` does not need a separate write grant, because create implies write across the project.

### 5.2 Editing existing staging values

Bob has `write:project_values(billing, staging)` and nothing else. He can append new versions of any existing value set for billing at staging. If a new template is added to billing and no staging values exist for it yet, Bob cannot create them — he'd need `create:env_values(billing)` or for someone else to bootstrap the record first.

### 5.3 Project admin is not a global admin

Carol is `project_admin:billing`. She can do anything within billing, including granting `read:global_values(test_db_values)` *if she herself holds that permission* (under the grant-only-what-you-hold reading of §6). She cannot unilaterally grant permissions on other projects, nor create new Global Values entries, nor delete Global Values entries — those powers live outside the project scope and are governed by whatever global/system-level role(s) §6 ultimately defines.

---

## 6. Open Questions

The following are explicitly unresolved and need decisions before the model is complete:

1. **Deploy and rollback permissions.** The deployment review GUI's Deploy action (and the rollback flow) currently have no permission gate. At minimum a `deploy(project, env)` permission is needed, and it should likely *not* be implied by write on values — authoring a value and pushing it to prod are different trust levels. Whether rollback is the same permission or separate is also open.

2. **Global Values ownership.** ~~Resolved.~~ Each Global Values entry has per-entry ownership: the creator is automatically assigned the `gv_group_admin:<name>` role, which grants write, delete, and grant authority over that entry. The `grant(global_values, name)` permission controls who can manage roles and the approval condition for the entry. See §4.1.

3. **Transitive read of referenced globals.** A Project Config Value may reference `${test_db_values.password}`. Rendering and deployment review need to read `test_db_values`. Does `read:project_values(P, env)` transitively authorize reading the globals referenced by those values, or must every reviewer hold explicit `read:global_values(name)` for each referenced entry? Affects the usability of the deployment review GUI directly.

4. **`grant(project)` semantics.** Two open sub-questions:
   - Can a grantor grant any project-scoped permission, or only permissions they themselves hold?
   - Can a project admin revoke the original project creator's admin role (risking lockout), or is the creator's role protected?

5. **`create:template(project)` split.** ~~Open.~~ **Resolved (partially):** `write:project_templates(project)` continues to cover both creating new templates and appending versions — no separate create-only atom. However, since template **deletion** now flows through the workspace/PR like any other change, a dedicated `delete:project_templates(project)` atom is introduced (see §3.2), symmetric with `delete:project_values`. A create-only split remains unadopted absent a concrete use case.

