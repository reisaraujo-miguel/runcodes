---
title: Image-based Compose deployment
description: Deploy a single host from published GHCR images, without building the application from source.
---

# Image-based Compose deployment

This path runs the single-host stack from **published images**. It never builds
the frontend, backend, judge or database; `deploy/compose/deploy.sh` pulls the
four application images from the configured registry and starts the stack. The
judge pulls the language runner images on its own.

Use it when you want a production-like single host without a Go, Bun or
container build toolchain. For local development or a demo from source, use
[Getting started](../guide/getting-started.md). For four hosts and the full
telemetry stack, use [Ansible](./ansible-setup.md).

## What runs

| Service | Image |
| --- | --- |
| Frontend + Caddy | `<registry>/<namespace>/runcodes-frontend:<tag>` |
| Backend API | `<registry>/<namespace>/runcodes-backend:<tag>` |
| Judge | `<registry>/<namespace>/runcodes-judge:<tag>` |
| Database | `<registry>/<namespace>/runcodes-database:<tag>` |
| Runner images | `<registry>/<runner prefix>-<language>:<runner tag>` |
| SeaweedFS, Redis, smtp4dev | public upstream images |

Only the frontend is published on a network interface. The backend, judge,
database, SeaweedFS and smtp4dev bind to loopback. Caddy serves the SPA and
reverse-proxies `/api/*` to the backend, so the API is same-origin.

## Requirements

- Docker Engine with Compose v2.
- Rootless Podman on the host, with its API socket.
- A host directory for the judge workspaces, shared with the Podman service.
- Network access to the configured registry, and `docker login` when private.
- About 15 GB of disk for the runner images, plus space for PostgreSQL,
  SeaweedFS and the workspaces.

```bash
systemctl --user start podman.socket
ls -l /run/user/$(id -u)/podman/podman.sock
```

## Quick start

From a repository checkout:

```bash
cd deploy/compose
cp .env.example .env
# Fill the required secrets and the image settings.
$EDITOR .env
./deploy.sh up
```

`deploy.sh up`:

1. loads `.env` and resolves the host execution directory to an absolute path;
2. checks the rootless Podman socket;
3. runs `docker compose pull` for the application and infrastructure images;
4. starts the stack with `docker compose up -d --remove-orphans`;
5. prints the container status.

