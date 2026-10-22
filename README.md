# RunCodes

An open-source platform for programming exercises: students submit code, the
platform compiles and runs it against the exercise's test cases in isolated
containers, and the results are streamed back live.

This repository is the next-generation rewrite of the legacy run.codes platform.
The original compiler engine and language images live in separate repositories
(`compiler-engine`, `compiler-images`); they were folded into this one as
`judge/`, `runners/` and `monitor/`.

## Documentation

The full documentation — architecture, the web client and every service, the
HTTP API, both deployment paths, operations and observability — is published at
<https://reisaraujo-miguel.github.io/runcodes/>. It is built with VitePress from
[`docs-site/`](docs-site/); run `npm ci && npm run dev` there to work on it
locally, or read [`docs-site/README.md`](docs-site/README.md).

## Architecture

```mermaid
flowchart TD
    Client["Browser client"]
    Backend["Backend API"]
    DB[("PostgreSQL")]
    S3[("SeaweedFS · S3")]
    Judge["Judge service"]
    Containers["Language containers"]

    Client -->|"REST + SSE"| Backend
    Backend -->|"register commit"| DB
    Backend -->|"upload source"| S3
    Backend -->|"readyz · wake · subscribe"| Judge
    Judge -->|"claim queued commits"| DB
    Judge -->|"read source and test cases"| S3
    Judge -->|"rootless podman"| Containers
    Judge -->|"status and results"| Backend
    Backend -->|"SSE relay"| Client
```

A submission flows through the platform like this:

1. The client uploads a source file to the backend.
2. The backend checks that the judge is healthy, uploads the source to S3 and
   registers a `queued` commit. If the judge is unreachable nothing is
   registered, so a submission is never silently lost.
3. The judge claims queued commits from PostgreSQL (at most
   `JUDGE_CONCURRENCY` at a time), downloads the source and test cases from S3,
   and runs the language container.
4. The judge streams status changes, per-test-case verdicts and the final result
   to the backend, which persists everything and relays it to the client over
   SSE. A reloading client reconnects and replays the persisted state.

## Services

| Service     | Path        | Stack                                     | Description                                      |
| ----------- | ----------- | ----------------------------------------- | ------------------------------------------------ |
| Frontend    | `frontend/` | React 19, Vite, TypeScript, Tailwind, Bun | The web client.                                  |
| Backend API | `backend/`  | Go, chi, PostgreSQL, JWT, S3              | Auth, courses, submissions, SSE relay.           |
| Judge       | `judge/`    | Go, rootless podman, PostgreSQL, S3       | Compiles, runs and grades submissions.           |
| Runners     | `runners/`  | Docker images per language                | The container each submission is graded in.      |
| Monitor     | `monitor/`  | C                                         | Limits, times and reports one graded process.    |
| Database    | `database/` | PostgreSQL                                | Schema, seeds, migrations and legacy import.     |

Each service has its own README: [`backend/README.md`](backend/README.md),
[`frontend/README.md`](frontend/README.md), [`judge/README.md`](judge/README.md),
[`runners/README.md`](runners/README.md),
[`monitor/README.md`](monitor/README.md).
The judge/backend integration contract is documented in
[`judge/DESIGN.md`](judge/DESIGN.md); the judge/image contract (the milestone
nonce and the per-case limits) is in the same file.

## Repository layout

```
.
├── backend/        Go API (auth, offerings, submissions, SSE relay)
├── frontend/       React client served by Caddy
├── judge/          Execution engine (rootless podman)
├── runners/        Language images the judge runs submissions in
├── monitor/        In-container process monitor (limits, timing, report)
├── database/       PostgreSQL schema, seeds, migrations and legacy import
├── observability/  Collector, Prometheus, Loki, Tempo, Grafana, Pyroscope and Alertmanager config
├── docs/           Observability architecture and runbooks
├── docs-site/      The documentation website (VitePress, GitHub Pages)
├── scripts/        Developer and observability validation scripts
├── docker-compose.yml
└── docker-compose.observability.yml
```

## Getting started

The whole stack runs with Docker Compose (or `podman compose`). The judge
additionally needs a **rootless podman** API socket on the host and a directory
shared with it (see [`judge/README.md`](judge/README.md)); the repository ships
`./exec` and the one-shot `judge-exec-init` service prepares it, so no manual
setup is required there:

