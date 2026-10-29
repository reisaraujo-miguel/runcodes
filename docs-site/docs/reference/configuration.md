---
title: Configuration reference
description: Every environment variable and deployment setting for the backend, judge, frontend, Compose, observability stack and Ansible deployment.
---

# Configuration reference

Each service loads configuration once at startup and fails fast on a missing
required value or an unparseable one. This page lists the variables by service.

::: tip Where values live
- **Compose** (Podman by default, Docker supported) reads a root `.env` or the
  process environment.
- **Four-machine deployment** reads `deploy/ansible/inventories/group_vars/all/config.yml`
  (untracked) and secrets from `vault.yml` (encrypted).
- Each service also has an `.env.example` with the development defaults.
:::

## Backend

Source: `backend/config/config.go`, `backend/.env.example`.

### HTTP and development

| Variable | Default | Required | Meaning |
| --- | --- | :-: | --- |
| `RUNCODES_API_PORT` | — | ✓ | HTTP port; must be a valid TCP port |
| `DEBUG_MODE` | `false` | | human-readable logs and non-`Secure` cookie |
| `FRONTEND_ORIGIN` | `http://localhost:5173` | | single allowed CORS origin (not `*`) |

`-debug` overrides `DEBUG_MODE`. In debug mode, request/response bodies are
logged only when the request carries `Debug: reveal-body-logs`.

### Authentication and accounts

| Variable | Default | Required | Meaning |
| --- | --- | :-: | --- |
| `RUNCODES_JWT_SECRET` | — | ✓ | HS256 session signing key |
| `RUNCODES_PUBLIC_URL` | — | ✓ | absolute browser-facing origin used in verification and recovery links |
| `RUNCODES_LEGACY_PASSWORD_SALT` | empty | | old platform salt for migrated password hashes |

`RUNCODES_PUBLIC_URL` must be `http(s)://host[:port]` with no path, query or
fragment. Sessions last 30 minutes and slide with `POST /api/v1/auth/refresh`.

### Database

| Variable | Default | Required | Meaning |
| --- | --- | :-: | --- |
| `RUNCODES_DB_HOST` | — | ✓ | PostgreSQL host |
| `RUNCODES_DB_PORT` | — | ✓ | PostgreSQL port |
| `RUNCODES_DB_NAME` | — | ✓ | database name |
| `RUNCODES_DB_USER` | — | ✓ | role; use `runcodes_app` for the backend and `runcodes_judge` for the judge |
| `RUNCODES_DB_PASSWORD` | — | ✓ | role password |
| `RUNCODES_DB_SSLMODE` | `disable` | | lib/pq SSL mode |

### Redis

Redis is optional. An unreachable Redis logs a warning and the cache becomes a
no-op.

| Variable | Default | Meaning |
| --- | --- | --- |
| `RUNCODES_REDIS_ADDR` | `redis:6379` | Redis address |
| `RUNCODES_REDIS_PASSWORD` | empty | Redis password |
| `RUNCODES_REDIS_DB` | `0` | Redis database number; must not be negative |

### Object storage

| Variable | Default | Meaning |
| --- | --- | --- |
| `RUNCODES_S3_ENDPOINT` | `http://seaweed:8333` | S3-compatible endpoint |
| `RUNCODES_S3_REGION` | `sa-east-1` | region |
| `RUNCODES_S3_CREDENTIALS_KEY` | empty | access key |
| `RUNCODES_S3_CREDENTIALS_SECRET` | empty | secret key |
| `RUNCODES_S3_BUCKET_PREFIX` | `runcodes` | bucket prefix; leading/trailing `-` trimmed |

Buckets: `<prefix>-commits`, `<prefix>-cases`, `<prefix>-files`,
`<prefix>-outputfiles`.

### Judge

