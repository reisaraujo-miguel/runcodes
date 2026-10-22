# PR: end-to-end observability for RunCodes

## Summary

Adds an opt-in LGTM-style observability stack to the RunCodes platform:

- **Instrumentation** of the Go backend and judge with the OpenTelemetry SDK:
  traces (HTTP, SQL, S3, judge calls), HTTP/runtime/DB metrics, and structured
  logs with trace correlation.
- An **OpenTelemetry Collector** gateway that exports traces to Tempo, logs to
  Loki and metrics to a Prometheus endpoint.
- **Prometheus** (application, trace-derived and infrastructure metrics),
  **Loki** (logs), **Tempo** (traces + span metrics/service graph), **Grafana**
  (provisioned datasources, dashboard, alerting), and **Alertmanager** with a
  local webhook sink for verification.
- A **Compose overlay**, Makefile targets, config-as-code, a smoke test and
  operator documentation.
- A new **readiness endpoint** for the backend and graceful shutdown so
  telemetry is flushed on SIGTERM.

The existing stack is unchanged unless the overlay is explicitly enabled.

## Why

The repository had structured logging and a liveness endpoint, but no metrics,
tracing, log aggregation, dashboards or alerting. This makes latency and failure
investigation depend on `docker logs` and manual database inspection. The PR
establishes a single correlation path: a request's trace id is in the response
header, its spans are in Tempo, and the same id is attached to the OTel log
records stored in Loki.

## What changed

### Backend (`backend/`)

- New `backend/telemetry` package:
  - OTLP trace, metric and log exporters, resource attributes (`service.name`,
    `service.namespace`, `service.version`, `service.instance.id`,
    `deployment.environment.name`), sampler and batching configuration from
    standard OTel env vars.
  - chi middleware for server spans, request ids (`TraceId` header), and the
    semantic-convention HTTP metrics (`http.server.request.duration`,
    `http.server.active_requests`).
  - A slog handler that keeps stdout logs and forwards the same records to OTLP;
    stdout records gain `trace_id`/`span_id`.
- `main.go`: telemetry setup/shutdown, signal-aware graceful shutdown.
- `router.go`: `otelchi`/trace-id/metrics middleware, `/readyz`, CORS trace
  headers, exposes `TraceId`.
- `handlers/ready-handler.go` (+ tests): database is required, judge is
  reported but does not fail readiness.
- `database` and `storage`: `otelsql` spans/pool metrics and `otelhttp` on S3.
- `judge/client.go`: `otelhttp` transport for judge and SSE calls.
- `judge/reconcile.go`: a span for the background sweeper.
- `logger.go`: composes console + OTel log handlers.

### Judge (`judge/`)

- New `judge/internal/telemetry` package (same design as the backend).
- Instrumented API (`otelchi`, metrics, trace ids), PostgreSQL (`otelsql`), S3
  (`otelhttp`), and a root span per claimed commit in the worker.
- Graceful shutdown flushes telemetry; the existing `/healthz`/`/readyz`
  behaviour is unchanged.

### Stack (`docker-compose.observability.yml`, `observability/`)

- Pinned images: Collector contrib 0.163.2, Prometheus 3.15.1, Alertmanager
  0.34.1, Loki 3.7.9, Tempo 3.1.1, Grafana 12.4.13, node/postgres/redis/blackbox
  exporters.
- Collector pipelines for traces (`otlp_grpc/tempo`), logs
  (`otlp_http/loki:/otlp`) and metrics (`prometheus` exporter on `:8889`), with
  memory limiting, batching, queues, retries and low-cardinality resource
  labels.
- Prometheus scrape configs for the collector, its own telemetry, Loki, Tempo,
  Grafana, Alertmanager and the infrastructure exporters, plus blackbox probes
  and 13 recording/alerting rules.
- Loki OTLP ingestion with 7-day retention, structured metadata and three index
  labels.
- Tempo monolithic mode with local storage, 7-day block retention and a
  metrics-generator remote-writing span metrics/service graphs to Prometheus.
- Grafana provisioning:
  - Prometheus, Loki, Tempo and Alertmanager datasources, with trace/log/metric
    correlation and exemplars.
  - `RunCodes — Observability Overview` dashboard (request rate, errors,
    latency, logs, trace span metrics, resources, infrastructure).
  - A Grafana-managed example alert rule and a `prometheus-alertmanager` contact
    point linked to Alertmanager.
- `alert-sink` (small Python webhook receiver) so alert delivery is testable
  end to end.
- Every published observability port binds to 127.0.0.1 by default.

### Tooling and docs

- `Makefile`: `observability-init`, `-up`, `-down`, `-status`, `-logs`,
  `-smoke-test`, `-validate`, `-destroy` (the last one removes only the
  observability data volumes, after a confirmation prompt).
- `scripts/observability/init-secrets.sh`: generates the local Grafana password
  in gitignored `.observability/`.
- `scripts/observability/validate.sh`: compose config, promtool, amtool,
  collector validate, dashboard JSON and shell syntax.
- `scripts/observability/smoke-test.sh`: starts/verifies the stack, generates
  traffic, follows one trace id through Tempo and Loki, checks Grafana
  datasources/dashboard/alerting, and verifies Alertmanager delivery to the
  sink.
- `docs/observability/README.md` and `runbook.md`; sections in the root and
  service READMEs; `.env.example` updates.
- `.github/workflows/observability.yml` runs the static validation on relevant
  changes.
