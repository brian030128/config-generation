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

---

## 3. Permission Atoms

### 3.1 Resource Read/Write

| Permission | Key | Meaning |
|---|---|---|
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
| `create:env_values(project)` | project | Create a new value set for any `(template, env)` pair in the project. Payload-bearing: the caller supplies the initial value JSON, which becomes v1 of the record. Fails if a record for that `(template, env)` already exists. **Implies `write:project_values(project, *)`** (and therefore read). |
| `delete:project(project)` | project | Delete the project. Scope of cascade (templates, values, deployments) is out of scope for this spec. |
| `delete:project_values(project, env)` | project, env | Delete the value set(s) for the given `(project, env)`. |

### 3.3 Administration

| Permission | Key | Meaning |
|---|---|---|
| `grant(project)` | project | Modify role assignments and role definitions within the scope of the project. Exact semantics (grant-only-what-you-hold vs. grant-anything-in-scope) are **open** — see §6. |

### 3.4 Deferred

The following are referenced in the system but **not yet specified** in this permission model:

- `deploy(project, env)` and/or `rollback(project, env)` — gating the Deploy action in the deployment review GUI. See §6.
- Ownership and grant authority over Global Values entries. See §6.
- Transitive read of Global Values referenced by a Project Config Value. See §6.

---

## 4. Roles

A **role** is a named bundle of permission atoms. Users are granted permissions by being assigned roles; permissions are not granted directly to users.

Roles are themselves first-class objects and can be created, edited, and assigned. A user's effective permissions are the union of the permissions granted by all roles they hold.

### 4.1 Auto-Created Roles

When a project is created, the system automatically creates a **project admin** role scoped to that project and assigns it to the creator.

#### `project_admin:<P>`

Contains the following permissions:

- `write:project_templates(P)` — full authoring on all templates in the project.
- `create:env_values(P)` — bootstrap new envs and (by implication) modify any existing env values in the project.
- `delete:project_values(P, *)` — tear down any env's values in the project.
- `delete:project(P)` — delete the project itself.
- `grant(P)` — manage roles and assignments within the project.
- *(Deploy permission — TBD, see §6.)*

Note: `write:project_values(P, *)` is not listed separately because it is implied by `create:env_values(P)`.

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

2. **Global Values ownership.** Global Values are cross-project by design, so no project admin owns them. Who can create a new Global Values entry? Who can grant `write:global_values(name)` on an existing one? Candidate answers: a system-level `system_admin` role with `*`; per-entry ownership where the creator gets admin on that entry; or a separate `create:global_values` atom plus per-entry grant rights.

3. **Transitive read of referenced globals.** A Project Config Value may reference `${test_db_values.password}`. Rendering and deployment review need to read `test_db_values`. Does `read:project_values(P, env)` transitively authorize reading the globals referenced by those values, or must every reviewer hold explicit `read:global_values(name)` for each referenced entry? Affects the usability of the deployment review GUI directly.

4. **`grant(project)` semantics.** Two open sub-questions:
   - Can a grantor grant any project-scoped permission, or only permissions they themselves hold?
   - Can a project admin revoke the original project creator's admin role (risking lockout), or is the creator's role protected?

5. **`create:template(project)` split.** Currently `write:project_templates(project)` covers both creating new templates and appending versions to existing ones. Whether to split out a create-only permission (symmetric with `create:env_values`) is open. Default assumption: keep merged unless a concrete use case demands the split.

