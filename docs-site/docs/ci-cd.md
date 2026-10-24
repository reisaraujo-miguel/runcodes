---
title: CI/CD & releases
description: GitHub Actions workflows, test gates, image publishing, versioning and the docs deployment.
---

# CI/CD & releases

All automation lives in `.github/workflows/`. This page describes what runs, when
it runs, and what a release publishes.

## Workflows

| Workflow | Trigger | Purpose |
| --- | --- | --- |
| `tests.yml` | push to `main`/`development`, all PRs, manual | formatting, vet, unit/integration tests, schema and migration checks |
| `judge-integration.yml` | judge/runner/monitor/schema paths, manual | real PostgreSQL + SeaweedFS + rootless Podman end-to-end grading test |
| `images.yml` | push to `main`/`development`, PRs, weekly, manual | build the four application images and scan them with Trivy |
| `app-images.yml` | `v*.*.*` tag, manual | publish frontend, backend, database and judge to GHCR |
| `monitor.yml` | `v*.*.*` tag, manual | publish the monitor image |
| `runners.yml` | `v*.*.*` tag, manual | publish the base image and 22 language images |
| `observability.yml` | telemetry-related paths on push/PR, manual | validate collector/Prometheus/Alertmanager/Grafana config |
| `trivy-repo.yml` | push to `main`, PRs, weekly, manual | scan the repository (report-only) |
| `docs.yml` | push to `main` touching `docs-site/`, manual | build this site and deploy it to GitHub Pages |

Dependabot updates Bun, Go modules, Dockerfiles, GitHub Actions and
docker-compose files weekly.

## Tests

`tests.yml` runs on every push to `main`/`development` and on every pull request.
The jobs:

### Backend (Go)

- `gofmt -l` check;
- `go vet ./...`;
- `go test -count=1 -race ./...`.

### Judge (Go)

- `gofmt` and `go vet` with the Podman remote/storage tags;
- `go test -race`; the database-backed end-to-end test lives in the separate
  `judge-integration.yml` workflow rather than being silently skipped here.

### Judge end-to-end

`judge-integration.yml` runs when the judge, monitor, runner or schema changes.
It starts rootless Podman, builds the monitor, base, C, R and Verilog runner
images from the checkout, starts PostgreSQL and SeaweedFS through
`judge/integration/run.sh`, and runs the real
`TestJudgeRunsLanguageImageEndToEnd` path once per language. The harness sets
`JUDGE_TEST_REQUIRE=1`, so an unavailable database, object store or Podman API
fails the job instead of producing a skipped test. Hosted runners have no
systemd user session/cgroup delegation, so the workflow sets
`JUDGE_TEST_DISABLE_CGROUPS=1` for the run; the cgroup-resource spec remains
covered by the podman unit tests.

### Frontend (Bun)

- `bun install --frozen-lockfile` (Bun pinned to the lockfile version);
- ESLint;
- SVG optimization check;
- `bun test`;
- production build.

### Monitor & runners (C, shell)

- `./monitor/test/monitor.test.sh` against real child processes;
- `./runners/test/harness.test.sh`, which asserts the nonce, config deletion,
  per-case limits, daemon isolation and global-limit precedence.

### Database (schema)

Starts PostgreSQL, applies `01-init.sql`, `02-schema.sql`, `03-seed-data.sql`
and `05-app-role.sh`, then asserts both directions of the role:

- `runcodes_app` can read and write rows;
- it **cannot** create/drop/alter tables, create roles or read `pg_authid`.

### Database (migrations)

- verifies `atlas.sum`;
- replays every migration in a disposable database;
- regenerates a diff and fails if `schema/02-schema.sql` changed without a
  matching migration;
- applies migrations to an empty database, runs the seed and app-role setup, and
  asserts the app role cannot read the migration history.

## Image builds and scanning

`images.yml` builds `backend`, `frontend`, `database` and `judge` with Buildx on
pushes, PRs, a weekly schedule and manual dispatch. Each image is loaded locally
and scanned by Trivy (`vuln`, `secret`, `misconfig`).

`trivy.yaml` sets:

- `exit-code: 1` for the image gate;
- severities `CRITICAL` and `HIGH`;
- scanners `vuln`, `secret`, `misconfig`;
- `ignore-unfixed: false` and the accepted-risk file `.trivyignore.yaml`.