- `.gitignore`: `.cache/` and `.observability/` (local build cache and generated
  secret).

## How to run

```bash
make observability-init
make observability-up
make observability-smoke-test
```

Then open <http://localhost:3000/d/runcodes-overview>. The Grafana admin
password is in `.observability/grafana_admin_password` (gitignored).

Explicit equivalent:

```bash
docker compose -f docker-compose.yml -f docker-compose.observability.yml up -d --build
```

If the environment's Docker config directory is read-only, set
`DOCKER_CONFIG=$PWD/.cache/docker` before `make observability-up`.

## Validation

| Check | Result |
| --- | --- |
| Backend `gofmt -l .` | clean |
| Backend `go vet ./...` | pass |
| Backend `go test -count=1 ./...` | pass (all packages) |
| Judge `gofmt -l .` | clean |
| Judge `go vet -tags ... ./...` | pass |
| Judge `go test -tags ... -count=1 ./...` | pass (all packages) |
| `make observability-validate` | pass (compose, promtool, amtool, collector, JSON/shell) |
| Full stack starts from `make observability-down` + `make observability-up` | pass |
| `make observability-smoke-test` | pass (15s, all checks; see below) |
| Frontend `bun test` | 131 pass, 0 fail |
| Frontend `bun run lint` | pass |

Smoke test observed:

- `TraceId` returned by the backend and found in Tempo.
- The same `/readyz` trace contains both `runcodes-backend` and `runcodes-judge`
  spans (W3C context propagation verified in Tempo).
- Prometheus contains `http_server_request_duration_seconds_count{service_name="runcodes-backend"}` and `...="runcodes-judge"`.
- Loki contains the smoke marker, judge logs, and the exact trace's log line via `trace_id` structured metadata.
- Grafana Prometheus/Loki/Tempo datasources healthy; Alertmanager proxy works; dashboard and alert rule provisioned.
- Alertmanager accepts an injected alert and delivers it to `alert-sink`.
- The Grafana-managed example rule and its `prometheus-alertmanager` contact
  point are provisioned and visible through the Grafana API.

Final smoke run (fresh `make observability-down` + `make observability-up`):

```
PASS backend HTTP metrics in Prometheus
PASS judge HTTP metrics in Prometheus
PASS backend logs in Loki (9 log line(s))
PASS judge logs in Loki (9 log line(s))
PASS trace-correlated log line in Loki (1 log line(s))
PASS trace <32-hex-id> in Tempo is reachable
PASS Grafana datasource Prometheus/Loki/Tempo is healthy
PASS Grafana Alertmanager datasource proxy is working
PASS Grafana dashboard is provisioned
PASS Grafana has 1 provisioned alert rule(s)
PASS Grafana has an Alertmanager contact point
PASS Alertmanager accepted the test alert
PASS Alertmanager shows the injected alert
PASS Alertmanager delivered the alert to the webhook sink
All observability smoke checks passed.
```

## Secrets / security

- No credentials, tokens or private keys are committed.
- The Grafana admin password is generated locally into `.observability/`, which
  is gitignored. The Compose file only references the mounted file.
- All observability UIs/APIs are loopback-only by default.
- OTLP and the backends are unauthenticated inside the Compose network; the
  README and runbook document the production TLS/auth requirements.
- `.env.example` contains placeholders only.

## Risks and rollback

- The application now exports telemetry when `OTEL_EXPORTER_OTLP_ENDPOINT` is
  set. Telemetry is asynchronous and cannot fail requests; a missing collector
  only produces debug-level exporter errors. Without an endpoint the SDK is a
  no-op.
- `/readyz` adds a short judge probe. It does not fail readiness when the judge
  is down, so an orchestrator will not remove a healthy API.
- The overlay is a new file; `docker compose up` is unchanged. Rollback is
  `make observability-down` and reverting the PR; no existing volumes or
  configuration are modified by the overlay.
- Backend/judge images grow due to the OTel dependencies (the OTel SDK plus
  protobuf/gRPC). This is expected and measured in the image build.

## Follow-ups

- Browser/frontend tracing (`@opentelemetry/sdk-trace-web` or Grafana Faro) and
  collector OTLP HTTP CORS.
- Optional Pyroscope with Alloy `pyroscope.scrape` or `pyroscope-go`, plus pprof
  endpoints on the Go services.
- Instrument individual judge compilation/test-case phases and add queue-depth
  metrics from the durable PostgreSQL queue.
- Collect container logs (not only OTel application logs) with Alloy or a
  filelog receiver.
- Kubernetes/Helm manifests or kube-prometheus-stack values for production;
  this repo currently uses Compose.
- Tail sampling and production retention/object storage for Loki and Tempo.

## Assumptions and open questions

- The repository has no Kubernetes or Helm deployment; Compose is the simplest
  reproducible runtime and is the one implemented. Production guidance is
  documented rather than shipped as manifests.
- Prometheus scrapes the collector's OTLP-to-Prometheus exporter instead of the
  applications directly, matching the requested `app -> collector -> backend`
  flow.
- The local `alert-sink` is a test fixture; the intended production receivers
  are Alertmanager webhooks/Slack/PagerDuty/email configured by the operator.
- `deployment.environment.name` is used (the current semantic convention), not
  the older `deployment.environment`.
- Service versions default to `dev`; a release can set `OTEL_SERVICE_VERSION`
  from its build metadata.
