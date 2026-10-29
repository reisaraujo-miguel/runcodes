---
title: "Ansible deployment: operations"
description: Day-2 operations for the four-machine deployment — verification, upgrades, backups, migrations, data import and troubleshooting.
---

# Ansible deployment: operations

This guide assumes the four-machine deployment from
[Ansible setup](./ansible-setup.md) is running. It covers verification, routine
operations, upgrades, migrations, backups and the failure modes seen most often.

## Verification

```bash
cd deploy/ansible
ansible-playbook playbooks/verify.yml --ask-vault-pass
```

`verify.yml`:

1. waits for every health endpoint;
2. asserts every active Prometheus target is `up`;
3. asserts every blackbox probe succeeds;
4. checks application metrics and the judge queue gauge;
5. runs the upstream observability smoke test from the observability host
   (unless `config_runcodes_run_smoke_test: false`).

To also exercise a real submission:

```bash
ansible-playbook playbooks/verify.yml --ask-vault-pass \
  -e runcodes_run_submission_smoke_test=true
```

To skip the slower smoke test:

```bash
ansible-playbook playbooks/verify.yml --ask-vault-pass \
  -e runcodes_run_smoke_test=false
```

## Day-2 operations

### Status and logs

```bash
ssh runcodes@<host>
# If the user bus is not found over SSH:
export XDG_RUNTIME_DIR=/run/user/$(id -u)

systemctl --user status runcodes-<stack>
cd /opt/runcodes/compose/<stack>
podman-compose -p runcodes-<stack> -f compose.yml ps
podman-compose -p runcodes-<stack> -f compose.yml logs -f --tail=100 <service>
```

Container names are stable: `runcodes-edge-backend`, `runcodes-data-database`,
`runcodes-compute-judge`, `runcodes-obs-prometheus`, and so on. `podman logs`
and `podman exec` work directly.

### Restart one service

```bash
ssh runcodes@<host>
cd /opt/runcodes/compose/<stack>
podman-compose -p runcodes-<stack> -f compose.yml restart <service>
```

This restarts the container but does not re-read changed mounted config. For a
config change, restart the **unit** (it force-recreates) or rerun the playbook:

```bash
systemctl --user restart runcodes-<stack>
```

### Reboot

The `runcodes-*` units are enabled, so the stacks return at boot. The compute
unit requires `podman.socket`, waits for it, and recreates the judge. After a
reboot, run `verify.yml`.

### Grafana, Prometheus, Alertmanager

They are private. Tunnel through the edge:

```bash
ssh -L 3000:203.0.113.13:3000 \
    -L 9090:203.0.113.13:9090 \
    -L 9093:203.0.113.13:9093 \
    -L 4040:203.0.113.13:4040 \
    runcodes@203.0.113.10
```

Then open <http://localhost:3000> (Grafana), <http://localhost:9090>
(Prometheus), <http://localhost:9093> (Alertmanager) and
<http://localhost:4040> (Pyroscope). Grafana uses
`config_runcodes_grafana_admin_user` and `vault_grafana_admin_password`.

### UFW

If `config_runcodes_firewall_enabled: true`, UFW allows SSH from the configured
CIDRs, all traffic from the private peers, the Podman container networks, and
80/443 only on the edge. It does **not** inspect container-published ports; the
node-address bindings and provider firewall are the real boundary. Test SSH from
a second session before closing the current one.

Setting the flag back to `false` skips the role; it does not disable an already
enabled UFW. Run `ufw disable` deliberately to roll it back.

## Upgrades and rollback

### Application upgrade

1. Merge/tag the release; `app-images.yml` publishes the new images.
2. Set `config_runcodes_image_tag` to the release tag (or keep `latest`, which
   moves).
3. If the schema changed, read the migration notes and back up.
4. Run:

   ```bash
   ansible-playbook playbooks/site.yml --ask-vault-pass
   ```

The stack units pull the configured images and force-recreate their containers.
The judge restarts with the rootless socket directory mounted. The data role
re-runs `05-app-role.sh` on every deploy, so service-role grants and passwords
that a release changes are applied to an existing database volume too.

