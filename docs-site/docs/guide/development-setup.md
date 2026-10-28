---
title: Development setup
description: Run the backend, judge and frontend from source against containerized dependencies.
---

# Development setup

This page sets up a local development environment where you edit and restart one
service at a time. Use it when you are changing the code. To try the complete
platform without a toolchain, use [Getting started](./getting-started.md); to
deploy it, use [Deployment](../deployment/index.md).

## Prerequisites

| Tool | Version | Used for |
| --- | --- | --- |
| Go | 1.26 for the backend, 1.27 for the judge | backend and judge |
| Bun | current release | frontend install, dev server and tests |
| Docker Compose v2 or Podman | current release | PostgreSQL, Redis, SeaweedFS and smtp4dev |
| Rootless Podman | current release | the judge's execution runtime |
| `make` | any | judge targets and observability helpers |
| Node.js | 20 or 22 | the documentation site only |

The backend and judge use separate Go modules and separate minimum versions.
Check `backend/go.mod` and `judge/go.mod` if your toolchain is older.

## Get the code

```bash
git clone https://github.com/reisaraujo-miguel/runcodes.git
cd runcodes
```

Install the frontend dependencies once. This also enables the repository's Git
hooks, including the SVG check:

```bash
cd frontend
bun install
cd ..
```

If you prefer not to install dependencies, enable the hooks directly:

```bash
git config core.hooksPath .githooks
```

## Choose how to run the stack

### Full stack in containers

```bash
cp .env.example .env
# fill the required RUNCODES_* secrets
docker compose up --build
```

This is the closest to production and needs no local Go or Bun toolchain. Use it
to verify a change end to end, not for the edit/restart loop.

### Dependencies in containers, services from source

Create a root `.env` from `.env.example` and fill the required secrets before
starting anything. Compose validates the required values even when you start
only some services.

```bash
cp .env.example .env
# fill the required RUNCODES_* secrets
docker compose up -d database seaweed redis smtp4dev
docker compose ps
```

The database container initializes the schema, the seed data, the
`runcodes_app` role and the `runcodes_judge` role on an empty volume. If you
change `RUNCODES_DB_APP_PASSWORD` or `RUNCODES_DB_JUDGE_PASSWORD` after the volume
exists, the roles keep the old passwords. Recreate the volume for a disposable
database, or change the passwords with SQL.

## Run the backend

```bash
cd backend
cp .env.example .env
```

Edit `backend/.env` for a backend on the host:

```bash
RUNCODES_JWT_SECRET='a-long-random-value'
RUNCODES_API_PORT=8443

RUNCODES_DB_HOST=127.0.0.1
RUNCODES_DB_PORT=5432
RUNCODES_DB_USER=runcodes_app
RUNCODES_DB_PASSWORD='the value of RUNCODES_DB_APP_PASSWORD'
RUNCODES_DB_NAME=runcodes
RUNCODES_DB_SSLMODE=disable

RUNCODES_JUDGE_URL=http://127.0.0.1:9000
RUNCODES_JUDGE_TOKEN='the value of RUNCODES_JUDGE_TOKEN'

RUNCODES_S3_ENDPOINT=http://127.0.0.1:8333
RUNCODES_S3_CREDENTIALS_KEY='the value of RUNCODES_S3_CREDENTIALS_KEY'
RUNCODES_S3_CREDENTIALS_SECRET='the value of RUNCODES_S3_CREDENTIALS_SECRET'
RUNCODES_S3_BUCKET_PREFIX=runcodes

RUNCODES_REDIS_ADDR=127.0.0.1:6379
RUNCODES_REDIS_PASSWORD='the value of RUNCODES_REDIS_PASSWORD'

RUNCODES_SMTP_HOST=127.0.0.1
RUNCODES_SMTP_PORT=1025
RUNCODES_SMTP_FROM='RunCodes <no-reply@runcodes.local>'
RUNCODES_PUBLIC_URL=http://localhost:5173
```

The backend loads `.env` from its working directory. Start it in debug mode:

```bash
go run . -debug
```

`-debug` enables human-readable logs and a non-`Secure` session cookie, which is
required on plain HTTP. The `.env.example` also documents the optional
OpenTelemetry and pprof variables.

The backend connects as `runcodes_app`, not as the database owner. Use the owner
role only for migrations and the admin bootstrap.

## Run the judge

The judge does not load a `.env` file itself. Export the values before `make run`,
or source a file:

```bash
systemctl --user start podman.socket
ls -l /run/user/$(id -u)/podman/podman.sock

mkdir -p exec
chmod 0777 exec
```

Create `judge/.env` from the example and export it:

```bash
cd judge
cp .env.example .env
# edit the file: database, S3, Podman socket, exec path, auth token
set -a
. ./.env
set +a
make run
```

