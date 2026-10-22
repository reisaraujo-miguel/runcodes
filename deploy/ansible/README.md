# RunCodes four-machine Ansible deployment

This directory deploys the **full current RunCodes stack** onto exactly four
machines, including the complete observability stack:

- frontend (React SPA served by Caddy) and backend API;
- judge and the rootless Podman execution environment;
- PostgreSQL, Redis and SeaweedFS;
- OpenTelemetry Collector, Prometheus, Loki, Tempo, Pyroscope, Grafana,
  Alertmanager, host/database/cache exporters and blackbox probes.

## The four machines

The example inventory uses four documentation addresses:

| Inventory group | Address | Services |
| --- | --- | --- |
| `edge` | `203.0.113.10` | Caddy/SPA, backend, smtp4dev, node-exporter, Alloy |
| `data` | `203.0.113.11` | PostgreSQL, Redis, SeaweedFS, postgres/redis/node exporters |
| `compute` | `203.0.113.12` | Judge, rootless Podman, node-exporter, Alloy |
| `observability` | `203.0.113.13` | Collector, Prometheus, Alertmanager, alert-sink, Loki, Tempo, Pyroscope, Grafana, node/blackbox exporters |

Replace the addresses in `hosts.yml` and `config.yml` with your own. The
deployment treats the node addresses as the private/management network and
binds every non-public service to them. **The provider firewall must allow only
the four peers to reach those non-public ports**; only 80/443 on the edge should
be reachable from the internet. See
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md#network-and-ports).

The full placement rationale is in
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md#why-these-placements).

## Why this split

- **Edge is stateless and public.** It owns the only public surface. Keeping the
  backend next to Caddy preserves the same-origin `/api` design (no CORS, no
  build-time API URL) and the SSE stream.
- **Data groups the stateful services.** PostgreSQL, Redis and SeaweedFS are the
  backup/restore boundary. They are private and sized around disk and memory.
- **Compute is isolated.** The judge runs untrusted submissions through the
  deployment user's rootless Podman socket, with a shared execution directory
  and no public ports. Runner containers cannot touch the edge or the telemetry
  store.
- **Observability is dedicated.** Prometheus/Loki/Tempo/Pyroscope are disk-heavy
  and have a different upgrade cadence. A separate host keeps telemetry from
  competing with request handling.

## Runtime: rootless Podman everywhere

All four stacks run as the deployment user under **rootless Podman** via
`podman-compose`. Each stack has a **user** systemd unit (`runcodes-edge`,
`runcodes-data`, `runcodes-compute`, `runcodes-observability`) enabled through
`loginctl enable-linger`. The judge and the runner containers share the same
rootless Podman socket; the judge uses `userns_mode: keep-id` so `/exec` and
the socket have the deployment user's ownership. No Docker daemon or Docker
socket is used.

Rootless Podman publishes Caddy's 80/443 through `rootlessport`; the hosts set
`net.ipv4.ip_unprivileged_port_start=80`. Port 22 and other privileged ports
remain restricted.

## Images: built by GitHub Actions, pulled by Ansible

The four application images are built and published to GHCR by
[`.github/workflows/app-images.yml`](../../.github/workflows/app-images.yml):

- `ghcr.io/runcodes-icmc/runcodes-frontend`
- `ghcr.io/runcodes-icmc/runcodes-backend`
- `ghcr.io/runcodes-icmc/runcodes-database`
- `ghcr.io/runcodes-icmc/runcodes-judge`

The deployment pulls them (default tag `latest`) and never builds them on the
hosts. The frontend is built with the same-origin `/otel` browser telemetry
endpoint baked in. Runner images keep their own `runners.yml` workflow and use
`runcodes_runner_tag` (default `latest`); the compute stage pre-pulls every
language into rootless Podman storage before deploying the judge.

`config.yml.example` uses the placeholder namespace `runcodes-example`; set
`config_runcodes_image_namespace` and `config_runcodes_runner_image_prefix` to
the registry/namespace that publishes your images.

