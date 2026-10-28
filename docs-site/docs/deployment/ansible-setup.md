---
title: "Ansible deployment: setup"
description: Prepare four hosts, inventory, configuration and secrets, then run the first four-machine deployment.
---

# Ansible deployment: setup

This is the operator setup guide for [`deploy/ansible/`](https://github.com/reisaraujo-miguel/runcodes/tree/main/deploy/ansible).
It deploys the full RunCodes stack — frontend/backend, PostgreSQL, Redis,
SeaweedFS, judge, runner images and the complete observability stack — onto four
machines with **rootless Podman**.

Day-2 operations are covered in [Ansible operations](./ansible-operations.md).

## What gets deployed

| Inventory group | Services |
| --- | --- |
| `edge` | Caddy + React SPA, backend API, smtp4dev, node-exporter, Alloy |
| `data` | PostgreSQL, Redis, SeaweedFS, postgres/redis/node exporters |
| `compute` | Judge, rootless Podman, runner containers, node-exporter, Alloy |
| `observability` | OTel Collector, Prometheus, Alertmanager, alert-sink, Loki, Tempo, Pyroscope, Grafana, node/blackbox exporters |

Each stack runs as the deployment user (`runcodes` by default) through a **user**
systemd unit: `runcodes-edge`, `runcodes-data`, `runcodes-compute`,
`runcodes-observability`. `loginctl enable-linger` starts the user manager at
boot.

## Prerequisites

### Hosts

- Four Debian 12/13 or Ubuntu 22.04/24.04 hosts, x86_64, systemd.
- A sudo-capable login user (`runcodes` by default) with key-based SSH,
  preferably `NOPASSWD` sudo.
- The four node addresses assigned to the right hosts, reachable from the
  controller and from each other.
- Outbound HTTPS for apt and GHCR.
- At least the resources in [Capacity](./security.md#capacity-starting-points):
  roughly 2 vCPU/4 GB for edge, 4 vCPU/8 GB for data, 4 vCPU/8–16 GB for
  compute, 4 vCPU/8–16 GB and 200–400 GB SSD for observability.

If the hosts already run services on the same ports, stop them or reimage the
hosts. The deployment binds 80/443, 5432, 6379 and 8333 and uses fixed container
names (`runcodes-*`).

### Controller

- Python 3.11+.
- `ansible-core` 2.17–2.19.
- The `community.general` collection from `requirements.yml`.
- `ansible-vault` for the secrets.
- SSH access to all four hosts.

The local `ansible.cfg` installs collections under `./.ansible-home/`, so the
system Python is not modified.

### Provider firewall

Set these rules in the cloud/provider firewall, not only in UFW:

| Source | Destination | Ports |
| --- | --- | --- |
| Operator/VPN ranges | all four hosts | 22/tcp |
| Anywhere | edge public IP | 80/tcp, 443/tcp |
| The four node addresses | all four hosts | all traffic among the four peers |

Do **not** expose PostgreSQL, Redis, SeaweedFS, the judge, Prometheus, Grafana,
Loki, Tempo, Pyroscope, Alertmanager or the collector to the internet. They bind
to the private node address and are meant to be reachable only by the peers.

## 1. Inventory

```bash
cd deploy/ansible
cp inventories/hosts.yml.example inventories/hosts.yml
$EDITOR inventories/hosts.yml
```

The example has one host per group:

| Group | Example address | Role |
| --- | --- | --- |
| `edge` | `203.0.113.10` | public SPA/API edge |
| `data` | `203.0.113.11` | PostgreSQL, Redis, SeaweedFS |
| `compute` | `203.0.113.12` | judge and runner host |
| `observability` | `203.0.113.13` | telemetry storage and query |

Each host needs:

| Variable | Meaning |
| --- | --- |
| `ansible_host` | address Ansible connects to |
| `private_ip` | address peers use; every private service binds here |
| `public_ip` | edge only; the address DNS resolves to |
| `ansible_user` | SSH user (default `runcodes`) |
| `ansible_become` | `true`; the user needs sudo |

The preflight enforces exactly one host in each group, four hosts total, and four
distinct non-empty `private_ip` values. If your SSH user is not `runcodes`, set
both `ansible_user` and `config_runcodes_deploy_user`.

Prefer keys and NOPASSWD sudo. If a host still requires a password, put
`ansible_ssh_pass` / `ansible_become_password` in vaulted inventory vars, never
in the tracked example.

Ansible verifies SSH host keys. Before the first run, add the four keys after
checking their fingerprints against a source you trust (provider console,
serial console, or a previous `known_hosts`), then scan them:

```bash
ssh-keyscan -H 203.0.113.10 203.0.113.11 203.0.113.12 203.0.113.13 \
  >> ~/.ssh/known_hosts
```

`ssh-keyscan` only learns the key the host presents; the fingerprint check is
what prevents a man-in-the-middle from receiving vault-derived secrets on the
first deployment.

## 2. Configuration

Every deployment value lives in an untracked file:

```bash
cp inventories/group_vars/all/config.yml.example \
   inventories/group_vars/all/config.yml
$EDITOR inventories/group_vars/all/config.yml
```

`config.yml` is gitignored. The tracked `main.yml` only maps `config_*` names to
the `runcodes_*` names used by roles; do not put values there.

The most important settings:

| Setting | Default / notes |
| --- | --- |
| `config_runcodes_cluster_name`, `_environment` | labels for Prometheus, Tempo, Alloy and OTel |
| `config_runcodes_version` | `service.version` for backend/judge; use the release you deploy |
| `config_runcodes_image_registry` / `_namespace` / `_tag` | application images (default `ghcr.io` / `runcodes-icmc` / `latest`) |
| `config_runcodes_pull_images` | pull app/third-party images before restarting a stack |
| `config_runcodes_runner_tag` | runner image tag (`latest` supported) |
| `config_runcodes_runner_image_prefix` | full prefix for runner images, ending in `-`; for example `ghcr.io/reisaraujo-miguel/runcodes-runner-` |
| `config_runcodes_prepull_runner_images` | pre-pull every language before deploying the judge |
| `config_runcodes_runner_languages` | languages to pre-pull; matches `runners/` |
| `config_runcodes_domain` / `_public_url` | hostname and HTTPS origin |
| `config_runcodes_backend_debug_mode` | only for HTTP-only labs; makes the cookie non-Secure |
| `config_runcodes_judge_*` | concurrency, execution directory, timeouts, cgroup limits |
| `config_runcodes_otel_*` | sampler and metric export interval |
| `config_runcodes_prometheus_retention_*`, `_tempo_retention_hours` | retention |
| `config_runcodes_alertmanager_gmail_*` | optional Gmail alert email; see [Gmail alert email](#gmail-alert-email) |
| `config_runcodes_firewall_*` | optional UFW; review SSH sources first |
| `config_runcodes_run_smoke_test` / `_run_submission_smoke_test` | verification toggles |

The complete list with defaults is in
[Configuration § Ansible deployment](../reference/configuration.md#ansible-deployment).

## 3. Secrets

```bash
cp inventories/group_vars/all/vault.yml.example \
   inventories/group_vars/all/vault.yml
$EDITOR inventories/group_vars/all/vault.yml
ansible-vault encrypt inventories/group_vars/all/vault.yml
```

| Vault variable | Used by | Generate with |
| --- | --- | --- |
| `vault_runcodes_jwt_secret` | session signing | `openssl rand -hex 32` |
| `vault_runcodes_judge_token` | backend ↔ judge | `openssl rand -hex 32` |
| `vault_runcodes_db_owner_password` | PostgreSQL owner / migrations | `openssl rand -hex 24` |
| `vault_runcodes_db_app_password` | `runcodes_app` (backend) | `openssl rand -hex 24` |
| `vault_runcodes_db_judge_password` | `runcodes_judge` (judge) | `openssl rand -hex 24` |
| `vault_runcodes_s3_access_key` | SeaweedFS S3 | `openssl rand -hex 16` |
| `vault_runcodes_s3_secret_key` | SeaweedFS S3 | `openssl rand -hex 24` |
| `vault_runcodes_redis_password` | Redis `requirepass` | `openssl rand -hex 24` |
| `vault_runcodes_admin_password` | seeded admin | strong password, no `$` |
| `vault_grafana_admin_password` | Grafana admin | strong password, no `$` |
| `vault_runcodes_legacy_password_salt` | legacy users (optional) | old platform salt |
| `vault_runcodes_smtp_username/password` | SMTP relay (optional) | relay credentials |
| `vault_alertmanager_webhook_url` | alert receiver (optional) | incident tool URL |
| `vault_alertmanager_gmail_app_password` | Gmail alert email (optional) | Google app password |
| `vault_registry_username/password` | private GHCR (optional) | token with `read:packages` |

::: danger No literal `$` in secrets
`podman-compose` interpolates `.env` values, so a `$` can silently corrupt a
password. Hex/base64url output is safe. The preflight rejects `$` and the
`REPLACE_WITH` placeholders.
:::

Secrets are rendered into `.env` files and secret files owned by the deployment
user on the hosts. `.env` files are 0600; mounted secret files are left readable
by the service container that consumes them. They are never committed and never
baked into images.

## 4. Application images

The four application images are built and pushed by
`.github/workflows/app-images.yml` on `v*.*.*` tags and manual dispatch. The
workflow derives the namespace from the repository; the upstream repository
publishes:

- `ghcr.io/runcodes-icmc/runcodes-frontend`
- `ghcr.io/runcodes-icmc/runcodes-backend`
- `ghcr.io/runcodes-icmc/runcodes-database`
- `ghcr.io/runcodes-icmc/runcodes-judge`

A fork publishes the same names under its own GHCR namespace. The workflow
publishes `latest`, `sha-<short>` and semver tags. The deployment defaults to
`latest`; pin `config_runcodes_image_tag` for reproducible deploys.

The frontend image is built with `VITE_OTEL_EXPORTER_OTLP_ENDPOINT=/otel`, so the
SPA emits browser traces to the same-origin edge route. No deployment hostname is
baked in.

Runner and monitor images are published by `runners.yml`, which calls
`monitor.yml` before building the base; the compute stage pre-pulls every
configured language before deploying the judge.

GHCR packages are **private by default**. Either make the application, runner and
monitor packages public, or create a token with `read:packages` and put it in
`vault_registry_username` / `vault_registry_password`. The compute pre-pull uses
the same credentials.

## 5. First deployment

```bash
cd deploy/ansible

python3 -m venv .ansible-venv
. .ansible-venv/bin/activate
pip install 'ansible-core>=2.17,<2.20'
ansible-galaxy collection install -r requirements.yml -p .ansible-home/collections

ansible-playbook playbooks/site.yml --ask-vault-pass
```

The first run is the slowest: it installs Podman and `podman-compose`, copies the
deploy assets, pulls images, pre-pulls the runners, starts four stacks and runs
verification.

| Stage | Effect |
| --- | --- |
| `preflight.yml` | validates inventory, config, assets, secrets and firewall inputs; touches no host |
| `setup.yml` | base packages, Podman/podman-compose, assets, optional UFW, subordinate IDs, linger, user manager, `podman.socket`, registry login |
| `deploy-data.yml` | pulls/starts PostgreSQL, Redis, SeaweedFS and exporters; waits for health; rotates the app password; bootstraps the admin if unset |
| `deploy-observability.yml` | writes configs, starts the telemetry stack, waits for readiness |
| `deploy-compute.yml` | pre-pulls runner images, starts the judge with the rootless socket, waits for `/readyz` |
| `deploy-edge.yml` | pulls/starts Caddy, API, mail catcher and agents, waits for `/healthz` and `/readyz` |
| `verify.yml` | waits for endpoints, asserts Prometheus targets and blackbox probes, checks metrics, runs the smoke test |

If a stage fails, fix the cause and rerun `site.yml`; it is idempotent. The data
stage will not re-bootstrap the admin after the first successful run.

To run individual stages:

```bash
ansible-playbook playbooks/setup.yml --ask-vault-pass
ansible-playbook playbooks/deploy-data.yml --ask-vault-pass
ansible-playbook playbooks/deploy-observability.yml --ask-vault-pass
ansible-playbook playbooks/deploy-compute.yml --ask-vault-pass
ansible-playbook playbooks/deploy-edge.yml --ask-vault-pass
ansible-playbook playbooks/verify.yml --ask-vault-pass
```

## 6. DNS, TLS and mail

### Normal HTTPS

1. Create an `A`/`AAAA` record for `config_runcodes_domain` pointing at the edge
   `public_ip`.
2. Set `config_runcodes_public_url` to `https://<domain>`.
3. Allow 80/443 to the edge (ACME HTTP-01 needs 80).
4. Run the edge stage. Caddy stores certificates in the
   `runcodes-edge_caddy_data` volume.

### HTTP-only / no DNS

```yaml
config_runcodes_domain: ":80"
config_runcodes_public_url: "http://203.0.113.10"
config_runcodes_backend_debug_mode: true
```

Caddy serves plain HTTP. Debug mode is required because the backend otherwise
marks the session cookie `Secure`, and the browser will not send it over plain
HTTP. Never enable it on an HTTPS production deployment.

### TLS behind another proxy

Keep `config_runcodes_public_url` as the external HTTPS origin, set
`config_runcodes_domain: ":80"`, and forward the original `Host` and
`X-Forwarded-*` headers to Caddy. The collector CORS list uses the public URL, so
it matches the browser origin.

### Mail

The default runs **smtp4dev** on the edge and points the backend at
`smtp4dev:25`. It catches every message; the UI is on edge loopback
`127.0.0.1:8081`. This is for bootstrap and testing only.

For real mail, set in `config.yml`:

```yaml
config_runcodes_smtp_host: "smtp.example.org"
config_runcodes_smtp_port: 587
config_runcodes_smtp_from: "RunCodes <no-reply@runcodes.example.com>"
```

and the optional credentials in `vault.yml`:

```yaml
vault_runcodes_smtp_username: "..."
vault_runcodes_smtp_password: "..."
```

STARTTLS is used when the relay advertises it. `config_runcodes_public_url` must
be the browser-facing origin because it builds the verification and recovery
links. The
smtp4dev container remains but is unused; remove it from
`roles/rc-edge/files/compose.yml` and the backend `depends_on` to drop it.

### Gmail alert email

Alertmanager on the observability host can email alerts through Gmail. This is
separate from the account-verification SMTP settings above. Enable it in
`config.yml`:

```yaml
config_runcodes_alertmanager_gmail_enabled: true
config_runcodes_alertmanager_gmail_from: "runcodes-alerts@gmail.com"
config_runcodes_alertmanager_gmail_to:
  - "oncall@example.com"
```

and put a Google app password in `vault.yml`:

```yaml
vault_alertmanager_gmail_app_password: "abcdefghijklmnop"
```

The sending account needs 2-Step Verification; use an app password, not the
account password, and remove the spaces Google displays. The `from` address is
also the SMTP username, so use a plain address. Ansible writes the password to
`/opt/runcodes/secrets/alertmanager_gmail_app_password`; Gmail is added to the
`default` receiver as an additional integration, so webhook delivery continues.
The smoke-test alert still goes to `alert-sink`. For a Google Workspace SMTP
relay, set `config_runcodes_alertmanager_gmail_smarthost`. See
[Ansible operations](./ansible-operations.md#alerts-do-not-arrive-at-the-receiver)
for SMTP failure modes.

## 7. Admin bootstrap

The database seed creates `admin@admin.com` with the unusable hash `!`. The data
stage runs `bootstrap-admin.sh` only while the hash is still `!`; later UI
changes are preserved.

Manual reset on the data host:

```bash
ssh runcodes@203.0.113.11
podman exec -it -e PGPASSWORD='<db-owner-password>' \
  -e RUNCODES_ADMIN_PASSWORD='<new-admin-password>' \
  -e RUNCODES_ADMIN_EMAIL='admin@admin.com' \
  runcodes-data-database /bootstrap/bootstrap-admin.sh
```

The script is mounted read-only from `/opt/runcodes/bootstrap/bootstrap-admin.sh`
and hashes the password with bcrypt cost 12.

## 8. First deployment checklist

1. **DNS** resolves `config_runcodes_domain` to the edge public address.
2. **Provider firewall** allows 22 from operators, 80/443 to the edge, and only
   the four peers to the private ports.
3. **Secrets** are random, contain no `$`, and the vault is encrypted.
4. **Images** exist for `config_runcodes_image_tag` and the runner tag; GHCR
   packages are public or the read token is valid.
5. **Admin password** is set in the vault.
6. **Runner tag** is intentional. `latest` is supported and pre-pulled; pin only
   for a coordinated release.
7. **HTTP-only labs** set `config_runcodes_backend_debug_mode: true`; HTTPS
   deployments leave it false.
8. **Mail**: smtp4dev is a catcher, not a mail server.
9. **Migrations**: the deployment does not apply Atlas migrations automatically.
   A fresh volume is initialized from the schema baked into the database image;
   an existing database must be migrated manually.

## Runtime model

- All four stacks run as `runcodes` under rootless Podman via `podman-compose`,
  managed by user systemd units.
- Each unit runs `podman-compose up -d --force-recreate` and stops with
  `podman-compose stop`. It is enabled under `default.target`; linger starts it
  at boot.
- Rootless Podman publishes Caddy's 80/443 through `rootlessport`;
  `net.ipv4.ip_unprivileged_port_start=80` lets the user bind them.
- `--force-recreate` is deliberate: Podman Compose does not diff container
  configuration like Docker Compose, so the unit recreates containers to apply
  image, `.env` and mounted-config changes. A deployment is a maintenance-window
  operation.
- The judge and graded runners share the rootless `podman.socket`. The judge uses
  `userns_mode: keep-id` and mounts `/run/user/<uid>/podman`, so a socket restart
  is visible without recreating the judge.
- Application images are pulled by Ansible as `runcodes` before the unit
  restarts; at boot the unit uses local images.
- The judge's `/readyz` is the authoritative check for PostgreSQL and Podman.

## Next

- [Ansible operations](./ansible-operations.md) — upgrades, backups, migrations,
  troubleshooting.
- [Four-machine observability](../observability/production.md) — what the
  observability host runs and how to use it.
- [Security & scaling](./security.md) — hardening and growth.