Values that must match the backend and the host:

```bash
JUDGE_AUTH_TOKEN='the value of RUNCODES_JUDGE_TOKEN'
JUDGE_PODMAN_URI="unix:///run/user/$(id -u)/podman/podman.sock"
JUDGE_EXEC_DIR="$PWD/../exec"
JUDGE_EXEC_DIR_REMOTE="$PWD/../exec"

RUNCODES_DB_HOST=127.0.0.1
RUNCODES_DB_PORT=5432
RUNCODES_DB_USER=runcodes_judge
RUNCODES_DB_PASSWORD='the value of RUNCODES_DB_JUDGE_PASSWORD'
RUNCODES_DB_NAME=runcodes

RUNCODES_S3_ENDPOINT=http://127.0.0.1:8333
RUNCODES_S3_CREDENTIALS_KEY='the value of RUNCODES_S3_CREDENTIALS_KEY'
RUNCODES_S3_CREDENTIALS_SECRET='the value of RUNCODES_S3_CREDENTIALS_SECRET'

# Runner images (defaults shown)
JUDGE_IMAGE_REGISTRY=ghcr.io
JUDGE_RUNNER_IMAGE_PREFIX=runcodes-icmc/runcodes-runner
JUDGE_RUNNER_IMAGE_TAG=latest
```

`JUDGE_EXEC_DIR_REMOTE` is the path as seen by the Podman service. On a local
rootless Podman socket that is the same absolute path. The directory must be
writable by the user that runs the judge and by the submitted containers.

Check readiness:

```bash
curl -s http://localhost:9000/readyz | jq
```

The judge pulls
`<JUDGE_IMAGE_REGISTRY>/<JUDGE_RUNNER_IMAGE_PREFIX>-<language>:<JUDGE_RUNNER_IMAGE_TAG>`
on the first run. With the defaults, pull an image ahead of a demo with:

```bash
podman pull ghcr.io/runcodes-icmc/runcodes-runner-c:latest
```

If you set a custom repository or tag, pull the matching reference (for example
`ghcr.io/reisaraujo-miguel/runcodes-runner-c:v1.2.3`), or run a submission once.

The Makefile supplies the Podman build tags. Calling `go run .` without them may
fail to compile the Podman client.

## Run the frontend

```bash
cd frontend
cp .env.example .env
```

For the Vite dev server, set:

```bash
VITE_API_ENDPOINT=http://localhost:8443
VITE_CONTACT_EMAIL=contact@example.com
VITE_OTEL_ENABLED=false
```

Then:

```bash
bun run dev
```

The dev server listens on <http://localhost:5173>. The backend must allow that
origin through `FRONTEND_ORIGIN`; the default is already
`http://localhost:5173`.

If you change `VITE_*` values, restart the dev server. Vite reads them at build
start, not on every reload.

## Local URLs

| Service | URL | Notes |
| --- | --- | --- |
| Frontend dev server | <http://localhost:5173> | Vite, hot reload |
| Backend | <http://localhost:8443> | direct API |
| Judge | <http://localhost:9000> | readiness and internal API |
| SeaweedFS | <http://localhost:8333> | S3 endpoint, loopback |
| smtp4dev | <http://localhost:8081> | caught mail; SMTP on 1025 |
| PostgreSQL | `127.0.0.1:5432` | owner and service roles |

## Bootstrap the admin and verify an account

The seed creates `admin@admin.com` with an unusable password hash. Set a password
before the first login:

```bash
RUNCODES_ADMIN_PASSWORD='choose-a-strong-password' \
  PGHOST=127.0.0.1 PGPORT=5432 PGUSER=runcodes PGDATABASE=runcodes \
  PGPASSWORD="$RUNCODES_DB_PASSWORD" ./database/bootstrap-admin.sh
```

`RUNCODES_DB_PASSWORD` here is the **owner** password from the root `.env`.

