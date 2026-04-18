# Config Generation System — Specification

## 1. Overview

This document specifies a config generation system that produces environment-specific configuration files for projects. The system uses [Go templates](https://pkg.go.dev/text/template) to render project-defined config templates, hydrating them with per-environment data that may reference reusable, globally-defined value sets.

The core problem this system solves: a single project typically needs multiple config files, each rendered differently per environment (e.g. `dev`, `staging`, `prod`), while many values (database credentials, shared service endpoints, etc.) are reused across many projects and should be defined once.

---

## 2. Domain Objects

The system is much more complicated than what's described here; the full data model will be designed later. For now, the following text descriptions of the core domain objects are sufficient.

### 2.1 Project

A **Project** represents a logical application or service that requires generated configuration. Each project has a unique human-readable name and an optional description. A project owns one or more Project Config Templates.

### 2.2 Environment

An **Environment** represents a deployment target such as `dev`, `staging`, `prod`, or `eu-prod`. Environments are global — they are not scoped to a single project — so the same environment definition can be referenced by every project. Each environment has a unique name and an optional description.

### 2.3 Project Config Template

A **Project Config Template** is a Go template owned by a project. A project may have many templates (e.g. `app.yaml`, `database.conf`, `nginx.conf`). The template's output format is **not constrained** by the system — it can be YAML, JSON, TOML, INI, plain text, or anything else. The system only renders text. A template's name is unique within its owning project.

### 2.4 Global Values

A **Global Values** entry is a named, reusable bag of key-value data, intended for values that are shared across many projects — e.g. `test_db_values` containing `db_host`, `db_port`, `password`. Each entry has a globally unique name.

For simplicity, **Global Values are always a single, flat level of key-value pairs**. No nesting. Values are scalars (strings, numbers, booleans).

Each Global Values entry is its own governance unit. When creating an entry, the creator specifies an **approval condition** (same grammar as project approval conditions — see the PR Flow spec) that governs how PRs proposing changes to that entry are reviewed and approved. The system auto-creates an admin role for the entry and assigns it to the creator, mirroring the project creation flow.

Example `test_db_values`:

```json
{
  "db_host": "test-db.internal",
  "db_port": 5432,
  "username": "app",
  "password": "s3cret",
  "db_url": "postgres://app:s3cret@test-db.internal:5432/app"
}
```

### 2.5 Project Config Value

A **Project Config Value** is the per-(project, environment, template) data that drives template rendering. It is the data source passed to Go template execution, stored as a JSON object, and may **reference** Global Values, which are resolved at render time. Exactly one value set exists per (template, environment) pair.

#### Referencing Global Values

A leaf value in the Project Config Value JSON may be a **reference string** of the form:

```
${<global_values_name>.<key>}
```

For example, `${test_db_values.db_url}` resolves to the `db_url` key of the `test_db_values` Global Values entry.

Because Global Values are flat, the path after the dot is always a single key — no deeper traversal is needed or supported.

Example Project Config Value:

```json
{
  "service_name": "billing",
  "env": "staging",
  "db_url": "${test_db_values.db_url}",
  "db_password": "${test_db_values.password}",
  "feature_flags": {
    "new_checkout": true,
    "legacy_invoices": false
  }
}
```

Resolution rules:

- A reference may appear anywhere a leaf string value would appear in the JSON tree, at any depth.
- `${name.key}` is replaced with the scalar value at `key` inside the Global Values entry named `name`.
- An unknown name or unknown key is a render-time error.
- References resolve to scalars only; since Global Values are flat, a reference cannot expand into an object or array.

---

## 3. Generation Flow

Given an `(environment, project)` pair, the system generates one rendered config per template owned by that project. Per template:

1. **Lookup template.** Load the Project Config Template.
2. **Lookup values.** Load the Project Config Value for `(template, environment)`. If none exists, generation for that template fails (or is skipped, depending on caller policy).
3. **Resolve references.** Walk the values JSON; for every `${name.key}` reference, substitute the corresponding Global Values scalar.
4. **Render.** Execute the Go template with the fully-resolved JSON object as the template's `.` (dot) data context.
5. **Return** the rendered text.

---

## 5. End-to-End Example

### 5.1 Setup

**Global Values** — `test_db_values`:

```json
{
  "host": "test-db.internal",
  "port": 5432,
  "username": "app",
  "password": "s3cret"
}
```

**Project**: `billing-service`

**Project Config Template** — `app.yaml`:

```yaml
service: {{ .service_name }}
environment: {{ .env }}
database:
  host: {{ .db_host }}
  port: {{ .db_port }}
  user: {{ .db_user }}
  password: {{ .db_password }}
features:
{{- range $k, $v := .feature_flags }}
  {{ $k }}: {{ $v }}
{{- end }}
```

**Project Config Value** for `(app.yaml, staging)`:

```json
{
  "service_name": "billing",
  "env": "staging",
  "db_host": "${test_db_values.host}",
  "db_port": "${test_db_values.port}",
  "db_user": "${test_db_values.username}",
  "db_password": "${test_db_values.password}",
  "feature_flags": {
    "new_checkout": true,
    "legacy_invoices": false
  }
}
```

### 5.2 Resolved data passed to the template

```json
{
  "service_name": "billing",
  "env": "staging",
  "db_host": "test-db.internal",
  "db_port": 5432,
  "db_user": "app",
  "db_password": "s3cret",
  "feature_flags": {
    "new_checkout": true,
    "legacy_invoices": false
  }
}
```

### 5.3 Rendered output

```yaml
service: billing
environment: staging
database:
  host: test-db.internal
  port: 5432
  user: app
  password: s3cret
features:
  new_checkout: true
  legacy_invoices: false
```

---

## 6. Error Cases

- **No Project Config Value for `(template, environment)`** → fail with `ErrMissingValues`.
- **Reference to unknown Global Values name** → fail with `ErrUnknownGlobalValues`.
- **Reference to unknown key inside a Global Values entry** → fail with `ErrUnknownKey`.
- **Go template parse error** → fail with `ErrTemplateParse`.
- **Go template execution error** (missing key, etc.) → fail with `ErrTemplateExec`.