```bash
# Rootless podman API socket used by the judge. Start it *before* `docker compose
# up`: the socket is bind-mounted into the judge container when the container is
# created, so a container created first keeps a dead mount (submit a solution and
# the judge logs "podman unavailable" instead of running it). If you restart the
# socket later, recreate the judge: `docker compose up -d --force-recreate judge`.
# See judge/README.md.
systemctl --user start podman.socket

# Required secrets. Compose refuses to start without them: a deployment that
# falls back to a committed or empty default has either a forgeable session key
# or an unauthenticated judge.
export RUNCODES_JWT_SECRET='change-me'
# Two database passwords: `RUNCODES_DB_PASSWORD` belongs to the owner (used by the
# database container and by migrations), `RUNCODES_DB_APP_PASSWORD` to the
# `runcodes_app` role the backend and the judge log in as.
export RUNCODES_DB_PASSWORD='change-me'
export RUNCODES_DB_APP_PASSWORD='change-me'
# Shared bearer token between the backend and the judge. The judge refuses every
# /v1 request while it is unset (see judge/README.md for the local-only escape
# hatch, JUDGE_ALLOW_INSECURE).
export RUNCODES_JUDGE_TOKEN='change-me'
# Credentials for the S3-compatible store that holds submissions and answers.
export RUNCODES_S3_CREDENTIALS_KEY='change-me'
export RUNCODES_S3_CREDENTIALS_SECRET='change-me'

docker compose up --build
```

| Service   | URL                                                             |
| --------- | --------------------------------------------------------------- |
| Frontend  | `http://localhost:8080` (serves the SPA and proxies `/api/*`)   |
| Backend   | reached through the frontend proxy at `/api` (loopback `:8443`) |
| Judge     | `http://localhost:9000` (loopback only)                         |
| SeaweedFS | `http://localhost:8333` (loopback only)                         |
| smtp4dev  | `http://localhost:8081` (web UI), `localhost:1025` (SMTP)       |

Only the frontend is published on a network interface. The judge, SeaweedFS and
smtp4dev are bound to loopback: the backend and the judge talk to each other over
the compose network, so none of them needs to be reachable from outside the host.

The database is seeded with an admin account (`admin@admin.com`) that has **no
password**: a hash committed to this repository would be a published credential.
Set one before the first login — the account cannot be used until you do:

```bash
RUNCODES_ADMIN_PASSWORD='...' \
  PGHOST=localhost PGPORT=5432 PGUSER=runcodes PGDATABASE=runcodes \
  PGPASSWORD="$RUNCODES_DB_PASSWORD" ./database/bootstrap-admin.sh
```

### Account verification

New accounts are created **unconfirmed**. Signing up sends a verification link
to the address; until that link is opened, `POST /api/v1/user/login` answers
`403` and the API refuses any protected route, even with a token issued before
the account was unconfirmed. The link points at `/verify-email?token=...` on the
frontend, expires after 24 hours, and can be reissued from the login page (or
`POST /api/v1/user/resend-verification`), which invalidates the previous link.

Locally, no real mail server is needed. The `smtp4dev` service catches every
message and lists it at `http://localhost:8081`:

1. Start the stack (`docker compose up --build`) or just the catcher
   (`docker compose up -d smtp4dev`) if you run the backend with `go run .`.
2. Sign up in the frontend. The backend sends the link to smtp4dev; open
   `http://localhost:8081` and click the message, then its verification link.
3. The `/verify-email` page confirms the account; sign in afterwards.

Compose points the backend at `smtp4dev:25` and builds links for
`http://localhost:8080`. The backend's own `.env` (used by `cd backend && go run .`)
uses `127.0.0.1:1025` and `http://localhost:5173` instead, matching the published
SMTP port and the Vite dev server. All of them are overridable:

| Variable                                            | Purpose                                                  |
| --------------------------------------------------- | -------------------------------------------------------- |
| `RUNCODES_PUBLIC_URL`                               | Origin used in verification and recovery links (no path). |
| `RUNCODES_SMTP_HOST`                                | SMTP server hostname; `smtp4dev` in Compose.             |
| `RUNCODES_SMTP_PORT`                                | SMTP port; defaults to `587`.                            |
| `RUNCODES_SMTP_FROM`                                | `From` address, e.g. `RunCodes <no-reply@example.com>`.  |
| `RUNCODES_SMTP_USERNAME` / `RUNCODES_SMTP_PASSWORD` | Optional relay credentials; leave empty for smtp4dev.    |