This is **not zero-downtime**: each stack's containers are recreated, and
in-flight judge runs are abandoned. The backend's stale sweeper marks those
commits `server_error`. Schedule upgrades outside exams and submission windows.

### Manual image update / restart

```bash
ssh runcodes@<host>
cd /opt/runcodes/compose/<stack>
podman-compose -p runcodes-<stack> -f compose.yml pull
systemctl --user restart runcodes-<stack>
```

### Rollback

- Pin `config_runcodes_image_tag` to a known release and rerun the stages. Keep
  previous tags in GHCR.
- Revert the controller checkout and rerun for config/asset changes.
- Database migrations are **forward-only**. A code rollback incompatible with the
  migrated schema requires restoring the database backup.
- Telemetry volumes are retained across redeploys.

## Database lifecycle and migrations

### Fresh volume

The published database image runs `database/schema/01..05` on an empty volume:
schema, seed data, owner password and the row-only `runcodes_app` role. The data
stage re-runs `05-app-role.sh` (idempotent) and bootstraps the admin.

The fresh volume is **not stamped in Atlas history**. Stamp it before applying the
next migration (see [Database § Existing databases](../services/database.md#existing-databases)).

### Existing volume

The deployment never applies migrations automatically. Back up first, then on
the data host:

```bash
ssh runcodes@203.0.113.11
cd /opt/runcodes/database

export DATABASE_URL="postgres://runcodes:<owner-password>@203.0.113.11:5432/runcodes?sslmode=disable"
./atlas.sh migrate status --url "$DATABASE_URL"
./atlas.sh migrate apply --url "$DATABASE_URL"
```

Use the **node address**, not `localhost`: PostgreSQL is published on the node
address, not loopback. The first `atlas.sh` run downloads the pinned binary into
`/opt/runcodes/database/.tools/`. If the history is not established, follow the
baseline/stamp instructions. Migrations run as the owner, never as
`runcodes_app`.

### Rotating the app password

Change `vault_runcodes_db_app_password`, then run data, compute and edge. The
data stage rotates the role; existing DB sessions remain valid, new ones use the
new secret. Run compute and edge promptly so their pools reconnect.

### Changing the owner password

`POSTGRES_PASSWORD` only initializes a new volume. Changing the vault value does
not change an existing cluster's `runcodes` role. Rotate it deliberately with
`ALTER ROLE` and update the vault in the same window, or restore from backup.

### Rotating the Gmail app password

Change `vault_alertmanager_gmail_app_password` and run
`deploy-observability.yml`. The role rewrites
`/opt/runcodes/secrets/alertmanager_gmail_app_password` and the observability
unit force-recreates Alertmanager. The YAML keeps only the file path.

## Migrating existing data

The new stack starts **fresh** database and object-store volumes. It does not
read existing host directories under `/opt/runcodes/`, so stopping old containers
and running `site.yml` does not migrate production data.

For the database, use the one-shot importer:

1. dump the existing database data-only;
2. start the new data stack on an empty volume;
3. run `database/legacy-migration/migrate.sh` against the new database as the
   owner.

The setup stage places the importer, its SQL and the legacy schema under
`/opt/runcodes/database/` on the data host. Set
`vault_runcodes_legacy_password_salt` so imported users can upgrade their old
hashes on first login.

Review and reconcile SeaweedFS object keys separately; the importer covers the
relational database only. Plan a cutover window: stop the old containers, take
dumps, deploy, import, run `verify.yml`, then point DNS and ports at the new edge.

## Backups and restore

The backup boundary is the `data` host. PostgreSQL and SeaweedFS are the only
irreplaceable data. Redis is a cache/session store; losing it forces re-login but
loses no submissions. Telemetry is optional.

### PostgreSQL

```bash
ssh runcodes@203.0.113.11 \
  'podman exec runcodes-data-database pg_dump -U runcodes -d runcodes -Fc' \
  > "runcodes-$(date -u +%Y%m%dT%H%M%SZ).dump"
```

Add `-e PGPASSWORD=<owner-password>` if local authentication requires it. Store
the dump encrypted and off-host.

Restore with writers stopped:

```bash
ssh runcodes@203.0.113.10 \
  'cd /opt/runcodes/compose/edge && podman-compose -p runcodes-edge -f compose.yml stop backend frontend'
ssh runcodes@203.0.113.12 \
  'cd /opt/runcodes/compose/compute && podman-compose -p runcodes-compute -f compose.yml stop judge'

ansible-playbook playbooks/deploy-data.yml --ask-vault-pass

ssh runcodes@203.0.113.11 \
  'podman exec -i runcodes-data-database pg_restore -U runcodes -d runcodes --clean --if-exists' \
  < runcodes-YYYY....dump
```

After a restore, run `deploy-edge.yml` and `deploy-compute.yml`, verify `/readyz`
and run a submission.

### SeaweedFS

Stop writers, archive the Podman volume, restart:

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

The exact volume name is shown by `podman volume ls`. Redeploy edge and compute
afterwards.

### Redis

Redis uses AOF in `runcodes-data_redis-data`. It is safe to omit. If desired,
stop Redis and archive the volume the same way.

### Telemetry

Prometheus/Loki/Tempo/Pyroscope/Grafana volumes are retained across deploys and
capped by retention. Back up dashboards and provisioning from git. For long-term
telemetry, use object storage as described in the
[production guidance](../observability/production.md).

## Troubleshooting

### Preflight fails before touching a host

| Message | Fix |
| --- | --- |
| `config.yml is missing` | copy `config.yml.example` to `config.yml` |
| `exactly four hosts` | one host per group, four total |
| `distinct private_ip` | every host needs a unique node address |
| `Secrets are missing...` | vault not loaded or placeholders remain |
| `contains a literal '$'` | generate as hex/base64url |
| `does not look like the RunCodes checkout` | run from `deploy/ansible` |
| controller assets missing | run from the repository root or set `runcodes_repo_root` |

### SSH, sudo or package installation fails

The deployment assumes key-based SSH and sudo. Check
`ssh runcodes@<host> sudo true`. If sudo needs a password, configure NOPASSWD or
add `ansible_become_password` (prefer vaulted). A failed `podman-compose` install
usually means the host cannot reach apt, or the distribution does not package it.
Debian 12/13 and Ubuntu 22.04/24.04 do.

### A stack unit fails to start

```bash
systemctl --user status runcodes-<stack>
journalctl --user -u runcodes-<stack> -n 100 --no-pager
cd /opt/runcodes/compose/<stack>
podman-compose -p runcodes-<stack> -f compose.yml config
podman-compose -p runcodes-<stack> -f compose.yml up -d
```

A missing image is usually a GHCR tag/visibility/auth problem. Check
`podman pull <image>` as `runcodes` and the rootless registry login.

### Every container stops shortly after the last SSH session closes

**Symptom.** About ten seconds after logging out, the whole stack stops at once.
`podman ps -a` shows `Exited (0)` or `Exited (143)`; `podman inspect` reports
`OOMKilled=false` and an empty `State.Error`; PostgreSQL logs
`received fast shutdown request` and the backend logs `Shutdown requested`. The
services shut down cleanly: nothing crashed.

**Cause.** Rootless containers live in the systemd user manager
(`user@<uid>.service`). With lingering off, logind stops that manager when the
last session ends and SIGTERMs everything in it.

```bash
loginctl show-user runcodes -p Linger    # Linger=no is the bug
```

**Fix.** Once per host, as root:

```bash
loginctl enable-linger runcodes
loginctl show-user runcodes -p Linger    # → Linger=yes
systemctl --user restart runcodes-<stack>
```

`setup.yml` enables lingering and fails if the host does not report
`Linger=yes`; `verify.yml` checks all four hosts. Without lingering the user
units are also never started at boot, so a reboot leaves the stack down.

### Backend `/readyz` returns 503

PostgreSQL is required:

```bash
ssh runcodes@203.0.113.11 \
  'podman exec runcodes-data-database pg_isready -U runcodes -d runcodes'
ssh runcodes@203.0.113.10 \
  'cd /opt/runcodes/compose/edge && podman-compose -p runcodes-edge -f compose.yml logs --tail=100 backend'
```

Common cause: the app password changed in the vault but the data stage was not
run. Run data, then compute/edge.

### Judge `/readyz` returns 503 with "podman unavailable"

```bash
ssh runcodes@203.0.113.12
systemctl --user status podman.socket
ls -l /run/user/1000/podman/podman.sock    # must be 's', not 'd'
systemctl --user status runcodes-compute
journalctl --user -u runcodes-compute -n 100 --no-pager
```

If the socket is missing:

```bash
loginctl enable-linger runcodes
systemctl --user enable --now podman.socket
systemctl --user restart runcodes-compute
```

`podman exec runcodes-compute-judge ls -l /run/podman/podman.sock` must show a
socket.

### Caddy does not get a certificate / 502 from the edge

- DNS must resolve the domain to the edge public address.
- Ports 80 and 443 must reach the edge.
- `config_runcodes_domain` and `config_runcodes_public_url` must match.
- Check `podman-compose -p runcodes-edge -f compose.yml logs frontend`.
- A 502 on `/api/*` is usually the backend, not Caddy.

### SeaweedFS 403 / S3 signature errors

- Backend and judge use the vault S3 key/secret.
- SeaweedFS reads `/etc/seaweedfs/s3.json` at start; the data unit recreates it.
- The S3 endpoint must be `http://203.0.113.11:8333`, not `seaweed:8333`, from
  another host.

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
3. Can edge/compute reach `203.0.113.13:4318`? Check the provider firewall.
4. Check collector logs and `otelcol_exporter_*` metrics.
5. Check Prometheus targets; `verify.yml` prints down targets.

### Prometheus target down

Use the Prometheus UI; `lastError` names the cause. Typical causes: a stopped
container, a blocked private port, an image/config error, or a password mismatch
after rotation.

### Alerts do not arrive at the receiver

- If `config_runcodes_alertmanager_gmail_enabled` is true, alerts are also
  emailed through the `default` receiver; webhook delivery continues. Check the
  Alertmanager log and
  `/opt/runcodes/secrets/alertmanager_gmail_app_password`.
  `535 5.7.8 Username and Password not accepted` means a wrong app password,
  spaces left in it, or 2-Step Verification disabled.
- Otherwise an empty `vault_alertmanager_webhook_url` sends everything to the
  bundled alert-sink (by design). Check
  `/opt/runcodes/secrets/alertmanager_webhook_url` and Alertmanager logs.
- The `ObservabilitySmokeTest` alert always goes to the sink.
- Test manually:

  ```bash
  curl -X POST http://203.0.113.13:9093/api/v2/alerts \
    -H 'Content-Type: application/json' \
    -d '[{"labels":{"alertname":"ManualTest","severity":"warning"}}]'
  curl http://203.0.113.13:8085/alerts
  ```

  In Gmail mode the manual warning alert is also emailed.

### Grafana login fails after a password change

The secret is a mounted file read at startup. Run `deploy-observability.yml`, or
restart Grafana with `podman-compose ... restart grafana`.

### Disk fills on the observability host

Prometheus/Loki/Tempo/Pyroscope have retention caps, but compactions need free
space. Check the disk alert, lower retention or move to object storage. Do not
delete Podman volumes blindly.

### A config change did not take effect

The stack units force-recreate containers on `systemctl --user restart`, and
Ansible restarts the unit on every stack deploy. If you edited a file directly on
the host, run the matching playbook or restart the unit.

## Related pages

- [Ansible setup](./ansible-setup.md)
- [Security & scaling](./security.md)
- [Observability runbook](../observability/runbook.md)
- [Database](../services/database.md)
