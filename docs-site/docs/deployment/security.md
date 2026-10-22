---
title: Security & scaling
description: The security model of the platform and hosts, known limits of the reference deployment, and how to grow beyond four machines.
---

# Security & scaling

This page covers the security model an operator is responsible for, the limits
of the reference deployment, and the scaling path. Service-level isolation
details are in [Judge](../services/judge.md#isolation).

## Public surface

Only the edge is public:

| Port | Service | Exposure |
| --- | --- | --- |
| 22/tcp | SSH | operator/VPN ranges only |
| 80/tcp | Caddy | ACME HTTP-01 and redirect |
| 443/tcp | Caddy | SPA, `/api/*`, `/otel/v1/traces` |

Everything else binds to a private node address (four-machine) or loopback
(Compose): PostgreSQL, Redis, SeaweedFS, the judge, Prometheus, Grafana, Loki,
Tempo, Pyroscope, Alertmanager and the collector.

The provider firewall is the primary boundary. UFW is optional and does not
filter container-published ports; `ss -lntp` and the bind addresses are the
truth. In a Compose install, only the frontend publishes on a network interface.

## Application security model

### Authentication and sessions

- JWT (HS256) in an `HttpOnly`, `SameSite=Strict`, `Secure` cookie; 30-minute
  sliding lifetime.
- Every token carries the account's `token_version`; every protected request
  compares it (and the role) with the database. Password changes, completed email
  changes and admin role/confirmation changes bump it, invalidating every issued
  token. Logout only clears the browser cookie, so a copied token remains valid
  until it expires; rotating the JWT secret invalidates all tokens.
- Email confirmation is required for login and checked again on every protected
  route. A session issued before an account was unconfirmed stops working. Email
  changes keep the old address active until the new one is verified.
- Verification tokens are stored only as SHA-256, expire after 24 hours, and a
  resend invalidates the previous one.
- Password recovery tokens are stored only as SHA-256, expire after 1 hour, and
  are single use; a new request invalidates the previous one. The request route
  answers the same way for every address, so it does not disclose accounts.
- The API-wide limit is 600 requests/minute per client IP (sized for a classroom
  behind one NAT); login/signup get 20 and the email routes 10 per minute.

### Authorization

Every route has a server-side check; hiding UI links is not a control. The class
relation (owner, co-professor, monitor, student, banned) is loaded from the
database, not trusted from the client. Submission reads are scoped to the owner
(or staff for the live stream), and a foreign commit is indistinguishable from a
missing one (`404`).

### Database roles

The services log in as `runcodes_app`, which owns nothing and cannot change the
schema, create roles or read `pg_authid`. Only the database container and
migration tooling use the owner. A SQL injection in the backend or judge cannot
drop the schema or read other databases' credentials. CI asserts the role's
limits.

### Untrusted code

- Containers get cgroup caps (memory, PIDs, CPU) independent of the in-container
  monitor.
- No network namespace, so submissions cannot reach the internet, other
  containers, the backend, the database or SeaweedFS.
- `no-new-privileges` and `cap_drop=ALL`, so setuid, file capabilities or the
  runtime's default capability set cannot elevate.
- Core dumps are disabled (`RLIMIT_CORE=0`), so a crash cannot fill the workspace.
- The image filesystem is read-only; only the workspace and size-capped
  `/tmp`/`/run` tmpfs are writable.
- One persistent mount (the workspace), a non-root user, a zero umask, and a
  harness that deletes `container.config` before running submitted code.
- The judge stops the container before grading and reads only regular files from
  the workspace, with expected outputs in a private directory.
- Leftover containers are pruned at startup; abandoned workspaces and output
  archives older than their retention are swept hourly.

### Secrets

- Compose requires secrets explicitly and fails fast when one is missing. Redis
  requires a password, and the SeaweedFS S3 gateway is configured with the
  backend/judge credentials instead of accepting anonymous requests.
- Ansible verifies SSH host keys; provision `known_hosts` before the first run.
- Ansible Vault encrypts deployment secrets; they are rendered into files owned
  by the deployment user on the hosts. `.env` files are 0600; mounted secret
  files are left readable by the service container that consumes them. They are
  never committed or baked into images.
- The Grafana admin password is a mounted file; the Alertmanager webhook URL and
  Gmail app password are mounted files (`*_file`), not YAML.
- The local observability stack generates the Grafana password into
  `.observability/` (gitignored).

### Public browser telemetry

The four-machine edge proxies **only** `POST /otel/v1/traces` to the collector,
with a 4 MiB body cap. It is unauthenticated by design. For a hostile
deployment, rate-limit/WAF that path, use a browser-specific collector with a
key, or disable browser telemetry. Metrics and logs are not exposed.

## Host hardening checklist

1. **Provider firewall** — only 22 (operators), 80/443 (edge), and peer traffic
   to private ports.
2. **SSH** — keys only, password auth disabled, restrict 22, use a bastion if the
   hosts are public.
3. **Rootless Podman** — the service stacks and graded runs execute as
   `runcodes`; container root maps to that unprivileged user. Treat the
   deployment user's storage and socket as sensitive.
4. **Images** — GHCR packages public or pulled with a read-only token. Do not put
   a write token on the hosts. Pin tags for reproducible deploys.
5. **TLS** — only Caddy's 443 is public and renews certificates. The private
   network is plaintext; add WireGuard/IPsec or mTLS if you do not trust it.
6. **Secrets** — vault-encrypted, restricted files, rotated on a schedule and
   after suspected exposure. No literal `$` in values.
7. **Grafana** — private, anonymous access and sign-up disabled, no plugin
   downloads; use SSO if exposed through a proxy. Never expose Prometheus,
   Alertmanager, Loki or Tempo directly.
8. **OTLP public path** — rate-limit or disable browser telemetry if abused.
9. **Backups** — encrypted, off-host, access-restricted, restores tested.
10. **Logs** — Podman journal and Loki may contain user data, request paths and
    SQL statements (without parameters). Do not enable body logging in production.

## Image and dependency scanning

- `.github/workflows/images.yml` builds and scans the four application images
  with Trivy on pushes, pull requests and a weekly schedule, uploading SARIF to
  the Security tab.
- `.github/workflows/trivy-repo.yml` scans the repository with `trivy.yaml`;
  accepted risks live in `.trivyignore.yaml`.
- `docker compose` builds are scanned the same way.
- Dependabot updates dependencies and GitHub Actions.

A finding in a base image is not automatically exploitable in the platform's
threat model, but it should be triaged. The judge is the highest-risk surface
because it handles untrusted input; the runner images are the second.

## Known limits

The reference four-machine deployment is production-shaped, not highly
available:

| Limit | Consequence |
| --- | --- |
| one replica of each service | a host failure is an outage |
| one observability host | telemetry is unavailable if it dies; apps keep serving |
| no automatic schema migrations | migrations are an explicit, backed-up operation |
| stack deploys force-recreate containers | every deployment is a maintenance window |
| single-node telemetry storage | retention is bounded by local disk, no object store |
| only application logs in Loki | infrastructure logs stay in the Podman journal |
| plaintext internal traffic | OTLP, PostgreSQL, Redis and S3 are unencrypted on the node network |
| public browser OTLP | unauthenticated trace ingestion, bounded by Tempo retention |
| runner tag defaults to `latest` | a moving target unless pinned |
| smtp4dev is not production mail | configure a real relay |
| no multi-edge load balancer | a single public entry point |
| backend compromise reads all submissions | isolation is strongest around graded code, not the API tier |

The Compose deployment has the same single-host limits plus hard-coded loopback
ports and no process supervision beyond the Compose restart policy.

## Alternatives considered

| Alternative | Why not |
| --- | --- |
| All services on one host | Simplest, but the judge, stateful data and telemetry compete for CPU and disk, and one compromise or disk-full event takes everything down. It also loses the public/private and trusted/untrusted separation. |
| Observability on the edge | Puts telemetry storage and its disk/CPU bursts on the request path, and exposes the dashboards' host to the internet-facing tier. |
| Judge on the data host | Couples untrusted code execution to the database and object store — the two things a runaway submission must never affect. |
| Public OTLP collector | An unauthenticated public OTLP endpoint can be abused to fill Tempo, Loki and Prometheus. The edge exposes only `/otel/v1/traces` and proxies it to the private collector. |
| A collector on every host | More moving parts and no single place to reason about sampling, retries and queues. Alloy runs per host only because pprof cannot leave its compose network. |

## Scaling beyond four hosts

The topology is deliberately fixed at four machines. When it outgrows them,
split in this order:

1. **Compute.** Add judge hosts and point the backend at a load balancer. The
   judge claims from PostgreSQL safely across instances. Keep each host's
   `/opt/runcodes/exec` local and its own rootless Podman socket.
2. **Observability storage.** Move Prometheus to remote write
   (Mimir/Thanos), Loki/Tempo/Pyroscope to object storage, and make the collector
   replicas. Grafana can move to an external database and SSO.
3. **Data.** Split SeaweedFS from PostgreSQL, then move to managed PostgreSQL and
   object storage with backups and point-in-time recovery.
4. **Edge.** Run multiple stateless edge hosts behind a load balancer. The
   backend is stateless; an SSE connection is sticky only for its duration, and a
   reconnect replays persisted state.

### Capacity starting points

| Host | vCPU | RAM | Disk | Notes |
| --- | --- | --- | --- | --- |
| edge | 2 | 4 GB | 40 GB | Caddy + Go backend + smtp4dev |
| data | 4 | 8 GB | 100–200 GB SSD | PostgreSQL + SeaweedFS grow with submissions |
| compute | 4 | 8–16 GB | 80–150 GB SSD | `JUDGE_CONCURRENCY × 1.5 GiB` is the execution floor; 22 runner images take disk |
| observability | 4 | 8–16 GB | 200–400 GB SSD | Prometheus 15d/8 GB, Loki/Tempo/Pyroscope 7d each, plus compaction headroom |

The judge's memory is not a container `mem_limit`: each runner is capped by
`JUDGE_CONTAINER_MEMORY_BYTES` (1.5 GiB default) and up to
`JUDGE_CONCURRENCY` (4 default) run at once. Size the compute host for at least
6 GiB of runner memory plus the host, Podman and judge.

Telemetry retention is capped, not quota-enforced at the filesystem level:

- Prometheus: 15 days and 8 GB, whichever comes first;
- Loki: 7 days;
- Tempo: 7 days;
- Pyroscope: 7 days.

Leave free disk for compactions. The `RunCodesHostDiskSpaceLow` alert fires below
15% free; it is a last resort, not a capacity plan.

## Incident response

1. **Contain** — remove the affected host from the public path or stop the
   stack unit; revoke GHCR/SSH credentials if a host is compromised.
2. **Preserve** — capture logs (`journalctl --user`, `podman logs`), a database
   dump if integrity is in question, and the image digests in use.
3. **Rotate** — JWT secret (invalidates sessions), judge token, database
   passwords, S3 keys, Redis password, SMTP credentials, and the Alertmanager
   webhook URL and Gmail app password.
4. **Restore** — restore PostgreSQL and SeaweedFS from the last known-good
   backup, then run `verify.yml`.
5. **Review** — compare the host against the deployment repo; Podman does not
   enforce drift detection, so a manual audit is required.

## Related pages

- [Ansible setup](./ansible-setup.md)
- [Ansible operations](./ansible-operations.md)
- [Judge isolation](../services/judge.md#isolation)
- [Four-machine observability](../observability/production.md)
- [CI/CD & releases](../ci-cd.md)