If the GHCR packages are private, put a token in
`vault_registry_username`/`vault_registry_password`; the deployment user logs
in to `config_runcodes_image_registry` once during setup.

## Prerequisites

- Four Debian 12/13 or Ubuntu 22.04/24.04 hosts with systemd. If the hosts
  already run services on the same ports, stop them (or reimage the hosts)
  before the first run.
- SSH access as a sudo-capable user (`runcodes` by default), preferably with a
  key and `NOPASSWD` sudo. Put an existing SSH password in a vaulted inventory
  var if the hosts still require one.
- The four node addresses above, reachable from the controller and from each
  other.
- A DNS name for the edge host (`runcodes.example.com` in the example) with
  ports 80/443 open. An IP-only HTTP deployment is possible with
  `config_runcodes_domain: ":80"`.
- On the controller: Python 3.11+, `ansible-core` 2.17–2.19 and the
  collection from [`requirements.yml`](requirements.yml).
- Outbound internet access from each host for apt and GHCR.
- The app-images workflow must have published the images, and the runner images
  must exist for `runcodes_runner_tag`.

## Configuration and secrets

Tracked files contain no deployment values:

- `inventories/hosts.yml` (gitignored) — the four addresses.
- `inventories/group_vars/all/config.yml` (gitignored) — **every** tunable
  value; copy from `config.yml.example`.
- `inventories/group_vars/all/vault.yml` (gitignored, ansible-vault) — secrets;
  copy from `vault.yml.example`.
- `inventories/group_vars/all/main.yml` (tracked) — only maps `config_*` and
  `vault_*` names to the `runcodes_*` names used by the roles.

## Quick start

```bash
cd deploy/ansible

# 1. Controller dependencies (vendored under .ansible-home/)
python3 -m venv .ansible-venv
. .ansible-venv/bin/activate
pip install 'ansible-core>=2.17,<2.20'
ansible-galaxy collection install -r requirements.yml -p .ansible-home/collections

# 2. Inventory
cp inventories/hosts.yml.example inventories/hosts.yml
$EDITOR inventories/hosts.yml

# 3. Deployment configuration (not tracked)
cp inventories/group_vars/all/config.yml.example inventories/group_vars/all/config.yml
$EDITOR inventories/group_vars/all/config.yml

# 4. Secrets (not tracked; encrypted)
cp inventories/group_vars/all/vault.yml.example inventories/group_vars/all/vault.yml
$EDITOR inventories/group_vars/all/vault.yml
ansible-vault encrypt inventories/group_vars/all/vault.yml

# 5. Preflight, then deploy
ansible-playbook playbooks/site.yml --ask-vault-pass
```

`site.yml` runs preflight, host setup (Podman/podman-compose and deploy assets),
data, observability, compute, edge, and verification. The first run pulls the
application images and pre-pulls the runner images; subsequent runs pull image
updates and recreate the stack containers through their user systemd units.

To run individual stages:

```bash
ansible-playbook playbooks/setup.yml --ask-vault-pass
ansible-playbook playbooks/deploy-data.yml --ask-vault-pass
ansible-playbook playbooks/deploy-observability.yml --ask-vault-pass
ansible-playbook playbooks/deploy-compute.yml --ask-vault-pass
ansible-playbook playbooks/deploy-edge.yml --ask-vault-pass
ansible-playbook playbooks/verify.yml --ask-vault-pass
```

## First deployment checklist

1. **DNS** points `config_runcodes_domain` at the edge public address.
2. **Provider firewall** allows:
   - 22/tcp from the operator ranges to all four hosts;
   - 80/tcp and 443/tcp from anywhere to the edge host;
   - only the four node addresses to reach the non-public ports.
3. **Secrets** are random hex/base64url and contain no literal `$`
   (`podman-compose` interpolates `.env` values). Preflight rejects `$` and the
   `REPLACE_WITH` placeholders.