Then sign up a normal account in the frontend, open the verification mail in
smtp4dev, and sign in. The full first-run flow is in
[Getting started](./getting-started.md#_4-create-and-verify-an-account).

## Run the checks

Run the checks for the service you changed. The full matrix is in
[Contributing](../contributing.md#making-a-change).

```bash
# Backend
(cd backend && gofmt -l . && go vet ./... && go test -count=1 -race ./...)

# Judge
(cd judge && make test && make vet)

# Frontend
(cd frontend && bun run lint && bun run check-svg && bun test && bun run build)

# Monitor and harness
./monitor/test/monitor.test.sh
./runners/test/harness.test.sh

# Documentation
(cd docs-site && npm run build)
```

The judge integration test starts real PostgreSQL and SeaweedFS containers and
runs a real language image:

```bash
cd judge
make integration
```

It needs the rootless Podman socket and network access to pull images.

## Work with the database

The schema is the desired state in `database/schema/02-schema.sql`; Atlas
migrations move an existing database to it.

```bash
export DATABASE_URL="postgres://runcodes:${RUNCODES_DB_PASSWORD}@127.0.0.1:5432/runcodes?sslmode=disable"
./database/atlas.sh migrate status --url "$DATABASE_URL"
./database/atlas.sh migrate apply  --url "$DATABASE_URL"
```

`RUNCODES_DB_PASSWORD` here is the **owner** password from the root `.env`, not
the app role password. The app role cannot change the schema.

For a schema change, follow [Database § Migrations](../services/database.md#migrations):
edit the schema, generate a migration against a disposable database, review it,
apply it, and commit the schema, migration and `atlas.sum` together.

A fresh Compose volume already has the current schema but no Atlas history. Stamp
it before the first migration. The commands are in
[Database § Existing databases](../services/database.md#existing-databases).

## Reset local data

This deletes the database, object store, cache and caught mail. It cannot be
undone.

```bash
docker compose down -v
docker compose up -d database seaweed redis smtp4dev
```

The next database start initializes a fresh volume. Run the admin bootstrap again
if you need the seeded account.

## Debug a request

### Backend

The backend logs one line per request. In debug mode you can ask it to log the
request and response bodies for a single request:

```bash
curl -H 'Debug: reveal-body-logs' ...
```

The header only works when the backend runs in debug mode. Do not enable it in
production: bodies contain credentials and submitted source.

Every response has a `TraceId` header. When the observability stack is running,
use it in Tempo or Loki to follow the request through the backend and judge. See
[Local observability stack](../observability/local.md).

### Judge

Set `JUDGE_KEEP_WORKSPACES=true` to keep a run's workspace for inspection. The
directory contains the source, compilation files, case inputs, outputs and the
monitor report. Remove the workspaces when you finish; they are untrusted data.

### SSE

Follow a submission stream from the command line with a session cookie:

```bash
curl -N -b cookies.txt \
  http://localhost:8443/api/v1/submissions/42/events
```

The stream starts with a `snapshot` event and then relays live events. See
[Submissions & live judging](./submissions.md).

## Common problems

### CORS or a missing session cookie

`FRONTEND_ORIGIN` must be the exact origin of the dev server, including scheme
and port. `http://localhost:5173` and `http://127.0.0.1:5173` are different
origins. Credentialed requests also need a real origin, never `*`.

### The session cookie is not stored

In debug mode the backend does not set `Secure`. If you run with `-debug` off
over plain HTTP, the browser refuses the cookie. Use debug mode locally or put
TLS in front.

### The judge reports `podman unavailable`

The socket did not exist when the podman client tried to connect, or the judge
cannot reach it. Check the path, start `podman.socket`, and confirm the judge
process runs as a user that can open the socket:

```bash
ls -l /run/user/$(id -u)/podman/podman.sock
curl -s http://localhost:9000/readyz | jq
```

The Compose-specific SELinux and uid notes are in
[Getting started](./getting-started.md#connect-permission-denied-on-the-podman-socket).

### S3 signature or bucket errors

The credentials in the backend, the judge and the SeaweedFS config must match.
In Compose, `seaweed-s3-init` generates the gateway config from
`RUNCODES_S3_CREDENTIALS_KEY` and `_SECRET`. If you started SeaweedFS without its
init container, the gateway may not have the same credentials.

### Mail does not arrive

The backend must point at smtp4dev on port 1025 when it runs on the host, or at
`smtp4dev` port 25 inside Compose. The web UI is on 8081. A verification link
uses `RUNCODES_PUBLIC_URL`; if it points at 8080 while you develop on 5173, the
link opens the wrong origin.

### Migrations disagree with the schema

The CI check fails when `database/schema/02-schema.sql` and `database/migrations/`
drift. Generate a migration, do not edit the live database. See
[Database](../services/database.md#migrations).

### Port already in use

Every host port in Compose is overridable in the root `.env`
(`BACKEND_PORT`, `JUDGE_PORT`, `DATABASE_PORT`, `SEAWEED_S3_PORT`,
`SMTP4DEV_WEB_PORT`, `SMTP4DEV_SMTP_PORT`). Change the port and the matching
service URL in the backend or judge `.env`.

### The frontend calls the wrong API

`VITE_API_ENDPOINT` is baked when the dev server starts. Restart `bun run dev`
after changing it. In the container image it is empty on purpose, so the SPA uses
same-origin `/api` through Caddy.

## Related pages

- [Getting started](./getting-started.md)
- [Contributing](../contributing.md)
- [Configuration reference](../reference/configuration.md)
- [Backend API](../services/backend.md)
- [Judge](../services/judge.md)
- [Frontend application](./frontend.md)
- [Database](../services/database.md)
