<div align="center">

# config-generation

**Template-driven configuration management with PR-gated deployments**

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://golang.org)
[![React](https://img.shields.io/badge/React-TypeScript-61DAFB?logo=react&logoColor=black)](https://react.dev)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)](https://docs.docker.com/compose)
[![ArgoCD](https://img.shields.io/badge/ArgoCD-GitOps-EF7B4D?logo=argo&logoColor=white)](https://argo-cd.readthedocs.io)

</div>

---

Define [Go templates](https://pkg.go.dev/text/template), bind per-environment values, and let teams propose changes through a pull-request review flow before anything is applied.

## Demo

<video src="https://github.com/user-attachments/assets/60f745d5-e1a8-4513-b868-545bbfb933e5" controls width="100%"></video>

## Features

| | |
|---|---|
| **Go template rendering** | Write `.tmpl` files; values are filled per environment on demand |
| **Global Values** | Define shared credentials once, reference them across all projects |
| **PR review flow** | Changes require approval with configurable conditions per project |
| **RBAC** | Role-based permissions with project-scoped membership |
| **OIDC / local auth** | SSO via Dex or username/password login |

## Quick Start

```bash
cp .env.example .env
docker compose up
```

| Service | URL |
|---|---|
| Frontend | http://localhost:5173 |
| Backend API | http://localhost:8080 |

## Continuous Delivery

Every push to `main` triggers the CD pipeline:

```
push to main
    ├─ Build & push → ghcr.io/brian030128/config-generation/{backend,frontend}:<sha>
    ├─ Staging  → updates values-staging.yaml → ArgoCD auto-syncs → staging.ycantech.com
    └─ Production → opens a PR against values-production.yaml → merge to deploy → app.ycantech.com
```

> Infrastructure & Kubernetes manifests: [solar224/config-generation-gitops](https://github.com/solar224/config-generation-gitops)

### Required GitHub Secrets

| Secret | Purpose |
|---|---|
| `GITOPS_TOKEN` | PAT with `contents:write` on the GitOps repo |
