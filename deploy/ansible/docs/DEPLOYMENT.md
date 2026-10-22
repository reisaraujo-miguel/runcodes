# Deployment and operations guide

This is the operator guide for
[`deploy/ansible`](../README.md). It covers preparation, first deployment,
upgrades, backups, database migrations, security hardening and the failure modes
seen most often.

The deployment uses four example node addresses and **rootless Podman**, and
pulls application images published by GitHub Actions instead of building them
on the hosts.

- [Prerequisites](#prerequisites)
- [Inventory](#inventory)
- [Configuration](#configuration)
- [Secrets](#secrets)
- [Application images](#application-images)
- [First deployment](#first-deployment)
- [Runtime model and boot](#runtime-model-and-boot)
- [DNS and TLS](#dns-and-tls)
- [Mail and account verification](#mail-and-account-verification)
- [Admin bootstrap](#admin-bootstrap)
- [Migrating existing data](#migrating-existing-data)
- [Database lifecycle and migrations](#database-lifecycle-and-migrations)
- [Upgrades and rollback](#upgrades-and-rollback)
- [Day-2 operations](#day-2-operations)
- [Backups and restore](#backups-and-restore)
- [Troubleshooting](#troubleshooting)
- [Security hardening](#security-hardening)
- [Known limits](#known-limits)

## Prerequisites

### Hosts

Four hosts, one per inventory group:

- Debian 12/13 or Ubuntu 22.04/24.04, systemd, x86_64.
- A sudo-capable login user (`runcodes` by default). The deployment creates
  `/opt/runcodes`; all stacks and the graded runner containers run as this user
  under rootless Podman.
- The four node addresses assigned to the right hosts (see
  [Inventory](#inventory)).
- Outbound HTTPS for apt and GHCR.
- At least the resources in
  [`ARCHITECTURE.md`](ARCHITECTURE.md#capacity-and-scheduling).

If the hosts already run services, stop them (or reimage the hosts) before
deploying: the stack binds 80/443, 5432, 6379 and 8333 and uses fixed container
names (`runcodes-*-*`). A conflicting service will make the new stack fail to
start.

The example uses RFC 5737 documentation addresses. The deployment treats the
four node addresses as the private/management network, so the provider firewall
must restrict every non-public port to the four peers.

### Controller

- Python 3.11+ and `ansible-core` 2.17–2.19.
- The collection in [`requirements.yml`](../requirements.yml): only
  `community.general` (no Docker or POSIX collection).
- SSH access to all four hosts.
- `ansible-vault` for the secrets.

The local `ansible.cfg` puts vendored collections/roles under
`./.ansible-home/`; installing them does not modify the system Python.

### Provider firewall

Set these rules in the cloud/provider firewall (not only in UFW):

| Source | Destination | Ports |
| --- | --- | --- |
| Operator/VPN ranges | all four hosts | 22/tcp |
| Anywhere | edge (`203.0.113.10`) | 80/tcp, 443/tcp |
| The four node addresses | all four hosts | all traffic among the four hosts |

Do not expose PostgreSQL, Redis, SeaweedFS, the judge, Prometheus, Grafana,
Loki, Tempo, Pyroscope, Alertmanager or the OTLP collector to the internet. They
are bound to the node address and are meant to be reachable only by the peers.

## Inventory

```bash
cp inventories/hosts.yml.example inventories/hosts.yml
$EDITOR inventories/hosts.yml
```

The example is pre-filled with the four-host plan:

| Group | Address | Role |
| --- | --- | --- |
| edge | `203.0.113.10` | public SPA/API edge |
| data | `203.0.113.11` | PostgreSQL, Redis, SeaweedFS |
| compute | `203.0.113.12` | judge and runner host |
| observability | `203.0.113.13` | telemetry storage and query |

Each host needs:

| Variable | Meaning |
| --- | --- |
| `ansible_host` | Address Ansible connects to. |
| `private_ip` | Address the peers use; every private service binds here. |
| `public_ip` | Edge only: the address the DNS name resolves to. |
| `ansible_user` | SSH user (default `runcodes`). |
| `ansible_become` | `true`; the user needs sudo. |

The preflight enforces exactly one host in each of `edge`, `data`, `compute` and
`observability`, four hosts total, and four distinct non-empty `private_ip`
values. If your SSH user is not `runcodes`, set both `ansible_user` in the
inventory and `config_runcodes_deploy_user` in `config.yml`.

Prefer SSH keys and NOPASSWD sudo. If the hosts still require a password, add
`ansible_ssh_pass` and `ansible_become_password` as vaulted inventory vars (or
an inventory group_vars file that is gitignored); never put them in the tracked
inventory example.

## Configuration

Every deployment value lives in an untracked file:

```bash
cp inventories/group_vars/all/config.yml.example \
   inventories/group_vars/all/config.yml
$EDITOR inventories/group_vars/all/config.yml
```

`config.yml` is gitignored. The tracked
[`main.yml`](../inventories/group_vars/all/main.yml) only maps `config_*` names
to the `runcodes_*` names used by the roles, and maps vault secrets. Do not put
values in `main.yml`.

The most important settings:

| Setting | Default / notes |
| --- | --- |
| `config_runcodes_cluster_name`, `config_runcodes_environment` | Labels used in Prometheus, Tempo, Alloy and OTel. |
| `config_runcodes_version` | `service.version` for backend/judge; use the release you deploy. |
| `config_runcodes_image_registry` / `_namespace` / `_tag` | Registry, namespace and tag for the four application images. The example uses `ghcr.io` / `runcodes-example` / `latest`; set the namespace to where your workflow publishes. |
| `config_runcodes_pull_images` | Pull app/third-party images before restarting a stack. Keep `true`. |
| `config_runcodes_runner_tag` | Runner image tag; `latest` is supported. |
| `config_runcodes_runner_image_prefix` | Full GHCR prefix for runner images. |
| `config_runcodes_prepull_runner_images` | Pre-pull every language before deploying the judge. |
| `config_runcodes_runner_languages` | Languages to pre-pull; matches `runners/`. |
| `config_runcodes_domain` / `_public_url` | `runcodes.example.com` and its HTTPS origin. |
| `config_runcodes_backend_debug_mode` | Only for HTTP-only labs; makes the session cookie non-Secure. |
| `config_runcodes_judge_*` | Concurrency, execution directory, timeouts, container limits. |
| `config_runcodes_otel_*` | Sampler and metric export interval. |
| `config_runcodes_prometheus_retention_*`, `_tempo_retention_hours` | Telemetry retention. |
| `config_runcodes_firewall_enabled` | Optional UFW; review SSH sources first. |
| `config_runcodes_firewall_private_sources` | Exact peer addresses allowed to reach private ports. |
| `config_runcodes_run_smoke_test` / `_run_submission_smoke_test` | End-to-end verification toggles. |

## Secrets

```bash
cp inventories/group_vars/all/vault.yml.example \
   inventories/group_vars/all/vault.yml
$EDITOR inventories/group_vars/all/vault.yml
ansible-vault encrypt inventories/group_vars/all/vault.yml
```

| Vault variable | Used by | Generate with |
| --- | --- | --- |
| `vault_runcodes_jwt_secret` | backend session signing | `openssl rand -hex 32` |
| `vault_runcodes_judge_token` | backend ↔ judge bearer token | `openssl rand -hex 32` |
| `vault_runcodes_db_owner_password` | PostgreSQL owner / migrations / bootstrap | `openssl rand -hex 24` |
| `vault_runcodes_db_app_password` | `runcodes_app` role used by backend and judge | `openssl rand -hex 24` |
| `vault_runcodes_s3_access_key` | SeaweedFS S3 identity | `openssl rand -hex 16` |
| `vault_runcodes_s3_secret_key` | SeaweedFS S3 identity | `openssl rand -hex 24` |
| `vault_runcodes_redis_password` | Redis `requirepass` | `openssl rand -hex 24` |
| `vault_runcodes_admin_password` | seeded `admin@admin.com` | strong password, no `$` |
| `vault_grafana_admin_password` | Grafana admin | strong password, no `$` |
| `vault_runcodes_legacy_password_salt` | optional legacy user login | old platform salt |
| `vault_runcodes_smtp_username/password` | optional SMTP relay | relay credentials |
| `vault_alertmanager_webhook_url` | optional real alert receiver | incident tool URL |
| `vault_registry_username/password` | optional GHCR token | `read:packages` if private |

**Do not use a literal `$` in a secret.** `podman-compose` interpolates `.env`
values, so a `$` can silently corrupt a password. Hex/base64url output is safe;
the preflight rejects `$` and the `REPLACE_WITH` placeholders.

Secrets are rendered into root-only `.env` files and secret files on the hosts.
They are not committed and are not baked into the published images.

## Application images

The four application images are built and pushed by
[`.github/workflows/app-images.yml`](../../../.github/workflows/app-images.yml):

| Image | Source |
| --- | --- |
| `ghcr.io/runcodes-icmc/runcodes-frontend` | `frontend/` |
| `ghcr.io/runcodes-icmc/runcodes-backend` | `backend/` |
| `ghcr.io/runcodes-icmc/runcodes-database` | `database/` |
| `ghcr.io/runcodes-icmc/runcodes-judge` | `judge/` |

The workflow runs on `v*.*.*` tags and on manual dispatch. It publishes
`latest`, `sha-<short>` and semver tags. The deployment's default is `latest`;
pin `config_runcodes_image_tag` to a release tag for reproducible deploys.

The frontend image is built with `VITE_OTEL_EXPORTER_OTLP_ENDPOINT=/otel`, so the
SPA emits browser traces to the same-origin edge route. No environment-specific
hostname is baked into the image.

The runner and monitor images are published by `runners.yml` and `monitor.yml`
and are pulled by the judge. The compute stage pre-pulls
`runcodes_runner_image_prefix + <language> + ":" + runcodes_runner_tag` into
rootless Podman storage.

Set `config_runcodes_image_namespace` and `config_runcodes_runner_image_prefix`
to the registry/namespace that publishes your images. GHCR packages are
**private by default**. Either make the application, runner and monitor
packages public, or create a token with `read:packages`, put it in
`vault_registry_username`/`vault_registry_password`, and make sure it is valid
for the deployment user's rootless login. With public packages, leave the
credentials empty.

## First deployment

```bash
cd deploy/ansible

python3 -m venv .ansible-venv
. .ansible-venv/bin/activate
pip install 'ansible-core>=2.17,<2.20'
ansible-galaxy collection install -r requirements.yml -p .ansible-home/collections

# inventory, config.yml and vault.yml as described above
ansible-playbook playbooks/site.yml --ask-vault-pass
```

The first run is the slowest: it installs Podman and `podman-compose`, copies
the deploy assets, pulls the published application and third-party images, pre-pulls
the runner images, starts the four stacks, and runs verification.

What the stages do:

| Stage | Effect |
| --- | --- |
| `preflight.yml` | Validates inventory, config.yml, controller assets, secret presence/characters and firewall inputs. No host is touched. |
| `setup.yml` | Installs base packages, Podman/podman-compose, deploy assets, optional UFW; on every host: subordinate IDs, linger, the user manager, `podman.socket`, and the rootless registry login. |
| `deploy-data.yml` | Pulls/starts PostgreSQL, Redis, SeaweedFS and exporters; waits for health; rotates the `runcodes_app` password; bootstraps the admin if still unset. |
| `deploy-observability.yml` | Writes configs, starts Collector/Prometheus/Alertmanager/Loki/Tempo/Pyroscope/Grafana/exporters, waits for readiness. |
| `deploy-compute.yml` | Pre-pulls every runner image, pulls/starts the judge stack with the rootless socket directory mounted, waits for `/readyz`. |
| `deploy-edge.yml` | Pulls/starts Caddy/API/mail catcher/agents, waits for `/healthz` and `/readyz`. |
| `verify.yml` | Waits for every endpoint, asserts every Prometheus target and blackbox probe, checks application/judge-queue metrics, then runs the observability smoke test. |

If a stage fails, fix the cause and rerun `site.yml`; it is idempotent. The data
stage will not re-bootstrap the admin after the first successful run.

## Runtime model and boot

- **Rootless Podman** runs all four stacks as `runcodes`, via `podman-compose`
  and **user** systemd units: `runcodes-edge`, `runcodes-data`,
  `runcodes-compute`, `runcodes-observability`.
- Each user unit runs `podman-compose up -d --force-recreate` and stops with
  `podman-compose stop`. It is enabled under `default.target`; linger starts the
  user manager at boot.
- Rootless Podman publishes Caddy's 80/443 through `rootlessport`. The hosts set
  `net.ipv4.ip_unprivileged_port_start=80`, so the deployment user can bind
  those ports without root privileges.
- `--force-recreate` is deliberate: Podman Compose does not diff container
  configuration the way Docker Compose does, so the unit recreates containers to
  apply image, `.env` and mounted-config changes. A deployment is therefore a
  maintenance-window operation: the containers in each stack are restarted.
- The judge and the graded runner containers share the same rootless
  `podman.socket`. The judge uses `userns_mode: keep-id` and mounts the whole
  `/run/user/<uid>/podman` directory, so a socket restart is visible without
  recreating the judge.
- The compute user unit requires `podman.socket`, waits for the socket, and
  starts after it.
- Application images are pulled by Ansible as `runcodes` before the user unit
  restarts. The unit itself uses local images at boot.

The judge's `/readyz` is the authoritative check for PostgreSQL and Podman.
The deployment does not declare success until it returns 200.

## DNS and TLS

### Normal deployment

1. Create an `A`/`AAAA` record for `config_runcodes_domain` pointing at the edge
   `public_ip` (`203.0.113.10`).
2. Set `config_runcodes_public_url` to `https://runcodes.example.com`.
3. Ensure the provider firewall allows 80/443 to the edge (ACME HTTP-01 needs 80).
4. Run the edge stage. Caddy obtains/renews the certificate into the
   `runcodes-edge_caddy_data` Podman volume.

### HTTP-only / no DNS

Set:

```yaml
config_runcodes_domain: ":80"
config_runcodes_public_url: "http://203.0.113.10"
config_runcodes_backend_debug_mode: true
```

Caddy serves plain HTTP. Browser OTLP still works through `/otel/v1/traces`.
Debug mode is required because the backend otherwise marks the session cookie
`Secure` and the browser will not send it over plain HTTP. Never enable it on
an HTTPS production deployment.

### TLS behind another proxy

Keep `config_runcodes_public_url` as the external HTTPS origin, set
`config_runcodes_domain: ":80"`, and forward the original `Host`/
`X-Forwarded-*` headers to Caddy. The collector CORS list uses the public URL,
so it matches the browser origin.

## Mail and account verification

The default runs `smtp4dev` on the edge and points the backend at
`smtp4dev:25`. It catches every message; the UI is on edge loopback
`127.0.0.1:8081`. This is for bootstrap and testing only.

For real mail, set in `config.yml`:

```yaml
config_runcodes_smtp_host: "smtp.example.org"
config_runcodes_smtp_port: 587
config_runcodes_smtp_from: "RunCodes <no-reply@runcodes.example.com>"
```

and put the credentials in `vault.yml`:

```yaml
vault_runcodes_smtp_username: "..."
vault_runcodes_smtp_password: "..."
```

The backend uses STARTTLS when the relay advertises it. The `smtp4dev` container
stays in the compose file but is unused. To remove it entirely, delete the
service and the backend `depends_on` entry from
[`roles/rc-edge/files/compose.yml`](../roles/rc-edge/files/compose.yml) and
rerun the edge stage.

`config_runcodes_public_url` must be the browser-facing origin because it builds
the verification link.

## Admin bootstrap

The database image seed creates `admin@admin.com` with the deliberately unusable
hash `!`. The data stage runs `bootstrap-admin.sh` only while the stored hash is
still `!`; later UI changes are preserved.

Manual reset:

```bash
ssh runcodes@203.0.113.11
podman exec -it -e PGPASSWORD='<db-owner-password>' \
  -e RUNCODES_ADMIN_PASSWORD='<new-admin-password>' \
  -e RUNCODES_ADMIN_EMAIL='admin@admin.com' \
  runcodes-data-database /bootstrap/bootstrap-admin.sh
```

The script is mounted read-only from
`/opt/runcodes/bootstrap/bootstrap-admin.sh` and hashes the password with
bcrypt cost 12 on the server.

## Migrating existing data

The new stack starts **fresh** database and object-store volumes. It does not
read existing host directories under `/opt/runcodes/`, so stopping the previous
containers and running `site.yml` does not migrate production data.

For the database, use the one-shot importer in
[`database/legacy-migration/README.md`](../../../database/legacy-migration/README.md):
dump the existing database data-only, start the new data stack on an empty
volume, and run `database/legacy-migration/migrate.sh` against the new database
as the owner. The setup stage places the importer, its SQL and the legacy schema
under `/opt/runcodes/database/` on the data host. Set
`vault_runcodes_legacy_password_salt` so imported users can upgrade their old
hashes on first login. Review and reconcile the existing and new SeaweedFS
object keys separately; the importer covers the relational database, not the
object store.

Plan a cutover window: stop the previous containers, take the dumps, deploy the
new stack, import, run `verify.yml`, and only then point DNS/ports at the new
edge.

## Database lifecycle and migrations

### Fresh volume

The published database image runs `database/schema/01..05` on an empty volume:
schema, seed data, owner password and the row-only `runcodes_app` role. The data
stage then re-runs `05-app-role.sh` (idempotent) and bootstraps the admin. The
fresh volume is not stamped in Atlas history; see
[`database/README.md`](../../../database/README.md#existing-databases) before
applying migrations later.

### Existing volume (upgrades)

The deployment never applies migrations automatically. Back up first, then:

```bash
ssh runcodes@203.0.113.11
cd /opt/runcodes/database

export DATABASE_URL="postgres://runcodes:<owner-password>@203.0.113.11:5432/runcodes?sslmode=disable"
./atlas.sh migrate status --url "$DATABASE_URL"
./atlas.sh migrate apply --url "$DATABASE_URL"
```

Use the node address, not `localhost`: PostgreSQL is published on the node
address, not loopback. The first `atlas.sh` run downloads the pinned binary into
`/opt/runcodes/database/.tools/`. If the history is not established, follow
the baseline/stamp instructions in the database README.

Migrations run as the owner (`runcodes`), never as `runcodes_app`.

### Changing the app password

Change `vault_runcodes_db_app_password`, then run data, compute and edge. The
data stage rotates the role; existing DB sessions remain valid, new ones use the
new secret. Run compute/edge promptly so their pools reconnect.

### Changing the owner password

`POSTGRES_PASSWORD` only initializes a new volume. Changing the vault value does
not change an existing cluster's `runcodes` role. Rotate it deliberately with
`ALTER ROLE` and update the vault in the same window, or restore from backup.

## Upgrades and rollback

### Application upgrade

1. Merge/tag the release; the app-images workflow publishes the new images.
2. Set `config_runcodes_image_tag` to the release tag (or keep `latest` if you
   accept a moving target).
3. If the schema changed, read the migration notes and back up.
4. Run:

   ```bash
   ansible-playbook playbooks/site.yml --ask-vault-pass
   ```

   The user stack units pull the configured images and force-recreate their
   containers. The judge is restarted with the rootless socket directory
   mounted.

This is not zero-downtime: each stack's containers are recreated, and in-flight
judge runs are abandoned (the backend's stale sweeper marks them
`server_error`). Schedule upgrades outside exams/submission windows.

### Manual image update / restart

```bash
ssh runcodes@<host>
cd /opt/runcodes/compose/<stack>
podman-compose -p runcodes-<stack> -f compose.yml pull
systemctl --user restart runcodes-<stack>
```

The restart runs the unit's `podman-compose up --force-recreate` and applies
mounted config changes. If only one service needs a restart, use
`podman-compose ... restart <service>`; the unit remains active.

### Rollback

- Pin `config_runcodes_image_tag` to a known release tag and rerun the stages.
  Keep previous tags available in GHCR.
- Revert the controller checkout and rerun for config/asset changes.
- Database migrations are forward-only. A code rollback incompatible with the
  migrated schema requires restoring the database backup.
- Telemetry volumes are retained across redeploys.

## Day-2 operations

```bash
# End-to-end check
ansible-playbook playbooks/verify.yml --ask-vault-pass

# On a host
ssh runcodes@<host>
# If the user bus is not found over SSH:
export XDG_RUNTIME_DIR=/run/user/$(id -u)
systemctl --user status runcodes-<stack>
cd /opt/runcodes/compose/<stack>
podman-compose -p runcodes-<stack> -f compose.yml ps
podman-compose -p runcodes-<stack> -f compose.yml logs -f --tail=100 <service>
```

Container names are stable: `runcodes-edge-backend`, `runcodes-data-database`,
`runcodes-compute-judge`, `runcodes-obs-prometheus`, etc. Use `podman logs` and
`podman exec` for direct inspection.

### Restart one service

```bash
ssh runcodes@<host>
cd /opt/runcodes/compose/<stack>
podman-compose -p runcodes-<stack> -f compose.yml restart <service>
```

### Reboot

The `runcodes-*` units are enabled, so Podman stacks return at boot. The compute
unit waits for the rootless socket and removes/recreates the judge. After a
reboot, run `verify.yml`.

### Grafana / Prometheus / Alertmanager access

They are private. Tunnel through the edge:

```bash
ssh -L 3000:203.0.113.13:3000 \
    -L 9090:203.0.113.13:9090 \
    -L 9093:203.0.113.13:9093 \
    runcodes@203.0.113.10
```

Open <http://localhost:3000>, <http://localhost:9090>,
<http://localhost:9093>. Grafana uses `config_runcodes_grafana_admin_user` and
the vault password. The default home dashboard is `runcodes-overview`.

### UFW

If `config_runcodes_firewall_enabled: true`, UFW allows SSH from
`config_runcodes_firewall_ssh_cidrs`, all traffic from
`config_runcodes_firewall_private_sources`, the Podman container networks, and
80/443 only on the edge. It does not inspect container-published ports; the
node-address bindings and provider firewall are the real boundary. Test SSH
from a second session before closing the current one.

Setting the flag back to `false` skips the role; it does not disable an already
enabled UFW. Run `ufw disable` deliberately to roll it back.

## Backups and restore

The backup boundary is the `data` host. The database and SeaweedFS are the only
irreplaceable data. Redis is a cache/session store; losing it degrades the API
and forces re-login but does not lose submissions.

### PostgreSQL

```bash
ssh runcodes@203.0.113.11 \
  'podman exec runcodes-data-database pg_dump -U runcodes -d runcodes -Fc' \
  > "runcodes-$(date -u +%Y%m%dT%H%M%SZ).dump"
```

Add `-e PGPASSWORD=<owner-password>` to `podman exec` if local authentication
requires it. Store the dump encrypted and off-host. Stop writers first:

```bash
ssh runcodes@203.0.113.10 \
  'cd /opt/runcodes/compose/edge && podman-compose -p runcodes-edge -f compose.yml stop backend frontend'
ssh runcodes@203.0.113.12 \
  'cd /opt/runcodes/compose/compute && podman-compose -p runcodes-compute -f compose.yml stop judge'

# Ensure the database container is up (this starts/restarts the data unit).
ansible-playbook playbooks/deploy-data.yml --ask-vault-pass
ssh runcodes@203.0.113.11 \
  'podman exec -i runcodes-data-database pg_restore -U runcodes -d runcodes --clean --if-exists' \
  < runcodes-YYYY....dump
```

After a restore, run `deploy-edge.yml` and `deploy-compute.yml` to recreate the
edge/judge containers. Verify `/readyz` and run a submission.

### SeaweedFS

Stop writers, then archive the Podman volume:

```bash
ssh runcodes@203.0.113.10 \
  'cd /opt/runcodes/compose/edge && podman-compose -p runcodes-edge -f compose.yml stop backend'
ssh runcodes@203.0.113.12 \
  'cd /opt/runcodes/compose/compute && podman-compose -p runcodes-compute -f compose.yml stop judge'

ssh runcodes@203.0.113.11
cd /opt/runcodes/compose/data
podman-compose -p runcodes-data -f compose.yml stop seaweed
podman run --rm \
  -v runcodes-data_seaweed-data:/data:ro \
  -v /opt/runcodes/backups:/backup \
  alpine tar czf /backup/seaweed-$(date -u +%Y%m%dT%H%M%SZ).tgz -C /data .
podman-compose -p runcodes-data -f compose.yml start seaweed
```

The exact volume name is shown by `podman volume ls`; the project is
`runcodes-data`, so the default name is `runcodes-data_seaweed-data`. Redeploy
edge/compute afterwards.

### Redis (optional)

Redis uses AOF in `runcodes-data_redis-data`. It is safe to omit from backups.
If needed, stop Redis and archive the volume with `podman run` as above.

### Telemetry

Prometheus/Loki/Tempo/Pyroscope/Grafana volumes are retained across deploys and
capped by retention. Back up dashboards/provisioning from git. For long-term
telemetry, use object storage as described in the upstream production guidance.

## Troubleshooting

### Preflight fails before touching a host

- **"config.yml is missing"**: copy
  `inventories/group_vars/all/config.yml.example` to `config.yml`.
- **"exactly four hosts"**: one host per group, four total.
- **"distinct private_ip"**: every host has a unique node address.
- **"Secrets are missing..."**: vault not loaded or placeholders remain.
- **"contains a literal '$'"**: regenerate as hex/base64url.
- **"does not look like the RunCodes checkout"**: run from
  `deploy/ansible`.
- **Controller assets missing**: the preflight lists the missing paths; run
  from the repository root or set `runcodes_repo_root`.

### SSH, sudo or package installation fails

The deployment assumes key-based SSH and sudo. Check `ssh runcodes@<host> sudo
true`. If sudo needs a password, configure NOPASSWD or add
`ansible_become_password` (prefer vaulted). A failed `podman-compose` package
install usually means the host cannot reach apt repositories or the distribution
does not package `podman-compose`; Debian 12/13 and Ubuntu 22.04/24.04 do.

### The stack unit fails to start

```bash
systemctl --user status runcodes-<stack>
journalctl --user -u runcodes-<stack> -n 100 --no-pager
cd /opt/runcodes/compose/<stack>
podman-compose -p runcodes-<stack> -f compose.yml config
podman-compose -p runcodes-<stack> -f compose.yml up -d
```

A missing image is usually a GHCR tag/visibility/auth problem. Check
`podman pull <image>` as `runcodes` and the rootless registry login.

### `backend /readyz` returns 503

PostgreSQL is required. Check:

```bash
ssh runcodes@203.0.113.11 \
  'podman exec runcodes-data-database pg_isready -U runcodes -d runcodes'
ssh runcodes@203.0.113.10 \
  'cd /opt/runcodes/compose/edge && podman-compose -p runcodes-edge -f compose.yml logs --tail=100 backend'
```

Common cause: the app password changed in the vault but the data stage was not
run. Run data, then compute/edge.

### `judge /readyz` returns 503 with "podman unavailable"

```bash
ssh runcodes@203.0.113.12
systemctl --user status podman.socket          # as runcodes
ls -l /run/user/1000/podman/podman.sock        # must be 's', not 'd'
systemctl --user status runcodes-compute
journalctl --user -u runcodes-compute -n 100 --no-pager
```

If the socket is missing:

```bash
loginctl enable-linger runcodes
systemctl --user enable --now podman.socket
systemctl --user restart runcodes-compute
```

The compute unit waits for the socket and recreates the judge. `podman exec
runcodes-compute-judge ls -l /run/podman/podman.sock` must show a socket.

### Caddy does not get a certificate / 502 from the edge

- DNS must resolve `runcodes.example.com` to `203.0.113.10`.
- Ports 80 and 443 must reach the edge.
- `config_runcodes_domain` and `config_runcodes_public_url` must match.
- `podman-compose -p runcodes-edge -f compose.yml logs frontend`.
- A 502 for `/api/*` is usually the backend, not Caddy.

### SeaweedFS 403 / S3 signature errors

- Backend/judge use the vault S3 key/secret.
- SeaweedFS reads `/etc/seaweedfs/s3.json` at start; the data unit recreates it.
- The S3 endpoint must be `http://203.0.113.11:8333`, not `seaweed:8333`
  from another host.

### Redis `NOAUTH` / connection failures

The backend needs the same Redis password. After rotating, run data then edge.
Check interactively:

```bash
ssh -t runcodes@203.0.113.11 'podman exec -it runcodes-data-redis sh'
# inside the container; REDIS_PASSWORD is in its environment:
redis-cli --no-auth-warning -a "$REDIS_PASSWORD" ping
```

### No telemetry reaches the observability host

1. `podman exec runcodes-edge-backend printenv | grep OTEL_`.
2. `curl http://203.0.113.13:13133/health/status`.
3. Can edge/compute reach `203.0.113.13:4318`? Check provider firewall.
4. Check collector logs and `otelcol_exporter_*` metrics.
5. Check Prometheus targets; `verify.yml` prints down targets.

### Prometheus target down

Open Prometheus through the tunnel. Typical causes: a container stopped, a
private port blocked, an image/config error, or a password mismatch after
rotation. `lastError` names the cause.

### Alerts do not arrive at the receiver

- Empty `vault_alertmanager_webhook_url` sends everything to the bundled
  alert-sink (by design).
- Check `/opt/runcodes/secrets/alertmanager_webhook_url` and Alertmanager logs.
- The `ObservabilitySmokeTest` alert always goes to the sink.
- Test manually:

  ```bash
  curl -X POST http://203.0.113.13:9093/api/v2/alerts \
    -H 'Content-Type: application/json' \
    -d '[{"labels":{"alertname":"ManualTest","severity":"warning"}}]'
  curl http://203.0.113.13:8085/alerts
  ```

### Grafana login fails after a password change

The secret is a mounted file read at startup. The observability unit
force-recreates containers on deploy, so run `deploy-observability.yml`; or
restart Grafana with `podman-compose ... restart grafana`.

### Disk fills on the observability host

Prometheus/Loki/Tempo/Pyroscope have retention caps, but compactions need free
space. Check the disk alert. Lower retention or move to object storage. Do not
delete Podman volumes blindly.

### A config change did not take effect

The user stack units force-recreate containers on `systemctl --user restart`,
and Ansible restarts the unit on every stack deploy. If you edited a file
directly on the host, run the matching playbook or
`systemctl --user restart runcodes-<stack>`.

## Security hardening

- **Provider firewall is the primary boundary.** Restrict every non-public
  port to the four peers. Only 80/443 on the edge should face the internet.
- **SSH**: keys only, no password auth, restrict 22/tcp, use a bastion if the
  hosts are public.
- **UFW** is optional and does not filter container-published ports. Do not
  assume `ufw status` reflects container exposure; `ss -lntp` and the bind
  addresses do.
- **Rootless Podman is the trust boundary.** The service stacks and graded
  runner containers run as `runcodes`; container root maps to that
  unprivileged user, not host root. Treat the deployment user's storage and
  socket as sensitive.
- **Images**: GHCR packages should be public or pulled with a read-only token.
  Do not put a write token on the hosts. Pin tags for reproducible deploys and
  scan with the repository's Trivy workflow.
- **TLS**: only Caddy's 443 is public; it renews certificates. The private
  network is plaintext. Add WireGuard/IPsec or mTLS if you do not trust it.
- **Secrets**: encrypted vault, restricted files owned by `runcodes`, never
  committed. Rotate on a schedule and after suspected exposure.
- **Grafana** is private and has anonymous access, sign-up and plugin downloads
  disabled. Use SSO if exposed through a proxy. Never expose Prometheus,
  Alertmanager, Loki or Tempo directly.
- **OTLP public path** is limited to `/otel/v1/traces`, body-capped at 4 MiB.
  It is unauthenticated; rate-limit/WAF it or disable browser telemetry if
  abused.
- **Backups**: encrypt dumps, store off-host, restrict access, test restores.
- **Container logs** are in the Podman journal; they may contain user data.

## Known limits

- **No high availability.** One edge, database, object store, judge and
  observability host. A host failure is an outage.
- **Observability is a telemetry single point.** Apps keep serving; OTLP
  exports retry/drop.
- **No automatic schema migrations.** The data stage initializes a fresh volume
  and rotates the app role; migrations are explicit.
- **Stack deploys restart containers.** The user systemd units use
  `podman-compose up --force-recreate`; every deployment restarts the affected
  stack. Schedule maintenance windows.
- **Single-node telemetry storage.** Local volumes with retention caps, no
  clustering/object store.
- **No container-log shipping.** Application logs go over OTLP; infrastructure
  logs stay in the Podman journal/`podman-compose logs`.
- **Internal traffic is plaintext.** OTLP, PostgreSQL, Redis and S3 are
  plaintext on the node network.
- **Browser telemetry is public and unauthenticated.** Only traces are proxied;
  Tempo retention bounds the damage.
- **Runner tag defaults to `latest`.** Pre-pulled every deploy; pin for a
  coordinated release.
- **smtp4dev is not production mail.**
- **No multi-edge load balancer or autoscaling.**
- **Not a hardened multi-tenant boundary beyond the judge.** A backend
  compromise can read all submissions.