| Variable | Default | Meaning |
| --- | --- | --- |
| `RUNCODES_JUDGE_URL` | `http://judge:9000` | absolute http(s) base URL |
| `RUNCODES_JUDGE_TOKEN` | empty | shared bearer token |
| `RUNCODES_JUDGE_STALE_TIMEOUT` | `35m` | age after which a claimed commit is marked `server_error` |
| `RUNCODES_JUDGE_MAX_QUEUE_DEPTH` | `100` | queued submissions above which new ones are refused with `503` |

`RUNCODES_JUDGE_STALE_TIMEOUT` must be positive and should be longer than the
judge's `JUDGE_MAX_RUN_DURATION` plus margin.

### Mail

| Variable | Default | Required | Meaning |
| --- | --- | :-: | --- |
| `RUNCODES_SMTP_HOST` | — | ✓ | SMTP server hostname, with no scheme |
| `RUNCODES_SMTP_PORT` | `587` | | SMTP port; STARTTLS, `465` is not supported |
| `RUNCODES_SMTP_FROM` | — | ✓ | valid `From` address |
| `RUNCODES_SMTP_USERNAME` | empty | | relay username |
| `RUNCODES_SMTP_PASSWORD` | empty | | relay password |
| `RUNCODES_DEADLINE_REMINDERS` | `true` | | email students without a submission when an exercise closes in a week and again in three days |
| `RUNCODES_DEADLINE_REMINDER_INTERVAL` | `30m` | | how often the deadline notifier looks for exercises entering a reminder window; at least `1m` |

STARTTLS is used when the server advertises it. In Compose the host is
`smtp4dev`, port 25; for a local backend outside Compose, `127.0.0.1:1025`.

### Telemetry

| Variable | Meaning |
| --- | --- |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | enables OTLP export; unset = no-op |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `http/protobuf` in the overlay |
| `OTEL_SERVICE_NAME` | `runcodes-backend` by default |
| `DEPLOYMENT_ENVIRONMENT` | `deployment.environment.name` |
| `OTEL_SERVICE_VERSION` | `service.version` |
| `OTEL_TRACES_SAMPLER`, `OTEL_TRACES_SAMPLER_ARG` | sampler and ratio |
| `OTEL_METRIC_EXPORT_INTERVAL` | metric export cadence |
| `OTEL_BSP_SCHEDULE_DELAY`, `OTEL_BLRP_SCHEDULE_DELAY` | span/log batch delay |
| `OTEL_SDK_DISABLED` | kill switch |
| `OTEL_RESOURCE_ATTRIBUTES` | extra resource attributes |
| `PPROF_ADDR` | internal pprof listener; empty disables |

