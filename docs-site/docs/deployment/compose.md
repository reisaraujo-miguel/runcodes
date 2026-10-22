---
title: Docker Compose deployment
description: Run RunCodes on a single host with Docker Compose, including TLS, volumes, updates and backups.
---

# Docker Compose deployment

`docker-compose.yml` runs the full application stack on one host: Caddy and the
SPA, the backend, the judge, PostgreSQL, Redis, SeaweedFS and smtp4dev. It is
the quickest way to run RunCodes and a reasonable fit for a small single-host
installation.

The observability stack is a **separate opt-in overlay**; see
[Local observability stack](../observability/local.md).

## Services

| Service | Image / build | Published ports | Volumes |
| --- | --- | --- | --- |
| `frontend` | `frontend/` (Caddy + SPA) | `8080:80`, `8444:443` | `caddy_data`, `caddy_config` |
| `backend` | `backend/` (Go) | `127.0.0.1:8443` | — |
| `judge` | `judge/` (Go + Podman client) | `127.0.0.1:9000` | `./exec`, Podman socket |
| `judge-exec-init` | `alpine` | — | `./exec` |
| `database` | `database/` (PostgreSQL) | `127.0.0.1:5432` | `runcodes-db-vol` |
| `redis` | `redis:8-alpine` | — | `runcodes-redis-vol` |
| `seaweed` | `chrislusf/seaweedfs:4.48` | `127.0.0.1:8333` | `runcodes-seaweed-vol` |
| `smtp4dev` | `rnwood/smtp4dev:v3.15` | `127.0.0.1:8081`, `127.0.0.1:1025` | `smtp4dev-data` |

Only the frontend publishes on a network interface. The backend, judge,
database, SeaweedFS and smtp4dev bind to loopback. The backend and judge reach
each other and the data services over the internal `runcodes-network`.

Caddy handles two jobs: it serves the SPA with a client-side-routing fallback,
and it reverse-proxies `/api/*` to `backend:8443`. That keeps the API same-origin
(no CORS) and the SSE stream on one connection.

## Prerequisites

- Docker Engine with Compose v2, or a Docker-compatible API.
- Rootless Podman on the host with `podman.socket` started.
- Root access (or a user in the `docker` group) to run Compose.
- Free disk for the images, the database and the object store.

```bash
systemctl --user start podman.socket
ls -l /run/user/$(id -u)/podman/podman.sock
```

If the socket path is not the default, set `PODMAN_SOCKET` before `docker compose
up`. The judge container mounts it at `/run/podman/podman.sock`.

::: warning Socket lifetime
A bind mount pins the socket that existed when the container was created.
Starting `podman.socket` after the judge, or restarting the socket later,
requires recreating the judge:

```bash
docker compose up -d --force-recreate judge
```
:::

## Configuration

Compose reads secrets from the environment or a root `.env` file. The required
variables are:

```bash
RUNCODES_JWT_SECRET=...
RUNCODES_DB_PASSWORD=...
RUNCODES_DB_APP_PASSWORD=...
RUNCODES_JUDGE_TOKEN=...
RUNCODES_S3_CREDENTIALS_KEY=...
RUNCODES_S3_CREDENTIALS_SECRET=...
```

Optional and commonly set:

| Variable | Default | Meaning |
| --- | --- | --- |
| `RUNCODES_PUBLIC_URL` | `http://localhost:8080` | origin used in verification and recovery links |
| `RUNCODES_SMTP_FROM` | `RunCodes <no-reply@runcodes.local>` | sender address |
| `RUNCODES_SMTP_HOST` / `_PORT` | Compose sets `smtp4dev:25` | real relay |
| `RUNCODES_SMTP_USERNAME` / `_PASSWORD` | empty | relay credentials |
| `RUNCODES_DOMAIN` | `:80` | Caddy site: hostname enables automatic HTTPS |
| `FRONTEND_HTTP_PORT` / `FRONTEND_HTTPS_PORT` | `8080` / `8444` | host ports |
| `PODMAN_SOCKET` | `/run/user/1000/podman/podman.sock` | rootless socket path |
| `JUDGE_UID` / `JUDGE_GID` | `0` / `0` | container identity for the judge |
| `CONTACT_EMAIL` / `CONTACT_DISCLAIMER` | example values | build-time fallbacks only |

The full list is in [Configuration](../reference/configuration.md).

## Start and verify

```bash
docker compose up -d --build
docker compose ps

curl -s http://localhost:8080/api/v1/settings/public | jq
curl -s http://localhost:9000/readyz | jq
```

