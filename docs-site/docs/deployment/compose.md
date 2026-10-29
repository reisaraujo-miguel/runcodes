---
title: Compose from source
description: Run the source-build RunCodes stack on a single host with Podman Compose (default) or Docker Compose, including TLS, volumes, updates and backups.
---

# Compose from source

`docker-compose.yml` at the repository root builds the application from source
and runs the full stack on one host: Caddy and the SPA, the backend, the judge,
PostgreSQL, Redis, SeaweedFS and smtp4dev. It is the development, demo and
source-inspection path.

Podman Compose is the default engine; Docker Compose v2 works the same way.
Both use the same file, and the judge always needs the host's rootless Podman
socket.

::: tip Podman is the default, Docker is supported
Commands on this page use `podman compose`. With Docker, replace
`podman compose` with `docker compose`, and `podman` with `docker` for
`run`, `exec`, `volume` and `login`. Keep the rootless Podman socket for the
judge in either case.
:::

For a single production host that pulls published images instead of building,
use [Image-based Compose](./image-compose.md) and
`deploy/compose/deploy.sh`.

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
each other and the data services over the internal `runcodes-network`. Every
host port above is overridable from `.env` (`FRONTEND_HTTP_PORT`,
`FRONTEND_HTTPS_PORT`, `BACKEND_PORT`, `JUDGE_PORT`, `DATABASE_PORT`,
`SEAWEED_S3_PORT`, `SMTP4DEV_WEB_PORT`, `SMTP4DEV_SMTP_PORT`), which lets a
second stack run on the same machine.

Caddy handles two jobs: it serves the SPA with a client-side-routing fallback,
and it reverse-proxies `/api/*` to `backend:8443`. That keeps the API same-origin
(no CORS) and the SSE stream on one connection.

## Prerequisites

- Podman with a Compose provider (the default), or Docker Engine with Compose v2.
- Rootless Podman on the host with `podman.socket` started. The judge needs the
  socket with either Compose engine.
- A user that can run Compose: root, a user in the `docker` group, or a
  rootless Podman user.
- Free disk for the images, the database and the object store.

```bash
systemctl --user start podman.socket
ls -l /run/user/$(id -u)/podman/podman.sock
```

If the socket path is not the default, set `PODMAN_SOCKET` before `podman compose
up`. The judge container mounts it at `/run/podman/podman.sock`.

::: warning Socket lifetime
A bind mount pins the socket that existed when the container was created.
Starting `podman.socket` after the judge, or restarting the socket later,
requires recreating the judge:

```bash
podman compose up -d --force-recreate judge
```
:::

## Configuration

Compose reads secrets from the environment or a root `.env` file. The required
variables are:

```bash
RUNCODES_JWT_SECRET=...
RUNCODES_DB_PASSWORD=...
RUNCODES_DB_APP_PASSWORD=...
RUNCODES_DB_JUDGE_PASSWORD=...
RUNCODES_REDIS_PASSWORD=...
RUNCODES_JUDGE_TOKEN=...
RUNCODES_S3_CREDENTIALS_KEY=...
RUNCODES_S3_CREDENTIALS_SECRET=...
```

Redis starts with `--requirepass` and the backend receives the same password.
SeaweedFS only enables S3 authentication when given a config file, so a one-shot
`seaweed-s3-init` container generates `s3.json` from the S3 credentials and the
gateway starts with `-s3.config`; anonymous S3 requests are rejected.

Optional and commonly set:

