---
title: Platform operations
description: Health checks, backups, upgrades, retention, capacity and incident response for a running RunCodes deployment.
---

# Platform operations

This is the day-2 runbook for the application itself: checking that the stack is
healthy, protecting the data, upgrading safely and responding when submissions
stop. It assumes the deployment is already running; installation is covered in
[Getting started](../guide/getting-started.md) and
[Deployment](../deployment/index.md).

Infrastructure-specific commands live in
[Docker Compose](../deployment/compose.md#operating) and
[Ansible operations](../deployment/ansible-operations.md). Metrics, logs, traces
and alert configuration are in [Observability](../observability/index.md) and the
[runbook](../observability/runbook.md).

## Know the failure domains

| Component | Required for | If it fails |
| --- | --- | --- |
| Frontend and Caddy | all browser traffic | outage |
| Backend API | all authenticated work | outage |
| PostgreSQL | every durable operation | outage; the queue and history are unavailable |
| SeaweedFS | source files, case files, materials | submissions cannot be created or graded |
| Judge | new runs | submissions queue and new ones return `503` when the backlog is full |
| Redis | cache | degraded performance, not data loss or lost sessions |
| SMTP relay | verification, recovery, reminders, class email | signup still succeeds, mail does not arrive |
| Rootless Podman socket | graded execution | judge readiness fails |
| Observability stack | diagnosis and alerting | no telemetry; the platform still serves requests |

The stack runs one replica of each service. A host failure is an outage. See
[Security & scaling](../deployment/security.md#known-limits) for the supported
topology and its limits.

## Health checks

Run these after every restart, upgrade or certificate change.

### Backend

```bash
curl -s http://localhost:8443/healthz
curl -s http://localhost:8443/readyz | jq
```

`healthz` answers as soon as the process is alive. `readyz` requires PostgreSQL
and reports the judge separately:

```json
{ "database": "ok", "judge": "ok", "status": "ok" }
```

A judge of `unavailable` leaves `status` at `ok`: authentication, classes and
browsing still work, but new submissions fail. A database of `unavailable`
returns `503` because every route needs it.

### Judge

```bash
curl -s http://localhost:9000/readyz | jq
```

The judge is ready only when PostgreSQL and the rootless Podman socket are both
reachable. `podman unavailable` is the most common first-run failure; see
[Getting started](../guide/getting-started.md#first-run-troubleshooting).

### Frontend

```bash
curl -sI https://<your-domain>/
curl -s https://<your-domain>/api/v1/settings/public | jq
```

The first command checks Caddy and the SPA. The second checks the `/api` proxy,
the backend and the public settings route in one request.

### Submission path

A health check does not prove that code can run. Submit a known-correct solution
to a test exercise after a deployment change. Check that the status reaches
**Concluído** and that the case verdicts appear.

## What to watch

The observability stack exposes the signals that matter. If it is not enabled,
use the endpoints and logs below.

| Signal | Why it matters | Where |
| --- | --- | --- |
| Submission error rate | a judge or storage failure | backend metrics and logs |
| Judge queue depth and oldest age | backlog growing faster than workers | judge metrics |
| `5xx` rate | API or database failure | reverse proxy and backend logs |
| Database connections | pool exhaustion | PostgreSQL exporter |
| Disk usage | images, workspaces, objects, WAL, telemetry | node exporter |
| TLS certificate expiry | outage at renewal | Caddy logs and blackbox exporter |
| Mail failures | users cannot verify or recover accounts | backend logs |
| Repeated `401`/`403` | expired sessions or an authorization regression | request logs |

The judge exports `runcodes.judge.queue.depth` and
`runcodes.judge.queue.oldest_age`. A rising oldest age with a flat depth means
runs are slow or stuck, not that demand increased.

Every API response carries a `TraceId` header. Use it to join a browser request
to the backend, judge and database spans in Tempo, or to grep Loki.

## Routine checks

### Daily

- Open the public URL and sign in.
- Check backend and judge readiness.
- Check that submissions are completing, not only queueing.
- Check free disk on the data and compute hosts.
- Check that last night's database and object-store backup finished.
- Review alerts and the error rate for unusual spikes.

### Weekly

- Confirm the backup can be read (list the dump or restore it to a scratch
  database).
- Check certificate expiry and renewal logs.
- Review judge queue age and worker concurrency against class deadlines.
- Review unconfirmed and dormant admin accounts.
- Check for new backend, judge, runner or dependency releases.

### Each term

- Ask professors to export their classes before deleting or replacing them.
- Review the file type catalog against the runner images actually deployed.
- Run a restore drill from an off-host backup.
- Review administrator and professor roles for people who left.
- Recompile classes whose exercises depend on a changed runner image.
- Update the deployment to a pinned release outside exam windows.

## Backups

PostgreSQL and SeaweedFS are the only irreplaceable data. Redis is disposable,
the judge workspace is disposable, and Caddy certificates are regenerable.

### What to back up

| Data | Backup | Notes |
| --- | --- | --- |
| PostgreSQL | `pg_dump -Fc` or a consistent volume snapshot | users, classes, exercises, cases, commits, grades |
| SeaweedFS | volume archive or bucket replication | source files, case files, materials |
| Secrets | your secret manager or encrypted vault | JWT secret, database, judge, S3, Redis, SMTP |
| Observability configuration | Git | dashboards, rules, collector config |
| Caddy state | optional | certificates can be reissued |

A database dump without the object store restores the course structure but not
the submitted files. Back up both, or accept that old sources are unavailable.

### Compose

```bash
docker compose exec -T database \
  pg_dump -U runcodes -d runcodes -Fc > "runcodes-$(date -u +%Y%m%dT%H%M%SZ).dump"
```

Stop the writers before archiving the object store, as described in
[Docker Compose § Data and volumes](../deployment/compose.md#data-and-volumes).
Copy both artifacts off the host and encrypt them at rest.

### Ansible

The commands for the four-machine deployment are in
[Ansible operations § Backups and restore](../deployment/ansible-operations.md#backups-and-restore).
The boundary is the `data` host.

### Restore

1. Stop the backend and judge so nothing writes during the restore.
2. Restore PostgreSQL from the dump.
3. Restore the SeaweedFS volume or bucket.
4. Start the data services, then the backend and judge.
5. Check `/readyz` and run a real submission.
6. Check that an old submission's source downloads.

A restore is not complete until a submission and a source download work.

::: danger Restoring an older database
The database dump and the object store must come from the same point in time. A
newer object store with an older database leaves orphaned objects. An older
object store with a newer database leaves commits whose source is missing. Keep
the backup pair together.
:::

## Upgrades

Use a pinned release tag. `latest` moves without warning and makes rollback and
incident review harder.

### Before an upgrade

1. Read the release notes and the migration notes.
2. Take a database and object-store backup.
3. Confirm the current stack is healthy.
4. Choose a window outside exams and submission deadlines.
5. Check that the runner images for the languages you use are available.

### Compose

```bash
git pull
docker compose up -d --build
docker compose ps
curl -s http://localhost:8443/readyz | jq
```

### Ansible

Follow [Ansible operations § Application upgrade](../deployment/ansible-operations.md#application-upgrade).
The playbook pulls the configured image tags and recreates each stack.

### During and after

- In-flight judge runs are interrupted when the judge restarts. The backend's
  stale sweeper marks them `server_error` after the configured timeout.
- Apply database migrations as the owner role, not as `runcodes_app`. See
  [Database § Migrations](../services/database.md#migrations).
- A migration is forward-only. A code rollback that is incompatible with the
  migrated schema requires restoring the database backup.
- After the rollout, test a submission, a source download and a verification
  email.
- If a runner image changed, recompile the affected classes from the professor
  tools. Running submissions use the image tag at execution time.

## Database lifecycle

- The `runcodes_app` role owns no tables and cannot change the schema. It is the
  backend's login.
- The `runcodes_judge` role can only claim and update commits and read the
  exercise data it needs.
- The owner role (`runcodes`) runs migrations and bootstrap scripts.
- A fresh Compose volume contains the current schema but no Atlas history. Stamp
  it before applying the next migration.
- Never edit a live schema by hand. Generate a migration, review it, apply it and
  commit the schema, migration and `atlas.sum` together.

The full workflow is in [Database](../services/database.md).

## Retention and data growth

| Data | Default retention | Where to change |
| --- | --- | --- |
| Output archives | 168 hours (`JUDGE_ARTIFACT_TTL`) | judge environment; `0` keeps them |
| Judge event log | 10 minutes in memory | `JUDGE_EVENT_RETENTION` |
| Judge workspaces | swept after the run budget plus grace | `JUDGE_KEEP_WORKSPACES`, exec directory |
| Submission sources | kept with the commit | database/object-store lifecycle |
| Prometheus metrics | 15 days / 8 GB | observability environment |
| Tempo traces | 168 hours | observability configuration |
| Logs and profiles | per Loki/Pyroscope configuration | observability configuration |
| Database rows | kept until deleted | no automatic pruning |

Class and account deletion cascade to submissions and storage objects. There is
no automatic purge of old classes or users. Decide with the institution how long
to keep a term's data, and export or archive a class before deleting it.

Output archives are built from the run workspace and kept on the judge host for
`JUDGE_ARTIFACT_TTL`. Once the judge removes them, they are gone. Do not set a
short TTL if students need to download their output files weeks later.

## Capacity

- Disk: the runner images take roughly 15 GB, plus the database, object store,
  workspaces and telemetry. Check before adding languages.
- Compute: each running container uses CPU and memory. `JUDGE_CONCURRENCY`
  defaults to 4; raise it only after measuring the host.
- Queue: the backend refuses new submissions when the queue reaches
  `RUNCODES_JUDGE_MAX_QUEUE_DEPTH` (100 by default). A full queue is a capacity
  signal, not a reason to raise the limit blindly.
- Per student: at most two unfinished submissions.
- Database: commits and results grow with every run, including recompilations.
  A recompile of a large class creates one new commit per student and exercise.
- Object store: every submission stores a source object; case files and materials
  add objects per exercise.
- Judge disk: output archives and workspaces live on the judge host, not in
  SeaweedFS. Size that volume for the artifact retention.

See [Security & scaling](../deployment/security.md#capacity-starting-points) for
host sizing and the four-machine topology.

## Security operations

### Rotate secrets

| Secret | Effect of rotation | Update |
| --- | --- | --- |
| `RUNCODES_JWT_SECRET` | every session is invalidated | backend, restart |
| `RUNCODES_JUDGE_TOKEN` | backend and judge stop trusting each other until both change | backend and judge |
| database role passwords | services cannot connect until updated | PostgreSQL role, service config |
| S3 credentials | backend and judge cannot read objects until updated | SeaweedFS config, backend, judge |
| Redis password | backend cache reconnects or degrades | Redis and backend |
| SMTP credentials | mail delivery fails until updated | backend |

Rotate after a suspected compromise, when a person with access leaves, or on the
schedule your institution requires. Do not rotate one side of a shared secret and
leave the other.

### Account hygiene

- Keep at least two administrators.
- Remove the `admin` role from people who no longer need it.
- Demote inactive professors after transferring their classes.
- Delete or unconfirm accounts that should no longer sign in.
- Review the class owners list before deleting a professor: deletion cascades to
  their classes.
- Do not disable the only administrator.

### Network and platform

- Only Caddy is public. Keep the backend, judge, database, object store,
  exporters and Grafana on private or loopback addresses.
- Keep the firewall rules from the deployment: SSH from known addresses, 80/443
  on the edge, private peers only for the rest.
- Keep TLS renewal working. A Caddy hostname renews automatically; check the
  logs after DNS or firewall changes.
- Scan images and the repository with Trivy as the CI workflows do.
- Apply dependency and base-image updates on a schedule, not only during
  incidents.

## Incident response

### Submissions return `503`

1. Check the judge: `curl -s http://localhost:9000/readyz | jq`.
2. If `podman unavailable`, check the rootless socket and recreate the judge
   container if the socket was restarted.
3. If the judge is ready, check the queue depth and oldest age. A full queue
   means workers are saturated or images are pulling slowly.
4. Check disk on the compute host and the object store.
5. Check backend logs for `RUNCODES_JUDGE_MAX_QUEUE_DEPTH` rejections and wake
   failures.

### Backend `/readyz` returns `503`

1. Check PostgreSQL is running and accepting connections.
2. Check the `runcodes_app` password and host in the backend environment.
3. Check disk and connection limits on the database host.
4. Check the backend log for migration or role errors.

### The page loads but API calls fail

1. Check Caddy logs for `502` or `504` on `/api/*`.
2. Check the backend is running and its readiness.
3. Check `FRONTEND_ORIGIN` if the SPA runs on a separate origin.
4. Check for a certificate or mixed-content error in the browser.

### Mail is not delivered

1. Check backend logs for SMTP errors.
2. Check the relay host, port, sender and credentials.
3. Check rate limits: repeated resends from one address can be throttled.
4. Check spam filtering at the recipient.
5. In Compose, confirm that smtp4dev is the intended target and not a real relay.

### The judge is slow or stuck

1. Check queue depth and oldest age.
2. Check CPU, memory and disk on the compute host.
3. Check whether images are pulling; pre-pull runner images before a term.
4. Check for large workspaces or repeated timeouts in the judge log.
5. Lower concurrency if the host is thrashing. Raise it only with measured
   headroom.

### Disk is full

1. Identify the volume: images, database WAL, object store, workspaces,
   Prometheus, Loki or Tempo.
2. Remove old backups only after a newer restore point exists off-host.
3. Remove unused container images carefully; keep the runner images in use.
4. Lower telemetry retention if it is the consumer.
5. Expand the volume before retrying writes. A full disk makes writes fail and
   can leave the database unable to start.

### TLS certificate expired or renewal fails

1. Check Caddy logs for ACME errors.
2. Check DNS points at the edge and port 80 is reachable.
3. Check the Caddy data volume is writable.
4. Restart Caddy only after the cause is clear; repeated failed renewals can hit
   rate limits.

### A class or account was deleted by mistake

There is no undo in the API. Restore the database and object store from the last
backup pair, or accept the loss. This is why a class export and a backup are
required before destructive changes.

### An administrator account is compromised

1. Unconfirm the account or change its role with another admin.
2. Reset its email to an address you control.
3. Rotate `RUNCODES_JWT_SECRET` to invalidate sessions.
4. Review recent admin changes in the backend logs.
5. Rotate the judge token, database passwords and S3 credentials if you cannot
   rule out access to the deployment secrets.
6. Restore from backup if data was altered.

### A submission is suspected of abuse

Graded code runs without a network namespace, without new privileges, with a
read-only root filesystem and with cgroup limits. It cannot reach the edge, the
database or the telemetry store. Preserve the judge log and, if
`JUDGE_KEEP_WORKSPACES=true`, the workspace for analysis. Delete the account
through the admin routes if the abuse warrants it; there is no endpoint to delete
a single submission.

## Maintenance calendar

| When | Task |
| --- | --- |
| Every deployment | health checks, real submission, source download, mail test |
| Daily | readiness, queue, disk, backup completion, alerts |
| Weekly | restore sample, certificate expiry, queue trend, account review |
| Monthly | dependency and image updates, access review, capacity review |
| Each term | class exports, file type review, role review, restore drill |
| After an incident | timeline, root cause, corrective action, backup check |

## Related pages

- [Administrator guide](./index.md)
- [Deployment overview](../deployment/index.md)
- [Production checklist](../deployment/checklist.md)
- [Docker Compose](../deployment/compose.md)
- [Ansible operations](../deployment/ansible-operations.md)
- [Observability overview](../observability/index.md)
- [Observability runbook](../observability/runbook.md)
- [Security & scaling](../deployment/security.md)
- [Database](../services/database.md)
