# CI/CD

This project uses GitHub Actions for quality checks, image publishing, and
deployment markers.

## Existing Quality Gates

The repository already has PR workflows for:

- Backend format, lint, tests, and build.
- Frontend install, lint, type check, and build.
- Docker build dry runs for Dockerfile and compose changes.
- Scheduled Trivy repository scans.

Branch protection should require the relevant PR checks before merging to
`main`. The delivery workflows assume `main` only receives reviewed changes.

## Image Publishing

`Publish Images` runs when `main` is updated, when a `v*.*.*` tag is pushed, or
when manually dispatched.

It publishes two GHCR images:

- `ghcr.io/<owner>/config-generation-backend`
- `ghcr.io/<owner>/config-generation-frontend`

Images are tagged with immutable `sha-<commit>` tags. Main builds also receive
the `main` tag, and release builds receive the Git tag.

## Deployment Markers

`Deploy` records GitHub deployment metadata after image publishing:

- Staging is marked automatically after a successful `main` image publish.
- Production is marked manually through `workflow_dispatch`.

The workflow resolves image digests before recording the deployment payload, so
downstream runtime deployment can use immutable image references.

## Runtime Deploy Contract

The current workflow does not roll out containers to a VM, Kubernetes cluster,
or external deployment platform. When a runtime target is added, keep these
rules:

- Deploy by image digest, not by a mutable tag.
- Run database migrations before replacing the backend container.
- Stop rollout if migrations fail.
- Check backend readiness after rollout.
- Mark the GitHub deployment failed if rollout or health checks fail.