| Variable | Default | Meaning |
| --- | --- | --- |
| `RUNCODES_PUBLIC_URL` | `http://localhost:8080` | origin used in verification and recovery links |
| `RUNCODES_SMTP_FROM` | `RunCodes <no-reply@runcodes.local>` | sender address |
| `RUNCODES_SMTP_HOST` / `_PORT` | Compose sets `smtp4dev:25` | real relay; bare hostname, STARTTLS on `587` |
| `RUNCODES_SMTP_USERNAME` / `_PASSWORD` | empty | relay credentials |
| `RUNCODES_DOMAIN` | `:80` | Caddy site: hostname enables automatic HTTPS |
| `FRONTEND_HTTP_PORT` / `FRONTEND_HTTPS_PORT` | `8080` / `8444` | frontend host ports |
| `BACKEND_PORT` / `JUDGE_PORT` | `8443` / `9000` | loopback host ports |
| `DATABASE_PORT` / `SEAWEED_S3_PORT` | `5432` / `8333` | loopback host ports |
| `SMTP4DEV_WEB_PORT` / `SMTP4DEV_SMTP_PORT` | `8081` / `1025` | loopback host ports |
| `PODMAN_SOCKET` | `/run/user/1000/podman/podman.sock` | rootless socket path |
| `RUNCODES_IMAGE_REGISTRY` | `ghcr.io` | runner image registry |
| `RUNCODES_RUNNER_IMAGE_PREFIX` | `runcodes-icmc/runcodes-runner` | runner image path; `-<language>` is appended |
| `RUNCODES_RUNNER_IMAGE_TAG` | `latest` | runner image tag |
| `JUDGE_IMAGE_FORMAT` | derived | advanced full template; wins over the registry, prefix and tag |
| `JUDGE_UID` / `JUDGE_GID` | `0` / `0` | container identity for the judge |
| `RUNCODES_JUDGE_STALE_TIMEOUT` | `35m` | backend age before a claimed run is marked stale |
| `JUDGE_MAX_RUN_DURATION` | `30m` | judge end-to-end budget per claim |
| `JUDGE_MAX_WORKSPACE_BYTES` | `1 GiB` | sampled aggregate workspace size per run |
| `JUDGE_MAX_WORKSPACE_ENTRIES` | `50000` | sampled aggregate workspace entries per run |
| `CONTACT_EMAIL` / `CONTACT_DISCLAIMER` | example values | build-time fallbacks only |

The full list is in [Configuration](../reference/configuration.md).

## Start and verify

```bash
podman compose up -d --build
podman compose ps

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
podman compose up -d
```

On a rootless Podman host, publishing below 1024 also needs
`net.ipv4.ip_unprivileged_port_start=80`; without it the frontend fails with
`rootlessport cannot expose privileged port 80`. The
[image-based deployment](./image-compose.md#public-hostname-and-tls) has the
sysctl, its trade-offs and the proxy alternative.

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
podman compose exec -T database pg_dump -U runcodes -d runcodes -Fc > runcodes.dump

# Restore into an empty database
podman compose exec -T database pg_restore -U runcodes -d runcodes --clean --if-exists < runcodes.dump
```

SeaweedFS (stop writers first):

```bash
podman compose stop backend judge seaweed
podman run --rm \
  -v runcodes-react_runcodes-seaweed-vol:/data:ro \
  -v "$PWD:/backup" \
  alpine tar czf /backup/seaweed-$(date -u +%Y%m%dT%H%M%SZ).tgz -C /data .
podman compose start seaweed backend judge
```

The exact volume name depends on the Compose project name; check
`podman volume ls`.

::: danger `podman compose down -v` destroys data
It removes the database and object-store volumes. A new volume is initialized
from `database/schema/02-schema.sql` and must be stamped in Atlas history before
the next migration; see [Database](../services/database.md#existing-databases).
:::

## Updating

```bash
git pull
podman compose up -d --build
```

Compose recreates changed containers. The judge and backend roll with the new
images; in-flight judge runs are interrupted and reconciled to `server_error` by
the backend's stale sweeper. Schedule updates outside submission windows.

If only one service changed:

```bash
podman compose up -d --build backend
```

If the schema changed, apply migrations before or after the rollout as the
release notes direct. See [Database](../services/database.md#migrations).

## Operating

```bash
podman compose ps
podman compose logs -f --tail=100 judge
podman compose logs -f backend
podman compose restart backend
podman compose exec judge id          # root inside the namespace
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
for the detailed Podman socket diagnosis.

## Adding observability

The overlay starts the application plus the telemetry stack and publishes every
UI on loopback:

```bash
make observability-init
make observability-up
make observability-smoke-test
```

It uses the same Compose project and volumes, so it is not a separate
deployment. The Make targets use `podman compose` by default; set
`CONTAINER_ENGINE=docker` to use Docker. See
[Local observability stack](../observability/local.md).

## Related pages

- [Getting started](../guide/getting-started.md)
- [Configuration](../reference/configuration.md)
- [Ansible deployment](./ansible-setup.md)
- [Database](../services/database.md)