4. **GHCR images** exist for `config_runcodes_image_tag` and the runner tag.
   The app-images workflow can be run manually before the first deployment.
   GHCR packages are private by default: either set them public or put a
   `read:packages` token in `vault_registry_username`/`password`. The compute
   pre-pull uses the same token for the runner images.
5. **Admin password**: the seeded `admin@admin.com` account is bootstrapped only
   while its hash is the unusable seed value `!`. Later UI changes are preserved.
6. **Runner tag**: `latest` is supported; the compute stage pre-pulls all
   languages. Change `config_runcodes_runner_tag` only for a coordinated release.
7. **HTTP-only labs**: set `config_runcodes_backend_debug_mode: true` so the
   session cookie is not marked `Secure` over plain HTTP. Never on HTTPS.
8. **Mail**: smtp4dev is a catcher, not a mail server. Set the relay in
   `config.yml` and the credentials in `vault.yml` for real verification mail.

The deployment does **not** apply Atlas migrations automatically. A fresh
database volume is initialized from the schema baked into the database image;
an existing database must be migrated manually as described in
[`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md#database-lifecycle-and-migrations).
It also does **not** import an existing database or SeaweedFS objects: see
[`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md#migrating-existing-data)
and [`database/legacy-migration/`](../../database/legacy-migration/README.md)
before reusing existing hosts.

## Operating the stack

```bash
# Full status
ansible-playbook playbooks/verify.yml --ask-vault-pass

# One stack
ssh runcodes@<host>
cd /opt/runcodes/compose/<edge|data|compute|observability>
podman-compose -p runcodes-<stack> -f compose.yml ps
podman-compose -p runcodes-<stack> -f compose.yml logs -f --tail=100 <service>

# The user systemd units own the stack lifecycle (and start at boot):
systemctl --user status runcodes-<stack>
systemctl --user restart runcodes-<stack>
# If the user bus is not found over SSH:
# export XDG_RUNTIME_DIR=/run/user/$(id -u)

# Grafana, Prometheus and Alertmanager are private. Tunnel through the edge:
ssh -L 3000:203.0.113.13:3000 \
    -L 9090:203.0.113.13:9090 \
    -L 9093:203.0.113.13:9093 \
  runcodes@203.0.113.10
# then open http://localhost:3000 (Grafana), :9090 (Prometheus), :9093 (Alertmanager)
```

The Grafana admin password is in the vault (`vault_grafana_admin_password`) and
is written to `/opt/runcodes/secrets/grafana_admin_password` on the
observability host, mode 0644 inside a 0700 directory.

## Verification and observability documentation

- [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) — inventory, config/vault, day-1,
  Podman lifecycle, upgrades, backups, migrations, troubleshooting, hardening.
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — topology, placement, port
  matrix, data flow, failure domains and capacity.
- [`docs/OBSERVABILITY.md`](docs/OBSERVABILITY.md) — signal inventory, routing,
  browser tracing, alerting, validation queries and runbooks.

`playbooks/verify.yml` waits for every health endpoint, asserts every
Prometheus target and blackbox probe is up, checks application metrics and the
judge queue gauge, and (unless `config_runcodes_run_smoke_test: false`) runs the
upstream end-to-end smoke test. Set
`-e runcodes_run_submission_smoke_test=true` to additionally create a
disposable exercise and submission and assert the judge's per-case span and
span-to-profile link.

## Scope and known limits

This is a **single-instance, four-host** deployment. It is production-shaped
(private networking, published images, secrets in a vault, retention limits)
but not highly available: each service has one replica. The observability host
is a single point of failure for telemetry. The database and object store are
single instances and must be backed up. Stack units use
`podman-compose up --force-recreate`, so a deployment restarts the containers in
that stack; schedule it in a maintenance window. See the "Known limits" section
of [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md#known-limits).