See [Observability configuration](#observability) for the values used by the
overlay.

## Judge

Source: `judge/internal/config/config.go`, `judge/.env.example`.

### HTTP and worker pool

| Variable | Default | Meaning |
| --- | --- | --- |
| `JUDGE_ADDR` | `:9000` | listen address |
| `JUDGE_AUTH_TOKEN` / `RUNCODES_JUDGE_TOKEN` | empty | shared bearer token (the first wins) |
| `JUDGE_ALLOW_INSECURE` | `false` | serve `/v1` without a token; local only |
| `JUDGE_CONCURRENCY` | `4` | maximum concurrent containers; must be ≥ 1 |
| `JUDGE_POLL_INTERVAL` | `1s` | queue poll interval; must be positive |
| `JUDGE_EVENT_RETENTION` | `10m` | how long finished runs stay replayable in memory |

Without a token the judge refuses `/v1` with `503` unless
`JUDGE_ALLOW_INSECURE=true`.

### Podman and execution

| Variable | Default | Meaning |
| --- | --- | --- |
| `JUDGE_PODMAN_URI` | `unix:///run/user/<uid>/podman/podman.sock` | rootless Podman API |
| `JUDGE_IMAGE_REGISTRY` | `ghcr.io` | registry that holds the runner images |
| `JUDGE_RUNNER_IMAGE_PREFIX` | `runcodes-icmc/runcodes-runner` | image path inside the registry; the judge appends `-<language>` |
| `JUDGE_RUNNER_IMAGE_TAG` | `latest` | tag pulled for every runner image |
| `JUDGE_IMAGE_FORMAT` | derived | full printf template; overrides the registry, prefix and tag |
| `JUDGE_EXEC_DIR` | `$TMPDIR/runcodes-judge` | workspace directory |
| `JUDGE_EXEC_DIR_REMOTE` | same as `JUDGE_EXEC_DIR` | path as seen by the Podman service |
| `JUDGE_KEEP_WORKSPACES` | `false` | keep per-commit workspaces for inspection |

The judge combines the three settings as
`<registry>/<prefix>-<language>:<tag>`. For example:

```bash
JUDGE_IMAGE_REGISTRY=ghcr.io
JUDGE_RUNNER_IMAGE_PREFIX=reisaraujo-miguel/runcodes-runner
JUDGE_RUNNER_IMAGE_TAG=v1.2.3
```

pulls `ghcr.io/reisaraujo-miguel/runcodes-runner-c:v1.2.3` for a C submission.
A trailing slash on the registry and leading/trailing slashes or a trailing
hyphen on the prefix are ignored, so the Ansible prefix form
(`.../runcodes-runner-`) also works.

`JUDGE_IMAGE_FORMAT` is the advanced escape hatch. When it is set, it wins over
all three variables and its `%s` is replaced with the language suffix directly,
for example `registry.example.com/team/runners/%s:edge`.

### Timeouts and limits

| Variable | Default | Meaning |
| --- | --- | --- |
| `JUDGE_DEFAULT_COMPILATION_TIMEOUT` | `0` | container compilation timeout; 0 = image default |
| `JUDGE_COMPILATION_WAIT` | `2m` | judge's patience for compilation milestones |
| `JUDGE_DEFAULT_EXEC_TIMEOUT` | `5s` | default execution timeout |
| `JUDGE_DEFAULT_CASE_TIMEOUT` | `3s` | default per-case timeout |
| `JUDGE_MAX_RUN_DURATION` | `30m` | end-to-end budget per claim |
| `JUDGE_MONITOR_MAX_FILE_SIZE` | `0` | `monitor_max_fs` override; 0 sends nothing |
| `JUDGE_MONITOR_MAX_MEM_SIZE` | `0` | `monitor_max_ms` override; 0 sends nothing |
| `JUDGE_MAX_OUTPUT_FILE_SIZE` | `1 MiB` | max size of one collected output file |
| `JUDGE_CONTAINER_MEMORY_BYTES` | `1536 MiB` | cgroup memory/swap cap |
| `JUDGE_CONTAINER_PIDS_LIMIT` | `256` | cgroup pid cap |
| `JUDGE_CONTAINER_CPU_QUOTA` | `100000` | CPU quota per 100 ms period |
| `JUDGE_CONTAINER_TMPFS_BYTES` | `256 MiB` | writable `/tmp` cap when the rootfs is read-only |
| `JUDGE_MAX_COMPARE_FILE_BYTES` | `16 MiB` | max file a case is graded from |
| `JUDGE_MAX_ARTIFACT_BYTES` | `64 MiB` | max output archive |
| `JUDGE_ARTIFACT_TTL` | `168h` | retention for output archives; `0` keeps them |
| `JUDGE_MAX_EXTRACT_FILE_BYTES` | `64 MiB` | max single zip entry |
| `JUDGE_MAX_EXTRACT_BYTES` | `256 MiB` | max total expanded zip |
| `JUDGE_MAX_EXTRACT_ENTRIES` | `1000` | max entries in a zip submission |
| `JUDGE_MAX_WORKSPACE_BYTES` | `1 GiB` | aggregate workspace size sampled while a run is alive; `0` disables |
| `JUDGE_MAX_WORKSPACE_ENTRIES` | `50000` | aggregate workspace entry/inode count sampled while a run is alive; `0` disables |

Validation rules:

- `JUDGE_COMPILATION_WAIT` must be positive and, when
  `JUDGE_DEFAULT_COMPILATION_TIMEOUT > 0`, strictly greater than it.
- workspace limits must not be negative; `0` disables either cap.
- extraction limits must be positive and the per-entry limit must not exceed the
  total.
- container and monitor limits must not be negative.

### Judge database and S3

| Variable | Default | Meaning |
| --- | --- | --- |
| `RUNCODES_DB_HOST` | `localhost` | PostgreSQL host |
| `RUNCODES_DB_PORT` | `5432` | port |
| `RUNCODES_DB_NAME` | `runcodes` | database |
| `RUNCODES_DB_USER` | `runcodes` (`RUNCODES_DB_USERNAME` accepted) | use `runcodes_judge` |
| `RUNCODES_DB_PASSWORD` | empty | password |
| `RUNCODES_DB_SSLMODE` | `disable` | lib/pq SSL mode |
| `RUNCODES_S3_ENDPOINT` | `http://localhost:8333` | S3 endpoint |
| `RUNCODES_S3_REGION` | `sa-east-1` | region |
| `RUNCODES_S3_CREDENTIALS_KEY` | `test_key` | access key |
| `RUNCODES_S3_CREDENTIALS_SECRET` | `test_secret` | secret key |
| `RUNCODES_S3_BUCKET_PREFIX` | `runcodes` | bucket prefix |

The judge's defaults for database credentials are development defaults; a
deployment must set real values. `JUDGE_PODMAN_URI` is required in practice and
defaults to the invoking user's socket.

### Judge telemetry

The same standard `OTEL_*` variables and `PPROF_ADDR` as the backend. Set
`OTEL_SERVICE_NAME=runcodes-judge`.

## Frontend

Source: `frontend/.env.example`.

### Build-time variables

| Variable | Default | Meaning |
| --- | --- | --- |
| `VITE_API_ENDPOINT` | empty | absolute API origin; empty = relative `/api` through Caddy |
| `VITE_CONTACT_EMAIL` | `contact@example.com` | fallback until public settings load |
| `VITE_CONTACT_DISCLAIMER_HTML` | example HTML | fallback disclaimer |
| `VITE_OTEL_ENABLED` | `false` | enable the Web SDK |
| `VITE_OTEL_EXPORTER_OTLP_ENDPOINT` | empty | browser-reachable collector URL |
| `VITE_OTEL_SERVICE_NAME` | `runcodes-frontend` | browser service name |
| `VITE_OTEL_SERVICE_VERSION` | `dev` | browser service version |
| `VITE_DEPLOYMENT_ENVIRONMENT` | `development` | browser environment |
| `VITE_OTEL_TRACES_SAMPLER_ARG` | `1.0` | browser sampling ratio |

Vite bakes these into the bundle: rebuild the image after changing them. The
production image sets `VITE_API_ENDPOINT=` (same origin) and, in the
four-machine release, `VITE_OTEL_EXPORTER_OTLP_ENDPOINT=/otel`.

### Caddy runtime variables

| Variable | Default | Meaning |
| --- | --- | --- |
| `RUNCODES_DOMAIN` | `:80` | Caddy site address; hostname enables automatic HTTPS |
| `BACKEND_UPSTREAM` | `http://backend:8443` | API upstream |

## Compose (source build)

Source: `docker-compose.yml`, `.env.example`. The same file works with Podman
Compose (default) and Docker Compose v2.

| Variable | Required | Used by | Meaning |
| --- | :-: | --- | --- |
| `RUNCODES_JWT_SECRET` | ✓ | backend | session signing |
| `RUNCODES_DB_PASSWORD` | ✓ | database | owner password |
| `RUNCODES_DB_APP_PASSWORD` | ✓ | backend, database init | app role password |
| `RUNCODES_DB_JUDGE_PASSWORD` | ✓ | judge, database init | judge role password |
| `RUNCODES_REDIS_PASSWORD` | ✓ | Redis, backend | cache password |
| `RUNCODES_JUDGE_TOKEN` | ✓ | backend, judge | shared token |
| `RUNCODES_S3_CREDENTIALS_KEY` | ✓ | backend, judge, SeaweedFS | S3 key |
| `RUNCODES_S3_CREDENTIALS_SECRET` | ✓ | backend, judge, SeaweedFS | S3 secret |
| `RUNCODES_PUBLIC_URL` | | backend | verification and recovery link origin |
| `RUNCODES_SMTP_FROM` | | backend | sender |
| `RUNCODES_SMTP_USERNAME` / `_PASSWORD` | | backend | relay credentials |
| `RUNCODES_LEGACY_PASSWORD_SALT` | | backend | legacy hashes |
| `FRONTEND_HTTP_PORT` / `FRONTEND_HTTPS_PORT` | | frontend | host ports (8080/8444) |
| `BACKEND_PORT` / `JUDGE_PORT` | | backend, judge | loopback host ports (8443/9000) |
| `DATABASE_PORT` / `SEAWEED_S3_PORT` | | database, seaweed | loopback host ports (5432/8333) |
| `SMTP4DEV_WEB_PORT` / `SMTP4DEV_SMTP_PORT` | | smtp4dev | loopback host ports (8081/1025) |
| `RUNCODES_DOMAIN` | | frontend | Caddy site address |
| `PODMAN_SOCKET` | | judge | rootless socket path |
| `RUNCODES_IMAGE_REGISTRY` | `ghcr.io` | judge | runner image registry |
| `RUNCODES_RUNNER_IMAGE_PREFIX` | `runcodes-icmc/runcodes-runner` | judge | runner image path inside the registry |
| `RUNCODES_RUNNER_IMAGE_TAG` | `latest` | judge | runner image tag |
| `JUDGE_IMAGE_FORMAT` | | judge | advanced runner image template; wins over the three variables above |
| `JUDGE_UID` / `JUDGE_GID` | | judge | container identity (0:0) |
| `CONTACT_EMAIL` / `CONTACT_DISCLAIMER` | | frontend build | login-page fallbacks |
| `DEPLOYMENT_ENVIRONMENT` | | observability | environment label |

Compose uses `:?` for the required secrets, so it fails before starting with an
explicit message.

## Image-based Compose

Source: `deploy/compose/docker-compose.yml`, `deploy/compose/.env.example`. The
stack runs on Podman Compose by default and on Docker Compose v2 when selected.

The `Default` column lists the Compose file's inline fallbacks.
`deploy/compose/.env.example` is a public-deployment profile on top of them: a
placeholder hostname in `RUNCODES_DOMAIN`, an `https://` `RUNCODES_PUBLIC_URL`,
and frontend host ports `80`/`443`. Ports below 1024 need
`net.ipv4.ip_unprivileged_port_start=80` on a rootless Podman host; see
[Image-based Compose § Public hostname and TLS](../deployment/image-compose.md#public-hostname-and-tls).

| Variable | Default | Meaning |
| --- | --- | --- |
| `COMPOSE_PROJECT_NAME` | `runcodes` | Compose project and volume prefix |
| `CONTAINER_ENGINE` | `auto` | `auto` prefers Podman and falls back to Docker; `podman` or `docker` forces one engine. An environment value overrides `.env`. |
| `RUNCODES_IMAGE_REGISTRY` | `ghcr.io` | registry for application and runner images |
| `RUNCODES_IMAGE_NAMESPACE` | `runcodes-icmc` | namespace holding the four application images |
| `RUNCODES_IMAGE_TAG` | `latest` | application image tag |
| `RUNCODES_RUNNER_IMAGE_PREFIX` | `runcodes-icmc/runcodes-runner` | runner image path inside the registry |
| `RUNCODES_RUNNER_IMAGE_TAG` | `latest` | runner image tag |
| `JUDGE_IMAGE_FORMAT` | derived | full runner template; wins over registry/prefix/tag |
| `RUNCODES_EXEC_DIR` | `./exec` | host directory for judge workspaces; the script makes it absolute |
| `PODMAN_SOCKET` | `/run/user/<uid>/podman/podman.sock` | rootless Podman API socket |
| `RUNCODES_DOMAIN` | `:80` | Caddy site address; a hostname enables HTTPS |
| `RUNCODES_PUBLIC_URL` | `http://localhost:8080` | browser origin used in mailed links |
| `FRONTEND_HTTP_PORT` / `FRONTEND_HTTPS_PORT` | `8080` / `8444` | frontend host ports |
| `BACKEND_PORT` / `JUDGE_PORT` | `8443` / `9000` | loopback host ports |
| `DATABASE_PORT` / `SEAWEED_S3_PORT` | `5432` / `8333` | loopback host ports |
| `SMTP4DEV_WEB_PORT` / `SMTP4DEV_SMTP_PORT` | `8081` / `1025` | loopback host ports |
| `RUNCODES_SMTP_HOST` / `_PORT` | `smtp4dev` / `25` | relay or the bundled catcher; bare hostname, STARTTLS on `587` |
| `RUNCODES_SMTP_FROM` | `RunCodes <no-reply@runcodes.local>` | sender |
| `RUNCODES_SMTP_USERNAME` / `_PASSWORD` | empty | relay credentials |
| `JUDGE_CONCURRENCY` | `4` | containers the judge runs at once |
| `SEAWEEDFS_VERSION` | `4.48` | public SeaweedFS image tag |

The required secrets are the same as the source-build stack. The
[image-based deployment page](../deployment/image-compose.md) documents the
workflow, first boot and upgrades.

## Observability

Source: `docker-compose.observability.yml`, `.env.example`.

| Variable | Default | Meaning |
| --- | --- | --- |
| `OBSERVABILITY_BIND_ADDRESS` | `127.0.0.1` | host interface for all published ports |
| `GRAFANA_ADMIN_USER` | `admin` | Grafana login |
| `GRAFANA_ADMIN_PASSWORD` | generated file | not read from the environment |
| `GRAFANA_ROOT_URL` | `http://localhost:3000` | links generated by Grafana |
| `GRAFANA_PORT` | `3000` | Grafana |
| `PROMETHEUS_PORT` | `9090` | Prometheus |
| `ALERTMANAGER_PORT` | `9093` | Alertmanager |
| `ALERT_SINK_PORT` | `8085` | test receiver |
| `LOKI_PORT` | `3100` | Loki |
| `TEMPO_PORT` | `3200` | Tempo |
| `PYROSCOPE_PORT` | `4040` | Pyroscope |
| `ALLOY_PORT` | `12345` | Alloy |
| `OTEL_GRPC_PORT` | `4317` | OTLP/gRPC |
| `OTEL_HTTP_PORT` | `4318` | OTLP/HTTP |
| `OTEL_HEALTH_PORT` | `13133` | collector health |
| `OTEL_METRICS_PORT` | `8888` | collector self-metrics |
| `OTEL_PROM_PORT` | `8889` | application metrics |
| `NODE_EXPORTER_PORT` | `9100` | node-exporter |
| `POSTGRES_EXPORTER_PORT` | `9187` | postgres-exporter |
| `REDIS_EXPORTER_PORT` | `9121` | redis-exporter |
| `BLACKBOX_EXPORTER_PORT` | `9115` | blackbox-exporter |
| `PROMETHEUS_RETENTION_TIME` | `15d` | Prometheus TSDB |
| `PROMETHEUS_RETENTION_SIZE` | `8GB` | Prometheus TSDB |
| `PYROSCOPE_CONFIG_FILE` | `./observability/pyroscope/config.yaml` | alternate config |
| `ALERTMANAGER_CONFIG_FILE` | `./observability/alertmanager/alertmanager.yml` | alternate Alertmanager config (Gmail example) |
| `FRONTEND_OTEL_ENABLED` | `true` | browser build input |
| `PUBLIC_OTEL_HTTP_URL` | `http://localhost:4318` | browser collector URL |
| `VITE_OTEL_TRACES_SAMPLER_ARG` | `1.0` | browser sampling |

The Grafana admin password, Alertmanager webhook URL and optional Gmail app
password are files in `.observability/`, created by
`scripts/observability/init-secrets.sh`.

## Ansible deployment

Source: `deploy/ansible/inventories/group_vars/all/config.yml.example` and
`vault.yml.example`. Copy them to untracked `config.yml` and encrypted
`vault.yml`; the tracked `main.yml` only maps names.

### Release and images

| Setting | Default | Meaning |
| --- | --- | --- |
| `config_runcodes_cluster_name` | `runcodes-prod` | cluster label |
| `config_runcodes_environment` | `production` | environment label |
| `config_runcodes_version` | `latest` | `service.version` for backend/judge |
| `config_runcodes_image_registry` | `ghcr.io` | registry |
| `config_runcodes_image_namespace` | `runcodes-icmc` | application image namespace |
| `config_runcodes_image_tag` | `latest` | application image tag |
| `config_runcodes_pull_images` | `true` | pull before restart |
| `config_runcodes_runner_tag` | `latest` | runner image tag |
| `config_runcodes_runner_image_prefix` | `ghcr.io/runcodes-icmc/runcodes-runner-` | runner prefix, ending in `-`; set the namespace here too |
| `config_runcodes_prepull_runner_images` | `true` | pre-pull runners |
| `config_runcodes_runner_languages` | 22 languages | languages to pre-pull |

### Edge and filesystem

| Setting | Default | Meaning |
| --- | --- | --- |
| `config_runcodes_domain` | `runcodes.example.com` | Caddy site address |
| `config_runcodes_public_url` | `https://runcodes.example.com` | browser-facing origin |
| `config_runcodes_backend_debug_mode` | `false` | HTTP-only labs only |
| `config_runcodes_deploy_user` | `runcodes` | SSH/deployment user |
| `config_runcodes_root` | `/opt/runcodes` | install root |
| `config_runcodes_timezone` | `America/Sao_Paulo` | host timezone |

### Application

| Setting | Default | Meaning |
| --- | --- | --- |
| `config_runcodes_db_name` | `runcodes` | database name |
| `config_runcodes_db_owner` | `runcodes` | owner role |
| `config_runcodes_redis_db` | `0` | Redis DB |
| `config_runcodes_s3_region` | `sa-east-1` | S3 region |
| `config_runcodes_s3_bucket_prefix` | `runcodes` | bucket prefix |
| `config_runcodes_smtp_host` | `smtp4dev` | SMTP host |
| `config_runcodes_smtp_port` | `25` | SMTP port |
| `config_runcodes_smtp_from` | example address | sender |
| `config_runcodes_bootstrap_admin` | `true` | run admin bootstrap while hash is `!` |
| `config_runcodes_admin_email` | `admin@admin.com` | admin account |

### Judge

| Setting | Default | Meaning |
| --- | --- | --- |
| `config_runcodes_judge_concurrency` | `4` | concurrent containers |
| `config_runcodes_judge_exec_dir` | `/opt/runcodes/exec` | execution directory |
| `config_runcodes_judge_stale_timeout` | `20m` | backend stale timeout |
| `config_runcodes_judge_max_run_duration` | `15m` | per-run budget |
| `config_runcodes_judge_compilation_wait` | `2m` | compilation patience |
| `config_runcodes_judge_poll_interval` | `1s` | poll interval |
| `config_runcodes_judge_container_memory_bytes` | `1610612736` | 1.5 GiB |
| `config_runcodes_judge_container_pids_limit` | `256` | pid cap |
| `config_runcodes_judge_container_cpu_quota` | `100000` | CPU quota |
| `config_runcodes_judge_container_tmpfs_bytes` | `268435456` | read-only rootfs `/tmp` cap |
| `config_runcodes_judge_max_workspace_bytes` | `1073741824` | sampled aggregate workspace bytes |
| `config_runcodes_judge_max_workspace_entries` | `50000` | sampled aggregate workspace entries |

Keep the stale timeout above the max run duration.

### OpenTelemetry and storage

| Setting | Default | Meaning |
| --- | --- | --- |
| `config_runcodes_otel_traces_sampler` | `parentbased_always_on` | sampler |
| `config_runcodes_otel_traces_sampler_arg` | `1.0` | ratio |
| `config_runcodes_otel_metric_export_interval` | `10000` | ms |
| `config_runcodes_prometheus_retention_time` | `15d` | Prometheus |
| `config_runcodes_prometheus_retention_size` | `8GB` | Prometheus |
| `config_runcodes_tempo_retention_hours` | `168` | Tempo |
| `config_runcodes_grafana_admin_user` | `admin` | Grafana |
| `config_runcodes_grafana_root_url` | `http://localhost:3000` | Grafana links |
| `config_runcodes_alertmanager_gmail_enabled` | `false` | email alerts through Gmail |
| `config_runcodes_alertmanager_gmail_smarthost` | `smtp.gmail.com:587` | Gmail/Workspace SMTP host |
| `config_runcodes_alertmanager_gmail_from` | example address | sender and SMTP username |
| `config_runcodes_alertmanager_gmail_to` | `["oncall@example.com"]` | recipient list |

### Firewall and verification

| Setting | Default | Meaning |
| --- | --- | --- |
| `config_runcodes_firewall_enabled` | `false` | enable UFW role |
| `config_runcodes_firewall_private_sources` | four node addresses | allowed private peers |
| `config_runcodes_firewall_ssh_cidrs` | example CIDR | allowed SSH sources |
| `config_runcodes_firewall_container_sources` | `10.88.0.0/16`, `10.89.0.0/16` | Podman networks |
| `config_runcodes_verify_public_url` | `true` | probe the public URL |
| `config_runcodes_verify_tls` | `true` | validate TLS |
| `config_runcodes_run_smoke_test` | `true` | run the smoke test |
| `config_runcodes_run_submission_smoke_test` | `false` | run the authenticated submission test |

### Ansible vault

| Secret | Purpose |
| --- | --- |
| `vault_runcodes_jwt_secret` | session signing |
| `vault_runcodes_judge_token` | backend ↔ judge |
| `vault_runcodes_db_owner_password` | PostgreSQL owner |
| `vault_runcodes_db_app_password` | app role |
| `vault_runcodes_db_judge_password` | judge role |
| `vault_runcodes_s3_access_key` / `_secret_key` | SeaweedFS S3 |
| `vault_runcodes_redis_password` | Redis |
| `vault_runcodes_admin_password` | seeded admin |
| `vault_grafana_admin_password` | Grafana |
| `vault_runcodes_legacy_password_salt` | legacy users |
| `vault_runcodes_smtp_username` / `_password` | SMTP relay |
| `vault_alertmanager_webhook_url` | alert receiver |
| `vault_alertmanager_gmail_app_password` | Gmail alert email |
| `vault_registry_username` / `_password` | private GHCR |

Never use a literal `$` in a secret: `podman-compose` interpolates `.env`
values. The preflight rejects `$` and the `REPLACE_WITH` placeholders.

## Documentation site

| Variable | Default | Meaning |
| --- | --- | --- |
| `DOCS_BASE` | `/runcodes/` | VitePress base path; set to `/` for a root deployment |

The GitHub Pages workflow sets `DOCS_BASE=/runcodes/`. See
[Documentation site](../documentation.md).

## Related pages

- [Ports & storage](./ports-and-storage.md)
- [Ansible setup](../deployment/ansible-setup.md)
- [Local observability stack](../observability/local.md)