Open <http://localhost:8080>. Bootstrap the admin password before logging in:

```bash
RUNCODES_ADMIN_PASSWORD='...' \
  PGHOST=localhost PGPORT=5432 PGUSER=runcodes PGDATABASE=runcodes \
  PGPASSWORD="$RUNCODES_DB_PASSWORD" ./database/bootstrap-admin.sh
```

Then sign up a normal account and open the verification link in smtp4dev at
<http://localhost:8081>.

## TLS and a real hostname

Set a hostname and publish ports 80/443:

```bash
export RUNCODES_DOMAIN=runcodes.example.com
export RUNCODES_PUBLIC_URL=https://runcodes.example.com
export FRONTEND_HTTP_PORT=80
export FRONTEND_HTTPS_PORT=443
docker compose up -d
```

Caddy obtains and renews a certificate into the `caddy_data` volume. DNS must
point at the host and ports 80/443 must be reachable (ACME HTTP-01 uses 80).

For an HTTP-only lab, keep `RUNCODES_DOMAIN=:80` and set `DEBUG_MODE=true` on the
backend so the session cookie is not marked `Secure`. Never do that behind HTTPS.

## Data and volumes

| Volume | Contents | Backup? |
| --- | --- | --- |
| `runcodes-db-vol` | PostgreSQL data directory | **yes** |
| `runcodes-seaweed-vol` | submissions, test cases, output archives | **yes** |
| `runcodes-redis-vol` | cache/session counters | no (disposable) |
| `caddy_data` / `caddy_config` | certificates and Caddy state | regenerable |
| `smtp4dev-data` | caught mail | no |
| `./exec` | judge workspaces | no (disposable) |

PostgreSQL:

```bash
docker compose exec -T database pg_dump -U runcodes -d runcodes -Fc > runcodes.dump

# Restore into an empty database
docker compose exec -T database pg_restore -U runcodes -d runcodes --clean --if-exists < runcodes.dump
```

SeaweedFS (stop writers first):

```bash
docker compose stop backend judge seaweed
docker run --rm \
  -v runcodes-react_runcodes-seaweed-vol:/data:ro \
  -v "$PWD:/backup" \
  alpine tar czf /backup/seaweed-$(date -u +%Y%m%dT%H%M%SZ).tgz -C /data .
docker compose start seaweed backend judge
```

The exact volume name depends on the Compose project name; check
`docker volume ls`.

::: danger `docker compose down -v` destroys data
It removes the database and object-store volumes. A new volume is initialized
from `database/schema/02-schema.sql` and must be stamped in Atlas history before
the next migration; see [Database](../services/database.md#existing-databases).
:::

## Updating

```bash
git pull
docker compose up -d --build
```

Compose recreates changed containers. The judge and backend roll with the new
images; in-flight judge runs are interrupted and reconciled to `server_error` by
the backend's stale sweeper. Schedule updates outside submission windows.

If only one service changed:

```bash
docker compose up -d --build backend
```

If the schema changed, apply migrations before or after the rollout as the
release notes direct. See [Database](../services/database.md#migrations).

## Operating

```bash
docker compose ps
docker compose logs -f --tail=100 judge
docker compose logs -f backend
docker compose restart backend
docker compose exec judge id          # root inside the namespace
```

### Common failures

| Symptom | Check |
| --- | --- |
| judge `/readyz` says `podman unavailable` | socket exists, judge created after it, mount is a socket |
| judge `/readyz` says `permission denied` | SELinux or uid mapping; keep `user: "0:0"` and `label=disable` |
| backend `/readyz` returns `503` | PostgreSQL health and the app-role password |
| submissions return `503` | judge readiness and SeaweedFS credentials |
| frontend loads but API calls fail | Caddy logs, backend logs, `/api` proxy |
| verification mail missing | smtp4dev at `:8081`, `RUNCODES_PUBLIC_URL`, SMTP vars |

See [Getting started § First-run troubleshooting](../guide/getting-started.md#first-run-troubleshooting)
for the detailed Podman diagnosis.

## Adding observability

The overlay starts the application plus the telemetry stack and publishes every
UI on loopback:

```bash
make observability-init
make observability-up
make observability-smoke-test
```

It uses the same Compose project and volumes, so it is not a separate
deployment. See [Local observability stack](../observability/local.md).

## Related pages

- [Getting started](../guide/getting-started.md)
- [Configuration](../reference/configuration.md)
- [Ansible deployment](./ansible-setup.md)
- [Database](../services/database.md)
