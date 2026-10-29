# RunCodes four-machine Ansible deployment

Deploys the full RunCodes stack — frontend/backend, judge, PostgreSQL, Redis,
SeaweedFS and the complete observability stack — onto exactly four machines
with **rootless Podman**.

**Full documentation:**

- [Ansible setup](https://reisaraujo-miguel.github.io/runcodes/deployment/ansible-setup.html)
- [Ansible operations](https://reisaraujo-miguel.github.io/runcodes/deployment/ansible-operations.html)
- [Deployment architecture](https://reisaraujo-miguel.github.io/runcodes/deployment/security.html)
- [Four-machine observability](https://reisaraujo-miguel.github.io/runcodes/observability/production.html)

## The four machines

| Group | Example address | Services |
| --- | --- | --- |
| `edge` | `203.0.113.10` | Caddy/SPA, backend, smtp4dev, node-exporter, Alloy |
| `data` | `203.0.113.11` | PostgreSQL, Redis, SeaweedFS, exporters |
| `compute` | `203.0.113.12` | Judge, rootless Podman, runners, node-exporter, Alloy |
| `observability` | `203.0.113.13` | Collector, Prometheus, Alertmanager, Loki, Tempo, Pyroscope, Grafana |

The addresses are RFC 5737 examples. The deployment treats them as the
private/management network; only 80/443 on the edge should be public.

Each stack runs as a **user** systemd unit (`runcodes-edge`, `runcodes-data`,
`runcodes-compute`, `runcodes-observability`). `setup.yml` enables systemd
lingering for the deployment user and fails if the host does not report
`Linger=yes`: without it, logind stops the user manager when the last SSH session
closes and the whole stack exits `0`/`143` about ten seconds later, and the units
never start at boot.

## Prerequisites

- Four Debian 12/13 or Ubuntu 22.04/24.04 hosts with systemd and x86_64.
- A sudo-capable SSH user (`runcodes` by default), preferably with a key and
  NOPASSWD sudo.
- The four node addresses assigned and reachable from the controller and each
  other.
- On the controller: Python 3.11+, `ansible-core` 2.17–2.19, and `git`.
- GHCR images published for the release and runner tag, or a `read:packages`
  token if the packages are private.

## Quick start

```bash
cd deploy/ansible

python3 -m venv .ansible-venv
. .ansible-venv/bin/activate
pip install 'ansible-core>=2.17,<2.20'
ansible-galaxy collection install -r requirements.yml -p .ansible-home/collections

cp inventories/hosts.yml.example inventories/hosts.yml
$EDITOR inventories/hosts.yml

cp inventories/group_vars/all/config.yml.example \
   inventories/group_vars/all/config.yml
$EDITOR inventories/group_vars/all/config.yml

cp inventories/group_vars/all/vault.yml.example \
   inventories/group_vars/all/vault.yml
$EDITOR inventories/group_vars/all/vault.yml
ansible-vault encrypt inventories/group_vars/all/vault.yml

# Once, before the first run: add the four host keys after verifying their
# fingerprints out of band (provider console, serial console, or a prior
# known-hosts entry). Host-key checking is enabled.
ssh-keyscan -H 203.0.113.10 203.0.113.11 203.0.113.12 203.0.113.13 \
  >> ~/.ssh/known_hosts

ansible-playbook playbooks/site.yml --ask-vault-pass
```

`site.yml` runs preflight, host setup, data, observability, compute, edge and
verification. It is idempotent; rerun it after fixing a failed stage. Individual
stages are available as `playbooks/setup.yml`, `deploy-data.yml`,
`deploy-observability.yml`, `deploy-compute.yml`, `deploy-edge.yml` and
`verify.yml`.

## Configuration and secrets

- `inventories/hosts.yml` (gitignored) — the four addresses.
- `inventories/group_vars/all/config.yml` (gitignored) — every tunable value.
- `inventories/group_vars/all/vault.yml` (gitignored, encrypted) — secrets.
- `inventories/group_vars/all/main.yml` (tracked) — maps names only.

Use hex/base64url secrets with **no literal `$`** (`podman-compose` interpolates
`.env` values). Preflight rejects `$` and `REPLACE_WITH` placeholders. The full
variable list, generated secret commands and image-publishing notes are in the
[setup guide](https://reisaraujo-miguel.github.io/runcodes/deployment/ansible-setup.html).

## Operating

```bash
# End-to-end check
ansible-playbook playbooks/verify.yml --ask-vault-pass

# On a host
ssh runcodes@<host>
export XDG_RUNTIME_DIR=/run/user/$(id -u)
systemctl --user status runcodes-<stack>
cd /opt/runcodes/compose/<stack>
podman-compose -p runcodes-<stack> -f compose.yml ps
podman-compose -p runcodes-<stack> -f compose.yml logs -f --tail=100 <service>
```

Upgrades, rollback, backups, database migrations, data import, troubleshooting
and the security model are in the
[operations guide](https://reisaraujo-miguel.github.io/runcodes/deployment/ansible-operations.html).

## Known scope

This is a **single-instance, four-host** deployment. It is production-shaped
(private networking, published images, vaulted secrets, retention limits) but not
highly available: each service has one replica, and a host failure is an outage.
See [Known limits](https://reisaraujo-miguel.github.io/runcodes/deployment/security.html#known-limits).
