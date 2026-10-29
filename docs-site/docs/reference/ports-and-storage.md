---
title: Ports & storage
description: Network matrix for the Compose and four-machine deployments, object-store layout and persistent volumes.
---

# Ports & storage

## Compose (single host)

Podman is the default engine; Docker Compose v2 works the same way.

### Published on the host

| Host port | Variable | Service | Binding |
| --- | --- | --- | --- |
| `8080` | `FRONTEND_HTTP_PORT` | frontend / Caddy HTTP | `0.0.0.0` |
| `8444` | `FRONTEND_HTTPS_PORT` | frontend / Caddy HTTPS | `0.0.0.0` |
| `8443` | `BACKEND_PORT` | backend API | `127.0.0.1` |
| `9000` | `JUDGE_PORT` | judge API | `127.0.0.1` |
| `5432` | `DATABASE_PORT` | PostgreSQL | `127.0.0.1` |
| `8333` | `SEAWEED_S3_PORT` | SeaweedFS S3 | `127.0.0.1` |
| `8081` | `SMTP4DEV_WEB_PORT` | smtp4dev web UI | `127.0.0.1` |
| `1025` | `SMTP4DEV_SMTP_PORT` | smtp4dev SMTP | `127.0.0.1` |

The table lists the Compose file's inline fallbacks. The deployment template
(`deploy/compose/.env.example`) ships the public profile instead: the frontend on
`80`/`443` and `RUNCODES_DOMAIN` set to a hostname. Ports below 1024 need
`net.ipv4.ip_unprivileged_port_start=80` on a rootless Podman host; see
[Image-based Compose § Public hostname and TLS](../deployment/image-compose.md#public-hostname-and-tls).

Every port can be changed with its variable (set it in `.env`) to run this stack
next to another one on the same machine. Only the frontend is published on every
interface.

Redis is not published; the backend reaches it as `redis:6379` on the Compose
network.

### Internal service ports

| Service | Port | Consumers |
| --- | --- | --- |
| frontend (Caddy) | 80, 443 | browser |
| backend | 8443 | Caddy, local API clients |
| judge | 9000 | backend |
| database | 5432 | backend, judge |
| redis | 6379 | backend |
| seaweed | 8333 | backend, judge |
| smtp4dev | 25 (SMTP), 80 (UI) | backend, browser |

With the observability overlay, see
[Local observability stack](../observability/local.md#urls) for the additional
loopback ports.

## Four-machine deployment

### Public

| Host | Port | Service |
| --- | --- | --- |
| edge | 22/tcp | SSH (restrict to operators/VPN) |
| edge | 80/tcp | Caddy: ACME HTTP challenge and redirect |
| edge | 443/tcp | Caddy: SPA, `/api/*`, `/otel/v1/traces` |

### Private (bound to `private_ip`)

| Host | Port | Service | Consumer |
| --- | --- | --- | --- |
| edge | 8443 | backend health/ready | blackbox, operator |
| edge | 9100 | node-exporter | Prometheus |
| edge | 12345 | Alloy UI/metrics | Prometheus |
| data | 5432 | PostgreSQL | backend, judge, postgres-exporter |
| data | 6379 | Redis (password required) | backend |
| data | 8333 | SeaweedFS S3 | backend, judge |
| data | 9100 | node-exporter | Prometheus |
| data | 9187 | postgres-exporter | Prometheus |
| data | 9121 | redis-exporter | Prometheus |
| compute | 9000 | judge API | backend, blackbox |
| compute | 9100 | node-exporter | Prometheus |
| compute | 12345 | Alloy UI/metrics | Prometheus |
| observability | 4317 | OTLP/gRPC | future/internal exporters |
| observability | 4318 | OTLP/HTTP | backend, judge, Caddy browser proxy, smoke test |
| observability | 13133 | collector health | verify/blackbox |
| observability | 8888 | collector self-metrics | Prometheus |
| observability | 8889 | application metrics | Prometheus |
| observability | 9090 | Prometheus | Grafana, verify, operator |
| observability | 9093 | Alertmanager | Prometheus, Grafana, operator |
| observability | 8085 | alert-sink | smoke test |
| observability | 3100 | Loki | collector, Grafana, verify |
| observability | 3200 | Tempo | collector, Grafana, verify |
| observability | 4040 | Pyroscope | Alloy, Grafana, verify |
| observability | 3000 | Grafana | operator (SSH tunnel) |
| observability | 9100 | node-exporter | Prometheus |
| observability | 9115 | blackbox-exporter | Prometheus |

### Host-local only

| Host | Port / path | Service |
| --- | --- | --- |
| edge | 8081, 1025 | smtp4dev (loopback) |
| compute | `/run/user/<uid>/podman` | rootless Podman runtime directory |
| all | `backend:6060`, `judge:6060` | pprof, inside the compose network only |

## Object storage layout

The S3 bucket prefix is `RUNCODES_S3_BUCKET_PREFIX` (default `runcodes`).

| Bucket | Object key | Contents |
| --- | --- | --- |
| `<prefix>-commits` | `commit.s3_key` | submitted source or zip |
| `<prefix>-cases` | `<case_id>/in` | test-case stdin |
| `<prefix>-cases` | `<case_id>/out` | expected stdout |
| `<prefix>-cases` | `<case_id>/files/<name>` | extra files for the case |
| `<prefix>-files` | `compilationfiles/<exercise_id>/<path>` | exercise compilation files |
| `<prefix>-files` | `attachments/...` | exercise attachments |
| `<prefix>-outputfiles` | generated archive | output archive metadata (the archive itself is produced on the judge) |

The judge reads source and cases, and writes nothing to S3. The backend uploads
submissions and exercise files; the judge's output archive is proxied through
the backend.

## Persistent volumes

### Compose

| Volume | Contents | Reconstructible? |
| --- | --- | --- |
| `runcodes-db-vol` | PostgreSQL data | **no** |
| `runcodes-seaweed-vol` | S3 objects | **no** |
| `runcodes-redis-vol` | cache/session counters | yes |
| `caddy_data`, `caddy_config` | certificates and state | yes |
| `smtp4dev-data` | caught mail | no (not important) |
| `./exec` | judge workspaces | yes |

### Four-machine deployment

| Data | Location | Backup |
| --- | --- | --- |
| PostgreSQL | `data` volume `runcodes-data_db-data` | `pg_dump` |
| Redis AOF | `data` volume `runcodes-data_redis-data` | optional |
| SeaweedFS | `data` volume `runcodes-data_seaweed-data` | volume archive |
| Telemetry | `observability` volumes | optional; retention-capped |
| Caddy certs | `edge` volumes | regenerable |
| Judge workspaces | `/opt/runcodes/exec` on compute | disposable |
| Deploy assets | `/opt/runcodes/` on each host | from git |

The database and object store are the only data that cannot be reconstructed.
They share the `data` host to make a consistent backup window and a single
restore target.

## Retention

| Store | Local overlay | Four-machine |
| --- | --- | --- |
| Prometheus | 15d / 8 GB | 15d / 8 GB (`config_runcodes_prometheus_retention_*`) |
| Loki | 7d | 7d |
| Tempo | 7d | 7d (`config_runcodes_tempo_retention_hours`) |
| Pyroscope | 7d | 7d |
| Grafana | SQLite volume | SQLite volume |
| Alertmanager | state volume | state volume |

Prometheus, Loki, Tempo and Pyroscope need free disk beyond the retention caps
for WAL and compactions. The `RunCodesHostDiskSpaceLow` alert fires below 15%
free.

## Timeouts and size limits

| Limit | Value |
| --- | --- |
| Submission upload | 10 MiB |
| Authoring upload (test case/files) | 32 MiB |
| Test-case batch import | 64 MiB compressed / 256 MiB expanded |
| Class archive import | 256 MiB compressed / 512 MiB expanded |
| OTLP browser trace body (Caddy) | 4 MiB |
| Per-case comparison files | 16 MiB |
| Output archive | 64 MiB |
| Zip entry / total expanded | 64 MiB / 256 MiB |
| Uploaded file per output | 1 MiB |
| Session lifetime | 30 minutes, sliding |
| Verification token lifetime | 24 hours |
| Password reset token lifetime | 1 hour |
| Judge event retention | 10 minutes after finish |
| API rate limit | 600/min per client IP |
| Signup/login rate limit | 20/min per client IP |
| Email route rate limit | 10/min per client IP |
| Enrollment code attempts | 5/min per account |

## Related pages

- [Configuration reference](./configuration.md)
- [Architecture](../guide/architecture.md)
- [Ansible operations § Backups](../deployment/ansible-operations.md#backups-and-restore)
