# config-generation

A web platform for generating environment-specific configuration files.  
Projects define [Go template](https://pkg.go.dev/text/template) files and per-environment values; the system renders them on demand. Shared values (e.g. database credentials) are defined once as **Global Values** and referenced across multiple projects. Changes go through a **pull-request review flow** with configurable approval conditions before being applied.

- **Backend** — Go (`backend/`)
- **Frontend** — React + TypeScript (`frontend/`)
- **Local dev** — `docker-compose.yml` (backend + frontend + Postgres)

## Continuous Delivery

Every push to `main` triggers the **CD Build & Push** workflow (`.github/workflows/cd-build-push.yml`):

```
push to main
    │
    ├─ Build & push Docker images → ghcr.io/brian030128/config-generation/{backend,frontend}:<commit-sha>
    │
    ├─ Staging  → commits updated image tags directly to
    │             solar224/config-generation-gitops  charts/config-gen/values-staging.yaml
    │             ArgoCD auto-syncs → staging.ycantech.com
    │
    └─ Production → opens a PR in the GitOps repo
                    charts/config-gen/values-production.yaml
                    A human merges the PR → ArgoCD auto-syncs → app.ycantech.com
```

Please remember
```
cp .env.example .env
```

Infrastructure and Kubernetes manifests live in the GitOps repository:  
**[solar224/config-generation-gitops](https://github.com/solar224/config-generation-gitops)**

### Required secrets (in this repo's GitHub Actions settings)

| Secret | Purpose |
|--------|---------|
| `GITOPS_TOKEN` | PAT with `contents: write` on the GitOps repo (to push tag updates and open PRs) |
