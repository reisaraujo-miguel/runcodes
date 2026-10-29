---
title: Image-based Compose deployment
description: Deploy a single host from published GHCR images, without building the application from source.
---

# Image-based Compose deployment

This path runs the single-host stack from **published images**. It never builds
the frontend, backend, judge or database; `deploy/compose/deploy.sh` pulls the
four application images from the configured registry and starts the stack. The
judge pulls the language runner images on its own.

The deployment runs on **Podman by default** and also supports **Docker
Compose v2**. Both engines use the same Compose file; the judge always drives
the host's rootless Podman socket.

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

- Podman with a Compose provider (the default), or Docker Engine with Compose v2.
- Rootless Podman on the host, with its API socket. The judge needs the socket
  even when Compose runs on Docker.
- A public hostname whose DNS resolves to the host, with inbound TCP 80/443
  open, when the frontend is published on those ports. A rootless Podman host
  must also allow the deployment user to bind them; see
  [Public hostname and TLS](#public-hostname-and-tls).
- A host directory for the judge workspaces, shared with the Podman service.
- Network access to the configured registry. `podman login ghcr.io`
  authenticates the application pulls and the judge's runner pulls; with Docker
  Compose, also use `docker login ghcr.io` for the application images.
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
# Fill the required secrets, the image settings and the public hostname.
$EDITOR .env
./deploy.sh up
```

`deploy.sh up`:

1. loads `.env` and resolves the host execution directory to an absolute path;
2. selects the Compose engine: Podman by default, Docker when Podman is absent
   or `CONTAINER_ENGINE=docker`;
3. checks the rootless Podman socket the judge needs;
4. runs `<engine> compose pull` for the application and infrastructure images;
5. starts the stack with `<engine> compose up -d --remove-orphans`;
6. prints the container status.

The template is a public-deployment profile: it publishes the frontend on 80 and
443 for `runcodes.example.com`. Replace that placeholder, point DNS at the host
and allow the port binding before the first `up`; see
[Public hostname and TLS](#public-hostname-and-tls).

The first start initializes PostgreSQL from the database image. The seeded
`admin@admin.com` account has no usable password; see
[First boot](#first-boot).

### Compose engine

`CONTAINER_ENGINE` selects the engine. An environment value overrides the value
in `deploy/compose/.env`.

| Value | Behaviour |
| --- | --- |
| `auto` (default) | uses `podman compose` when available, otherwise `docker compose` |
| `podman` | requires Podman and a Compose provider |
| `docker` | requires Docker Engine and Compose v2 |

With `auto`, Podman wins when both engines are installed and working. Force
Docker for one run:

```bash
CONTAINER_ENGINE=docker ./deploy.sh up
```

The judge still needs the rootless Podman socket with either engine, because it
creates the runner containers through the Podman API.

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

The release workflows derive the GHCR namespace from the repository that runs
them. The upstream repository publishes:

```text
ghcr.io/runcodes-icmc/runcodes-{frontend,backend,judge,database}
ghcr.io/runcodes-icmc/runcodes-runner-<language>
```

A fork publishes the same names under its own namespace, for example
`ghcr.io/reisaraujo-miguel/runcodes-frontend`. Point `RUNCODES_IMAGE_NAMESPACE`
and `RUNCODES_RUNNER_IMAGE_PREFIX` at that namespace; no workflow edit is
needed.

For a different registry or a mirror that is not published by these workflows,
publish or mirror the application images and the `runcodes-runner-<language>`
images there. The deployment does not build them. If a required image is
missing, Compose fails during `pull` with a `not found` or `unauthorized` error.

## The deploy script

| Command | Action |
| --- | --- |
| `./deploy.sh up` | pull, start or update, then show status (default) |
| `./deploy.sh pull` | pull the configured images |
| `./deploy.sh status` | show container status |
| `./deploy.sh logs` | follow the stack logs |
| `./deploy.sh restart` | restart the containers |
| `./deploy.sh down` | stop and remove the containers, including the observability stack when available; volumes are kept |
| `./deploy.sh config` | render the final Compose configuration |
| `./deploy.sh observability` | start the stack plus the telemetry stack |
| `./deploy.sh observability-logs` | follow the telemetry stack logs |

`ENV_FILE=/path/to/env ./deploy.sh up` uses a different environment file. The
script sources that file, so keep it shell-compatible: quote values with spaces
(as `.env.example` does) and avoid a literal `$` in secrets.

The script runs `podman compose` by default and uses `docker compose` when
Podman is unavailable or `CONTAINER_ENGINE=docker`. Set `CONTAINER_ENGINE` in
the environment or in `deploy/compose/.env`; an environment value wins.

The Compose file itself is
[`deploy/compose/docker-compose.yml`](https://github.com/reisaraujo-miguel/runcodes/blob/main/deploy/compose/docker-compose.yml).
It has no `build:` sections and works with either engine.

## Public hostname and TLS

The stack publishes only the frontend, so Caddy is the public entry point and
owns the certificate. A public deployment needs three things: a hostname, host
ports 80/443, and permission for the rootless Podman user to bind those ports.

### Set the origin

```bash
RUNCODES_DOMAIN=runcodes.example.com          # a hostname enables automatic HTTPS
RUNCODES_PUBLIC_URL=https://runcodes.example.com
FRONTEND_HTTP_PORT=80
FRONTEND_HTTPS_PORT=443
```

`RUNCODES_DOMAIN` is the Caddy site address. A hostname makes Caddy obtain and
renew a certificate into the `caddy_data` volume and redirect HTTP to HTTPS.
`RUNCODES_PUBLIC_URL` must be that same origin, because verification and
password-reset links are built from it; leaving it at `localhost` sends users to
an address only the server can reach.

### Allow the ports

Rootless Podman forwards published ports through an unprivileged helper,
`rootlessport`. Where `net.ipv4.ip_unprivileged_port_start` is still 1024, the
frontend fails to start:

```text
rootlessport cannot expose privileged port 80 ... (currently 1024)
```

Lower the limit to **80**, not 0:

```bash
echo 'net.ipv4.ip_unprivileged_port_start=80' \
  | sudo tee /etc/sysctl.d/99-rootless-ports.conf
sudo sysctl --system
sysctl net.ipv4.ip_unprivileged_port_start    # -> 80
```

Accounts on the host can then bind 80–1023; 1–79, including SSH on 22, stay
restricted. The setting gives the deployment user nothing it did not already
have, because that user runs the containers. It is the value the four-machine
deployment writes to every host through `rc_common_sysctl`
(`deploy/ansible/roles/rc-common/defaults/main.yml`). A rootful Docker engine
binds published ports itself, so only rootless Podman needs this.

No reboot is needed. If the stack already ran on other ports, recreate the
frontend so the new mapping applies: Podman Compose does not reconcile a running
container with an edited Compose file the way Docker Compose does.

```bash
podman compose --env-file .env -f docker-compose.yml up -d --force-recreate frontend
```

To keep the 1024 default instead, publish the frontend on loopback and let a
host proxy own 80/443; see
[Fronting the stack with a proxy](#fronting-the-stack-with-a-proxy).

### Point DNS and verify

Caddy validates through port 80 (HTTP-01) or 443 (TLS-ALPN-01), so DNS must
resolve to this host and the provider firewall must allow both before the first
start. If DNS is not ready, issuance retries in the background and the site
stays unreachable over HTTPS.

```bash
getent hosts runcodes.example.com      # -> this host's public address
./deploy.sh up
curl -sI http://runcodes.example.com   # 308 -> https://
curl -sI https://runcodes.example.com  # 200 with a valid certificate
```

Then open the origin and sign in. The session cookie is created with `Secure`,
so it is only sent over HTTPS: a plain-HTTP origin cannot hold a login. Port 80
alone is not enough for a real deployment.

### Plain HTTP without TLS

`RUNCODES_DOMAIN=:80` makes Caddy serve any host on port 80 with no
certificate. It is useful to check that the SPA and the `/api/*` proxy answer
before DNS exists, but not to run a class: this stack does not set `DEBUG_MODE`,
so the session cookie stays `Secure` and sign-in does not survive outside
`localhost`. For a classroom network without public DNS, terminate TLS at a
proxy and keep the stack on loopback.

### Fronting the stack with a proxy

If another service already owns 80/443, or host policy forbids the sysctl,
publish the frontend on loopback and terminate TLS at the proxy:

- set `RUNCODES_DOMAIN=:80` and `FRONTEND_HTTP_PORT=8080` in `.env`;
- replace the two frontend `ports:` entries in `deploy/compose/docker-compose.yml`
  with `127.0.0.1:${FRONTEND_HTTP_PORT:-8080}:80` (`FRONTEND_HTTPS_PORT` is then
  unused);
- point the proxy at `127.0.0.1:8080` and set `RUNCODES_PUBLIC_URL` to the
  proxy's HTTPS origin.

Caddy keeps serving the SPA and proxying `/api/*`, so the proxy only forwards
HTTP and the security headers stay in the container. The `caddy_data` volume no
longer holds certificates; the proxy does.

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

Then open the origin in `RUNCODES_PUBLIC_URL`, sign in as `admin@admin.com`,
change the password in **Meu perfil**, create a second administrator, and set
the contact details in **Configurações**.

The first account verification mail is caught by the bundled smtp4dev at
<http://localhost:8081>. For a real deployment, configure the SMTP relay in
`.env` before inviting users.

### Default URLs

The frontend is public at the origin in `.env`. The remaining services bind to
loopback on the host:

| Service | URL |
| --- | --- |
| Frontend | `https://<RUNCODES_DOMAIN>` (`runcodes.example.com` in the template) |
| smtp4dev | <http://localhost:8081> |
| Backend (loopback) | <http://localhost:8443> |
| Judge (loopback) | <http://localhost:9000> |
| SeaweedFS S3 (loopback) | <http://localhost:8333> |

## Mail

The bundled `smtp4dev` catches all mail on loopback, so a fresh deployment works
without a relay: verification links appear at <http://localhost:8081>. A real
deployment points the backend at a relay instead.

### Relay settings

| Variable | Meaning |
| --- | --- |
| `RUNCODES_SMTP_HOST` | relay hostname, with no scheme and no port |
| `RUNCODES_SMTP_PORT` | relay port; the backend defaults to `587`, the template keeps `25` for `smtp4dev` |
| `RUNCODES_SMTP_USERNAME` / `_PASSWORD` | credentials, only when the relay authenticates |
| `RUNCODES_SMTP_FROM` | `From` header of every message, parsed as an address |

```bash
RUNCODES_SMTP_HOST=mail.example.com
RUNCODES_SMTP_PORT=587
RUNCODES_SMTP_USERNAME=runcodes@example.com
RUNCODES_SMTP_PASSWORD=...
RUNCODES_SMTP_FROM='RunCodes <no-reply@example.com>'
```

The mailbox in `RUNCODES_SMTP_FROM` must be one the relay lets you send as; the
display-name form above is valid.

### How the backend connects

Two constraints decide whether a relay works:

- **The host is a bare hostname.** The value goes straight to the TCP dialer, so
  a scheme breaks it: `RUNCODES_SMTP_HOST=ssl://smtp.gmail.com` fails at send
  time with `mail: connecting to ssl://smtp.gmail.com:465: ... no such host`.
- **STARTTLS only.** The backend connects in plaintext, upgrades when the server
  advertises STARTTLS, and authenticates only after the upgrade. Use port `587`,
  or `25`/`2525` when the relay offers STARTTLS there. Port `465` is implicit
  TLS (SMTPS): the session waits for a greeting the server will not send until
  the client starts TLS, and the send fails on the 10-second mail timeout.

`RUNCODES_PUBLIC_URL` must be the public browser origin: verification and
recovery links are built from it, so a stale value produces links to an address
only the server can reach.

### Google Workspace and Gmail

Gmail's SMTP works over `smtp.gmail.com:587` with STARTTLS. Two account settings
must match:

```bash
RUNCODES_SMTP_HOST=smtp.gmail.com
RUNCODES_SMTP_PORT=587
RUNCODES_SMTP_USERNAME=runcodes@your-domain.example
# App Password from the account, not the account password
RUNCODES_SMTP_PASSWORD=...
RUNCODES_SMTP_FROM='RunCodes <no-reply@your-domain.example>'
```

- **Use an App Password**, not the account password. A normal password is
  rejected with `535 5.7.8 Username and Password not accepted`, and App
  Passwords require 2-Step Verification on the account.
- **Send as an address the account owns.** `RUNCODES_SMTP_FROM` must be the
  authenticated mailbox or a verified alias ("Send mail as") of it, or Gmail
  rejects the message or rewrites the `From` header. Add
  `no-reply@your-domain.example` as an alias in the Workspace admin console, or
  send from the account address.

### When a send fails

Signup still returns `201` and the account is created: the failure is logged as
`error sending account verification email` and the user stays unverified. After
fixing the relay, resend instead of deleting the account:

```bash
curl -sS -X POST https://<RUNCODES_DOMAIN>/api/v1/user/resend-verification \
  -H 'Content-Type: application/json' \
  -d '{"email":"the-user@example.com"}'
```

The endpoint answers `202` whether or not the address exists, so confirm
delivery in the mailbox or the backend log. The `smtp4dev` container can stay in
the Compose file; it is unused once the backend points somewhere else.

Repeat requests for the same address inside five minutes are suppressed by a
per-recipient cooldown shared by password resets and verification resends. A
suppressed request still answers success; the backend logs
`transactional email suppressed` with the `flow` and `reason`, and
`password reset requested for an unknown address` when the address has no
account. A send that fails releases the cooldown, so retrying after a relay
error is not held back.

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

The telemetry stack is opt-in and off by default. Start it on top of the
application:

```bash
./deploy.sh observability
```

This layers the repository-root `docker-compose.observability.yml` onto the
stack: an OpenTelemetry collector, Prometheus, Alertmanager, Loki, Tempo,
Pyroscope, Grafana and the node, Postgres, Redis and blackbox exporters. Every
UI and API binds to `127.0.0.1`, so nothing becomes public:

| Service | URL |
| --- | --- |
| Grafana | <http://127.0.0.1:3000> |
| Prometheus | <http://127.0.0.1:9090> |
| Tempo | <http://127.0.0.1:3200> |
| Loki | <http://127.0.0.1:3100> |
| Alertmanager | <http://127.0.0.1:9093> |
| Collector health | <http://127.0.0.1:13133/health/status> |

The first run generates the Grafana admin password into the repository's
`.observability/grafana_admin_password`; the user is `GRAFANA_ADMIN_USER`
(`admin` by default). Bridge it to a laptop with an SSH tunnel
(`ssh -L 3000:127.0.0.1:3000 <host>`) rather than changing
`OBSERVABILITY_BIND_ADDRESS`.

The backend and the judge start exporting traces, metrics and logs to the
collector as soon as the overlay applies, which recreates both containers. The
frontend image is built with the browser exporter pointing at the same-origin
`/otel` path, and Caddy proxies `/otel/v1/traces` to the collector, so browser
traces join the same traces without exposing the collector. That route is part
of `frontend/Caddyfile`, so it needs an image built from the same release as
this Compose file; an older frontend answers that path with the SPA.

::: warning Needs the repository checkout
The overlay and its configuration live at the repository root, so
`./deploy.sh observability` needs more than `deploy/compose`. A deployment
directory copied out of a checkout can still run the application; point
`RUNCODES_OBSERVABILITY_DIR` and `RUNCODES_OBSERVABILITY_STATE` at a copy of
`observability/` and the overlay to enable telemetry there.
:::

`./deploy.sh observability-logs` follows the telemetry containers, and
`./deploy.sh down` removes them together with the application. The
[local observability stack](../observability/local.md) documents the signals,
the smoke tests and the alert rules.

The application also exposes `/healthz`, `/readyz` and `TraceId` headers without
the overlay. The [platform operations guide](../administration/operations.md)
lists the checks to run after every deployment.

## Files

```text
deploy/compose/
├── docker-compose.yml     image-based stack; no build sections
├── .env.example           public origin, ports, registry, secrets and tuning
├── deploy.sh              pull/start/status/logs/down and observability workflow
└── seaweedfs-s3-init.sh   S3 gateway credentials for the bundled SeaweedFS
```

## Troubleshooting

| Symptom | Cause and fix |
| --- | --- |
| `deploy.sh: .env not found` | copy `.env.example` to `.env` in `deploy/compose` |
| `podman socket not found` | start `podman.socket` or set `PODMAN_SOCKET`; the judge needs it with Docker too |
| `unauthorized` during pull | run `podman login ghcr.io` for the runner images (and the application images with Podman); with Docker Compose, also run `docker login ghcr.io` for the application images |
| `not found` during pull | the namespace/tag does not contain that image; check `RUNCODES_IMAGE_NAMESPACE`, `RUNCODES_IMAGE_TAG` and the runner settings |
| judge says `podman unavailable` | the socket mount is stale; recreate the judge with `podman compose --env-file .env -f docker-compose.yml up -d --force-recreate judge` (use `docker compose` when Compose runs on Docker) |
| judge cannot write the workspace | check `RUNCODES_EXEC_DIR` permissions and the SELinux label; the one-shot makes it writable, but the host must allow it |
| the frontend loads but API calls fail | run `./deploy.sh logs` and check the backend and Caddy |
| frontend fails with `rootlessport cannot expose privileged port 80` | the host still starts unprivileged ports at 1024; set `net.ipv4.ip_unprivileged_port_start=80` and run `./deploy.sh up` again; see [Public hostname and TLS](#public-hostname-and-tls) |
| HTTPS does not load and the frontend log shows ACME errors | DNS does not resolve to this host, or 80/443 are blocked; Caddy retries in the background. Check that the name resolves here and that the provider firewall allows both ports |
| `http://localhost` returns 404 | the site address is a hostname, so Caddy only answers that Host; use `RUNCODES_PUBLIC_URL`, or `RUNCODES_DOMAIN=:80` for a no-TLS check |
| no verification mail | the target is still `smtp4dev`, or the relay settings are wrong; see [Mail](#mail) |
| mail log says `no such host` | `RUNCODES_SMTP_HOST` carries a scheme or a port; use the bare hostname |
| mail log says `535` or `authenticating` | wrong password, or a Gmail account password instead of an App Password |
| mail send stalls for ~10 s, then fails | the port is implicit TLS (`465`); use `587` with STARTTLS |
| admin login fails after first boot | the seeded admin has no password until `bootstrap-admin.sh` runs |
| database is empty after an upgrade | the image only initializes a new volume; apply migrations |

## Related pages

- [Deployment overview](./index.md)
- [Production checklist](./checklist.md)
- [Compose from source](./compose.md)
- [Ansible setup](./ansible-setup.md)
- [Configuration reference](../reference/configuration.md)
- [Platform operations](../administration/operations.md)
- [Security & scaling](./security.md)