Findings are uploaded as SARIF to the Security tab. The repository scan
(`trivy-repo.yml`) overrides the exit code to 0: findings are reported, not
blocking.

`docker compose` builds are covered by the same configuration. Base-image CVEs are
caught by the weekly schedule even without code changes.

## Releases

A release is a `v*.*.*` tag. Three workflows react to it:

### `app-images.yml`

Publishes:

- `ghcr.io/runcodes-icmc/runcodes-frontend`
- `ghcr.io/runcodes-icmc/runcodes-backend`
- `ghcr.io/runcodes-icmc/runcodes-database`
- `ghcr.io/runcodes-icmc/runcodes-judge`

Tags: semver (`1.2.3`, `1.2`, `1`), `sha-<short>` and `latest`. All are
`linux/amd64`. The frontend is built with:

```text
VITE_API_ENDPOINT=
VITE_OTEL_ENABLED=true
VITE_OTEL_EXPORTER_OTLP_ENDPOINT=/otel
VITE_OTEL_SERVICE_NAME=runcodes-frontend
VITE_OTEL_SERVICE_VERSION=ci
VITE_DEPLOYMENT_ENVIRONMENT=production
VITE_OTEL_TRACES_SAMPLER_ARG=1.0
```

The relative `/otel` endpoint means the image works behind the deployment's
same-origin Caddy route without a baked-in hostname.

### `monitor.yml`

Publishes `ghcr.io/runcodes-icmc/runcodes-monitor` with the same tag scheme,
including `:latest`. The base runner image copies the monitor binary out of
`:latest`, so the moving tag is the one in use.

### `runners.yml`

Publishes `runcodes-runner-base` first, then one job per language:

```text
c cpp c-omp c-omp-mpi cpp-omp cpp-omp-mpi dotnet fortran go haskell
java julia lua octave pascal portugol prolog python r rust verilog zig
```

The language jobs depend on the base job and use a fail-fast-disabled matrix, so
a partial release is visible as a full matrix rather than stopping at the first
failure. Images are `linux/amd64`.

## The judge ⇄ runner contract

The judge, the monitor and the language images are one contract:

- `container.config` keys (`run_nonce`, `t_src`, `t_<case>`, `ms_<case>`,
  `fs_<case>`, `stack_<case>`, `monitor_max_*`, `compilation_timeout`);
- milestone format `<name> <run_nonce>`;
- monitor report fields;
- the user shape (`USER runcodes`, `$HOME=/root`).

A change to any of them requires a judge release **and** a rebuild of the base,
every language image and (if it changed) the monitor. Deploying a new judge
against old images logs nonce-less milestones as warnings and the runs time out.

## Versioning and pinning

- The deployment defaults to `latest` for application and runner images.
- The version tags exist for reproducible deployments: set
  `config_runcodes_image_tag` and `config_runcodes_runner_tag` to a release.
- Application and runner releases should be coordinated; a mismatched runner tag
  is the most common cause of a judge that cannot grade.
- Keep previous tags in GHCR for rollback.

## GitHub Pages documentation

`docs.yml` builds this site with VitePress and deploys the static output with
`actions/upload-pages-artifact` + `actions/deploy-pages`.

- Base path: `DOCS_BASE` (default `/runcodes/`, set by the workflow).
- Build: `npm ci && npm run build` in `docs-site/`.
- Permissions: `contents: read`, `pages: write`, `id-token: write`.
- Environment: `github-pages`.
- Concurrency: one Pages deployment at a time, not cancelled.
- Pull requests build the site without deploying, so a broken link or build fails
  review before it reaches the live site.

See [Documentation site](./documentation.md) for local development and editing.

## Adding a workflow or check

- Pin third-party actions to a commit SHA, as the existing workflows do;
  Dependabot updates them.
- Use the least privilege: most jobs need only `contents: read`.
- Add path filters where a check is expensive and unrelated changes are common
  (the observability workflow is a good example).
- Keep the job name stable; branch protection and dashboards may refer to it.
- Update this page when you add a gate that can block a merge.

## Related pages

- [Contributing](./contributing.md)
- [Documentation site](./documentation.md)
- [Security & scaling](./deployment/security.md)
- [Runners & monitor](./services/runners-and-monitor.md)