The first start initializes PostgreSQL from the database image. The seeded
`admin@admin.com` account has no usable password; see
[First boot](#first-boot).

### Required secrets

Set these in `deploy/compose/.env`:

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

Generate passwords and secrets with `openssl rand -hex 24` or `-hex 32`. Do not
reuse the example values.

## Configure the images

| Variable | Default | Meaning |
| --- | --- | --- |
| `RUNCODES_IMAGE_REGISTRY` | `ghcr.io` | registry for application and runner images |
| `RUNCODES_IMAGE_NAMESPACE` | `runcodes-icmc` | namespace that holds the application images |
| `RUNCODES_IMAGE_TAG` | `latest` | application image tag |
| `RUNCODES_RUNNER_IMAGE_PREFIX` | `runcodes-icmc/runcodes-runner` | runner image path inside the registry |
| `RUNCODES_RUNNER_IMAGE_TAG` | `latest` | runner image tag |
| `JUDGE_IMAGE_FORMAT` | derived | advanced full printf template; wins over the registry/prefix/tag settings |

Application images combine the first three:

```text
<registry>/<namespace>/runcodes-frontend:<tag>
<registry>/<namespace>/runcodes-backend:<tag>
<registry>/<namespace>/runcodes-judge:<tag>
<registry>/<namespace>/runcodes-database:<tag>
```

Runner images combine the registry and runner prefix on the judge:

```text
<registry>/<runner prefix>-<language>:<runner tag>
```

For a fork under `ghcr.io/reisaraujo-miguel`, with all images published there:

```bash
RUNCODES_IMAGE_REGISTRY=ghcr.io
RUNCODES_IMAGE_NAMESPACE=reisaraujo-miguel
RUNCODES_IMAGE_TAG=v1.2.3
RUNCODES_RUNNER_IMAGE_PREFIX=reisaraujo-miguel/runcodes-runner
RUNCODES_RUNNER_IMAGE_TAG=v1.2.3
```

This produces `ghcr.io/reisaraujo-miguel/runcodes-frontend:v1.2.3` and
`ghcr.io/reisaraujo-miguel/runcodes-runner-c:v1.2.3`. The application namespace
and the runner prefix are separate so an installation can use upstream
application images with runner images from a fork, or the reverse.

A trailing slash on the registry and leading or trailing slashes or a trailing
hyphen on the runner prefix are ignored.

`JUDGE_IMAGE_FORMAT` is the escape hatch for a registry layout that does not
follow `<registry>/<prefix>-<language>:<tag>`:

```bash
JUDGE_IMAGE_FORMAT=registry.example.com/team/runners/%s:edge
```

### Publishing the images

The release workflows in this repository publish the upstream application and
runner images to `ghcr.io/runcodes-icmc`. To deploy from another namespace,
publish or mirror all of these names there:

```text
runcodes-frontend  runcodes-backend  runcodes-judge  runcodes-database
runcodes-runner-<language>
```

The deployment does not build them. If a required image is missing, Compose
fails during `pull` with a `not found` or `unauthorized` error.

## The deploy script

| Command | Action |
| --- | --- |
| `./deploy.sh up` | pull, start or update, then show status (default) |
| `./deploy.sh pull` | pull the configured images |
| `./deploy.sh status` | show container status |
| `./deploy.sh logs` | follow the stack logs |
| `./deploy.sh restart` | restart the containers |
| `./deploy.sh down` | stop and remove the containers; volumes are kept |
| `./deploy.sh config` | render the final Compose configuration |

`ENV_FILE=/path/to/env ./deploy.sh up` uses a different environment file. The
script sources that file, so keep it shell-compatible: quote values with spaces
(as `.env.example` does) and avoid a literal `$` in secrets.

The Compose file itself is
[`deploy/compose/docker-compose.yml`](https://github.com/reisaraujo-miguel/runcodes/blob/main/deploy/compose/docker-compose.yml).
It has no `build:` sections.

## First boot

After `./deploy.sh up`, bootstrap the seeded administrator from a checkout of the
repository with `psql` installed. The database port is published on loopback:

```bash
# Owner password from deploy/compose/.env
export RUNCODES_DB_PASSWORD='...'
RUNCODES_ADMIN_PASSWORD='choose-a-strong-password' \
  PGHOST=127.0.0.1 PGPORT=5432 PGUSER=runcodes PGDATABASE=runcodes \
  PGPASSWORD="$RUNCODES_DB_PASSWORD" ./database/bootstrap-admin.sh
```

Then open the frontend, sign in as `admin@admin.com`, change the password in
**Meu perfil**, create a second administrator, and set the contact details in
**Configurações**.

The first account verification mail is caught by the bundled smtp4dev at
<http://localhost:8081>. For a real deployment, configure the SMTP relay in
`.env` before inviting users.

### Default URLs

| Service | URL |
| --- | --- |
| Frontend | <http://localhost:8080> |
| smtp4dev | <http://localhost:8081> |
| Backend (loopback) | <http://localhost:8443> |
| Judge (loopback) | <http://localhost:9000> |
| SeaweedFS S3 (loopback) | <http://localhost:8333> |

## Mail

The bundled `smtp4dev` catches all mail on loopback. For production, set:

```bash
RUNCODES_SMTP_HOST=mail.example.com
RUNCODES_SMTP_PORT=587
RUNCODES_SMTP_USERNAME=...
RUNCODES_SMTP_PASSWORD=...
RUNCODES_SMTP_FROM='RunCodes <no-reply@example.com>'
```

The backend uses STARTTLS when the relay advertises it. The smtp4dev container
can stay in the Compose file; it is unused when the backend points elsewhere.
`RUNCODES_PUBLIC_URL` must be the public browser origin, because verification and
recovery links are built from it.

## Data, backups and upgrades

The named volumes are:

- `runcodes-db-vol` (PostgreSQL);
- `runcodes-seaweed-vol` (source, case and material objects);
- `runcodes-redis-vol` (cache, disposable);
- `smtp4dev-data`, `caddy_data`, `caddy_config`.

Back up PostgreSQL and SeaweedFS together. See
[Platform operations § Backups](../administration/operations.md#backups) and
[Ansible operations § Backups](../deployment/ansible-operations.md#backups-and-restore)
for the commands and the restore order.

### Upgrade

1. Take a database and object-store backup.
2. Read the release and migration notes.
3. Update `RUNCODES_IMAGE_TAG` and `RUNCODES_RUNNER_IMAGE_TAG` in
   `deploy/compose/.env`.
4. Pull the new images:

   ```bash
   ./deploy.sh pull
   ```

5. If the database schema changed, apply migrations as the owner from a
   checkout **before** starting the new application images. The database image
   only initializes an **empty** volume; it does not migrate an existing one:

   ```bash
   export DATABASE_URL="postgres://runcodes:${RUNCODES_DB_PASSWORD}@127.0.0.1:5432/runcodes?sslmode=disable"
   ./database/atlas.sh migrate apply --url "$DATABASE_URL"
   ```

6. Start the stack. `up` pulls again to confirm the tags, then recreates the
   containers:

   ```bash
   ./deploy.sh up
   ```

7. Run a real submission and check `/readyz` on the backend and judge.

Migrations are forward-only. A code rollback that is incompatible with a
migrated schema needs the database backup restored.

## Observability

This Compose file is the application stack. It does not include the
observability overlay; the overlay at the repository root is written for the
source-build stack. Use the [four-machine deployment](./ansible-setup.md) when
you need the full telemetry stack, or run the
[local observability stack](../observability/local.md) separately against a
development checkout.

The application still exposes `/healthz`, `/readyz` and `TraceId` headers. The
[platform operations guide](../administration/operations.md) lists the checks to
run after every deployment.

## Files

```text
deploy/compose/
├── docker-compose.yml     image-based stack; no build sections
├── .env.example           registry, paths, secrets and optional tuning
├── deploy.sh              pull/start/status/logs/down workflow
└── seaweedfs-s3-init.sh   S3 gateway credentials for the bundled SeaweedFS
```

## Troubleshooting

| Symptom | Cause and fix |
| --- | --- |
| `deploy.sh: .env not found` | copy `.env.example` to `.env` in `deploy/compose` |
| `podman socket not found` | start `podman.socket` or set `PODMAN_SOCKET` |
| `unauthorized` during pull | run `docker login ghcr.io` for the registry that holds the images |
| `not found` during pull | the namespace/tag does not contain that image; check `RUNCODES_IMAGE_NAMESPACE`, `RUNCODES_IMAGE_TAG` and the runner settings |
| judge says `podman unavailable` | the socket mount is stale; recreate the judge with `docker compose --env-file .env -f docker-compose.yml up -d --force-recreate judge` |
| judge cannot write the workspace | check `RUNCODES_EXEC_DIR` permissions and the SELinux label; the one-shot makes it writable, but the host must allow it |
| the frontend loads but API calls fail | run `./deploy.sh logs` and check the backend and Caddy |
| no verification mail | smtp4dev is the target, or the real relay settings are wrong; see [Mail](#mail) |
| admin login fails after first boot | the seeded admin has no password until `bootstrap-admin.sh` runs |
| database is empty after an upgrade | the image only initializes a new volume; apply migrations |

## Related pages

- [Deployment overview](./index.md)
- [Production checklist](./checklist.md)
- [Docker Compose from source](./compose.md)
- [Ansible setup](./ansible-setup.md)
- [Configuration reference](../reference/configuration.md)
- [Platform operations](../administration/operations.md)
- [Security & scaling](./security.md)