A database created before the email features needs the migration applied once (a
fresh Compose volume already contains the tables):

```bash
export DATABASE_URL="postgres://runcodes:${RUNCODES_DB_PASSWORD}@localhost:5432/runcodes?sslmode=disable"
./database/atlas.sh migrate apply --url "$DATABASE_URL"
```

Accounts migrated from the legacy system inherit their old `confirmed` flag, so
an unconfirmed legacy account must be confirmed from the admin panel or by
reissuing its verification link.

### Password recovery

A user who forgot their password can open **Esqueceu a senha?** on the login
page, or call `POST /api/v1/user/forgot-password`. The route always answers
`202`, whether or not the address has an account, so it cannot be used to
enumerate users. The link points at `/reset-password?token=...`, expires after
1 hour, is single use, and only its SHA-256 is stored (`password_reset_tokens`);
requesting a new link invalidates the previous one.

Resetting a password does not confirm an unconfirmed account. That user still
needs a verification link, which the login page offers after a `403`.

### Configuration

Compose reads secrets from the environment (or a root `.env` file, see
`.env.example`). The most relevant variables are `RUNCODES_JWT_SECRET`,
`RUNCODES_DB_PASSWORD`, `RUNCODES_DB_APP_PASSWORD`,
`RUNCODES_LEGACY_PASSWORD_SALT`, `RUNCODES_JUDGE_TOKEN`, the `RUNCODES_S3_*`
settings and the mail variables described above. All but the legacy salt are
required and compose fails fast when one is missing. Every service documents its
own variables in its `.env.example`.

## Deployment

A four-machine Ansible deployment, including the full observability stack, lives
in [`deploy/ansible/`](deploy/ansible/README.md). It uses four example node
addresses and rootless Podman for every service stack and graded runner. It
separates the public edge/backend, the stateful PostgreSQL/Redis/SeaweedFS host,
the judge host and a dedicated telemetry host.
The deployment pulls published application images from GHCR rather than building
on the hosts; deployment values live in an untracked `config.yml`. The topology
and rationale are documented in
[`deploy/ansible/docs/ARCHITECTURE.md`](deploy/ansible/docs/ARCHITECTURE.md).

## Development

Run each service locally against the Compose-provided database, SeaweedFS and
podman socket:

```bash
# Backend (needs a .env or environment variables)
cd backend && go run . -debug

# Judge
cd judge && make run

# Frontend
cd frontend && bun install && bun run dev
```

### Database changes

`database/schema/02-schema.sql` is the desired schema and
`database/migrations/` holds the versioned migrations that move an existing
database to it. Generate a migration after editing the schema instead of
changing a live database by hand:

```bash
ATLAS_DEV_URL='postgres://runcodes:PASSWORD@localhost:5432/atlas_dev?sslmode=disable&search_path=public' \
  ./database/atlas.sh migrate diff add_the_column
```

See [`database/README.md`](database/README.md) for the disposable dev database,
applying migrations and baselining an existing database. CI fails when the
schema and the migration directory disagree.

## Observability

The platform ships an opt-in OpenTelemetry stack: the backend and judge emit
traces, metrics and structured logs, and the browser SPA emits fetch traces, all
over OTLP to an OpenTelemetry Collector. The collector writes traces to Tempo,
logs to Loki and exposes application metrics for Prometheus. Judge queue gauges and per-test-case spans are included.
Grafana provides dashboards, Explore, trace/log correlation and span-to-profile
links; Alloy scrapes the Go pprof endpoints into Pyroscope, and the
`otel-profiling-go` bridge adds trace/span labels so a span opens its matching
flame graph. Alertmanager routes alerts from Prometheus rules (plus a
Grafana-managed example rule) using a secret-backed receiver URL.

The overlay is separate from the normal stack and publishes every UI on
loopback only:

```bash
make observability-init       # generate the local Grafana admin password
make observability-up         # start app + Prometheus, Loki, Tempo, Grafana, ...
make observability-smoke-test # generate traffic and validate every signal
make observability-browser-smoke-test # real Chromium trace (install once: cd frontend && bun run test:e2e:install)
SMOKE_ADMIN_PASSWORD='...' make observability-submission-smoke-test # authenticated submission + per-case span
```

| URL | Purpose |
| --- | --- |
| <http://localhost:3000> | Grafana (dashboard `runcodes-overview`) |
| <http://localhost:9090> | Prometheus |
| <http://localhost:9093> | Alertmanager |
| <http://localhost:3100/ready> | Loki readiness |
| <http://localhost:3200/ready> | Tempo readiness |
| <http://localhost:4040/ready> | Pyroscope readiness |
| <http://localhost:12345> | Alloy UI/health |

The Grafana password is generated into `.observability/` (gitignored) and is
never committed. `make observability-down` keeps the telemetry volumes;
`make observability-destroy` removes them.

See [`docs/observability/README.md`](docs/observability/README.md) for the
architecture, data flow, ports, configuration, retention and production
guidance, and [`docs/observability/runbook.md`](docs/observability/runbook.md)
for start/stop, validation, failure modes, alert testing and adding services.
The browser and submission checks are optional and need Playwright Chromium and
an admin password, respectively.

## Releases

Three image families are consumed by name and are published to GHCR on a
`v*.*.*` tag:

| Workflow                          | Publishes                                                                     |
| --------------------------------- | ----------------------------------------------------------------------------- |
| `.github/workflows/app-images.yml` | `ghcr.io/runcodes-icmc/runcodes-frontend`, `-backend`, `-database`, `-judge` |
| `.github/workflows/monitor.yml`   | `ghcr.io/runcodes-icmc/runcodes-monitor`                                      |
| `.github/workflows/runners.yml`   | `ghcr.io/runcodes-icmc/runcodes-runner-base` and `runcodes-runner-<language>` |

All three workflows also move `:latest` on every tag; the four-machine
deployment defaults to it, while the version tags exist for pinning. The
per-language Dockerfiles, the judge's default `JUDGE_IMAGE_FORMAT` and the
integration test read the runner `:latest` tag.
`runners.yml` builds `base` before the languages, since 12 of them inherit from
it and 11 copy the harness and the monitor out of it.

That makes the judge, the monitor and the language images one contract (the
milestone nonce and the per-case limits, documented in
[`judge/DESIGN.md`](judge/DESIGN.md)): changing it means releasing the judge and
rebuilding the images in `runners/` together, or the runs of the new judge
fail against images that are still on the old base script.

The application images (backend, frontend, database, judge) are built and
scanned from source by `.github/workflows/images.yml` and published by
`.github/workflows/app-images.yml` for the four-machine deployment. The local
development `docker-compose.yml` still builds them from source; the Ansible
deployment pulls the published images instead. The frontend image is built with
the same-origin `/otel` browser-telemetry endpoint, so no deployment hostname is
baked into the SPA.

## Security

API surface: only the frontend is published on a network interface. The backend,
the judge, SeaweedFS and smtp4dev are bound to loopback, and the judge refuses
its API unless `RUNCODES_JUDGE_TOKEN` is set. The observability overlay publishes
its UIs/APIs on loopback only; the Go pprof listeners are internal and are not
published at all. Graded containers get no network
namespace, are capped in memory, PIDs and CPU (`JUDGE_CONTAINER_*`), and cannot
gain privileges through a setuid binary in an image (`no-new-privileges`).

Database: the backend and the judge log in as `runcodes_app`, a role that can
read and write rows and **cannot** change the schema, create roles or read
`pg_authid` (`database/schema/05-app-role.sh`, asserted by CI). Only the database
container and the migration tooling use the superuser the postgres image creates.

Container images and the repository are scanned with [Trivy](https://trivy.dev/)
(configuration in `trivy.yaml`, accepted risks in `.trivyignore`) by the
`Trivy Security Scan` workflow and via `docker compose` builds.

## License

RunCodes is distributed under the **GNU Affero General Public License v3.0** —
see [LICENSE](LICENSE).

## Contributing

Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) and
see [CONTRIBUTORS.md](CONTRIBUTORS.md) for the people behind the project.
