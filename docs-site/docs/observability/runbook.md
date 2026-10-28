---
title: Observability runbook
description: Operational procedures for the telemetry stack — validation, failure modes, alert testing, retention and queries.
---

# Observability runbook

Operational procedures for the stack described in
[Local observability stack](./local.md) and
[Four-machine observability](./production.md). Commands assume the repository
root and a working Podman Compose (the default) or Docker Compose v2;
substitute the four-machine paths where noted.

::: tip Podman is the default, Docker is supported
The examples use `podman compose`. Replace it with `docker compose` for Docker
Compose v2; set `CONTAINER_ENGINE=docker` for the Make targets and helper
scripts.
:::

## Starting and stopping

```bash
# First time (and after deleting .observability/)
make observability-init

# Build the instrumented backend/judge and start the whole stack
make observability-up

# Status and logs
make observability-status
make observability-logs

# Follow one service
podman compose -f docker-compose.yml -f docker-compose.observability.yml \
  logs -f backend
podman compose -f docker-compose.yml -f docker-compose.observability.yml \
  logs -f otel-collector

# Stop, keep data volumes
make observability-down

# Stop and delete only the observability data volumes (prompts)
make observability-destroy
FORCE=1 make observability-destroy
```

If `make observability-up` fails while building with a read-only Docker config
(for example in a sandbox), point Docker at a writable configuration directory:

```bash
export DOCKER_CONFIG="$PWD/.cache/docker"
make observability-up
```

This workaround is Docker-specific; Podman uses its own configuration
directory.

The non-application images and versions are pinned in
`docker-compose.observability.yml`. Changing a version requires updating
`scripts/observability/validate.sh` and the dashboard/rules if semantic
conventions changed.

## Validating the stack

```bash
make observability-validate            # static: compose, promtool, amtool, collector
make observability-smoke-test          # end to end; starts the stack if needed
make observability-browser-smoke-test  # real Chromium trace
SMOKE_ADMIN_PASSWORD='...' make observability-submission-smoke-test
```

`SMOKE_START=0` fails instead of starting a stack that is not running.
`SMOKE_BUILD=0` reuses existing images. Endpoints can be overridden with
`BACKEND_URL`, `PROMETHEUS_URL`, `LOKI_URL`, `TEMPO_URL`, `GRAFANA_URL`,
`ALERTMANAGER_URL`, etc. The Grafana password is read from
`.observability/grafana_admin_password` unless `GRAFANA_ADMIN_PASSWORD` is set.

Expected output ends with `All observability smoke checks passed.` On failure the
script prints the Compose status (`podman compose ps` or `docker compose ps`),
the last 40 lines of each service log and the last API responses, then exits
non-zero.

### Manual checks

**Metrics** (Prometheus):

```bash
curl -s http://localhost:9090/api/v1/query \
  --data-urlencode 'query=sum(http_server_request_duration_seconds_count{service_name="runcodes-backend"})' \
  | jq '.data.result[0].value'
curl -s http://localhost:9090/api/v1/targets | jq -r \
  '.data.activeTargets[] | "\(.labels.job)\t\(.health)\t\(.lastError // "")"'
```

**Logs** (Loki):

```bash
curl -sG http://localhost:3100/loki/api/v1/labels | jq '.data'
curl -sG http://localhost:3100/loki/api/v1/query_range \
  --data-urlencode 'query={service_name="runcodes-backend"} |= "error"' \
  --data-urlencode "start=$(( $(date +%s) - 900 ))" \
  --data-urlencode "end=$(date +%s)" --data-urlencode 'limit=20' \
  | jq '.data.result | length'
curl -sG http://localhost:3100/loki/api/v1/query_range \
  --data-urlencode 'query={service_name="runcodes-backend"} | trace_id="<32-hex-trace-id>"' \
  --data-urlencode "start=$(( $(date +%s) - 900 ))" \
  --data-urlencode "end=$(date +%s)" --data-urlencode 'limit=20' \
  | jq '.data.result | length'
```

**Traces** (Tempo):

```bash
curl -sG http://localhost:3200/api/search \
  --data-urlencode 'tags=service.name=runcodes-backend' \
  --data-urlencode 'limit=20' | jq '.traces | length'
curl -s -o /dev/null -w '%{http_code}\n' \
  http://localhost:3200/api/traces/<32-hex-trace-id>
```

**Grafana datasources and dashboard**:

```bash
PASSWORD=$(cat .observability/grafana_admin_password)
for ds in prometheus loki tempo pyroscope; do
  curl -s -u "admin:$PASSWORD" \
    "http://localhost:3000/api/datasources/uid/$ds/health" | jq -c .
done
curl -s -u "admin:$PASSWORD" \
  http://localhost:3000/api/dashboards/uid/runcodes-overview \
  | jq -r '.dashboard.title'
```

**Profiles** (Pyroscope/Alloy):

```bash
curl -s http://localhost:4040/ready
curl -s http://localhost:12345/-/healthy
curl -s http://localhost:12345/metrics | grep pyroscope_write_sent_profiles_total
curl -sG http://localhost:9090/api/v1/query \
  --data-urlencode 'query=sum(pyroscope_distributor_profiles_received_total)' \
  | jq -c .
```

**Judge queue and browser path**:

```bash
curl -sG http://localhost:9090/api/v1/query \
  --data-urlencode 'query=runcodes_judge_queue_depth' | jq -c '.data.result'
curl -s -o /dev/null -w '%{http_code}\n' -X OPTIONS \
  http://localhost:4318/v1/traces \
  -H 'Origin: http://localhost:8080' \
  -H 'Access-Control-Request-Method: POST' \
  -H 'Access-Control-Request-Headers: content-type'
```

**Trace to profiles**:

```bash
TRACE_ID=<32-hex-trace-id>
curl -s "http://localhost:3200/api/traces/$TRACE_ID" \
  | jq -r '.batches[].scopeSpans[].spans[]
      | select(.name=="judge.grade_test_case")
      | .attributes[]
      | select(.key=="pyroscope.profile.id")
      | .value.stringValue'
```

In Grafana Explore, open the trace, expand the root/grade span, and use
**Profiles for this span**. The Pyroscope data source must be healthy and the
Tempo data source must have `tracesToProfiles` (provisioned).

The external Alertmanager datasource intentionally returns `plugin.unavailable`
from Grafana's generic health endpoint. Check its proxy instead:

```bash
curl -s -u "admin:$PASSWORD" \
  http://localhost:3000/api/alertmanager/alertmanager/api/v2/status \
  | jq '.cluster'
```

## Expected startup noise

These are normal:

- Loki: `error getting ingester clients: empty ring` during the first seconds.
- Loki: `Ingester not ready: waiting for 15s after being ready` from `/ready`.
- Tempo 3.1: `error calling scheduler ... no jobs found` on an idle compaction
  queue.
- Pyroscope 2.x: `Metastore not ready` / `Ingester not ready` / `Segment Writer
  not ready` while single-binary components do rolling-update safety delays.
  Compose sets `min-ready-duration=0` for local use.
- Grafana: `Plugin unavailable` if you call
  `/api/datasources/uid/alertmanager/health` directly; use the proxy.
- Collector: deprecation warnings from optional exporter settings.

`make observability-validate` fails on invalid configuration, not deprecation
notices.

## Common failure modes

### Containers restart with "permission denied" reading a config file

Symptom: `loki`, `tempo`, `prometheus`, `alertmanager`, `otel-collector`,
`grafana` or `alert-sink` restart with `open /etc/...: permission denied`.

The services run as non-root and repository files must be world-readable. A
checkout normally has mode 0644; a restrictive umask or an editor may leave 0600.

```bash
chmod -R a+rX observability
chmod a+r .observability/grafana_admin_password
podman compose -f docker-compose.yml -f docker-compose.observability.yml \
  up -d --force-recreate loki tempo prometheus alertmanager otel-collector grafana alert-sink
```

The services use `security_opt: label=disable` because the repo may be under
`/home` on SELinux hosts. Do not remove that without relabelling the files.

### Grafana will not start: `failure parsing contact points ... unknown integration type`

Grafana's contact-point type for an external Prometheus Alertmanager is
`prometheus-alertmanager`, not `alertmanager`. The provisioning file in this repo
is correct for Grafana 12.4. A provisioning error intentionally fails startup.

### Grafana starts, but the Alertmanager datasource health is 500

Expected. Grafana's generic `/api/datasources/uid/alertmanager/health` has no
health route for external Alertmanager. Use the proxy
`/api/alertmanager/alertmanager/api/v2/status` or the Alerting UI.

### No application metrics in Prometheus

1. Is the collector up? `curl -s localhost:13133/health/status`.
2. Does the collector expose metrics?
   `curl -s localhost:8889/metrics | grep http_server_request_duration` (the
   endpoint can be empty until the first batch; export interval is 10s).
3. Is the Prometheus target up?
   `curl -s localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.labels.job=="otel-collector")'`.
4. Are the apps exporting?
   `podman compose ... logs backend | grep -i opentelemetry`.
5. Query by the actual label: `{service_name="runcodes-backend"}`. If the metric
   exists with a different service label, inspect
   `/api/v1/label/__name__/values` and the collector resource configuration.
6. Check `otelcol_exporter_send_failed_metric_points_total` and
   `otelcol_exporter_queue_size{data_type="metrics"}`.

### No logs in Loki

1. Does the app log to stdout?
2. Does the collector receive logs?
   `otelcol_receiver_accepted_log_records_total{receiver="otlp"}` on
   `localhost:8888/metrics`.
3. Is Loki ready? `curl -s localhost:3100/ready` returns `ready` after the
   15-second ingester delay.
4. Query labels: `curl -sG localhost:3100/loki/api/v1/labels | jq`. If
   `service_name` is missing, the OTLP resource mapping changed.
5. Look for `loki_discarded_samples_total` reasons and structured-metadata
   limits; the collector's `otlphttp/loki` errors are in its logs.

### No traces in Tempo

1. `curl -s localhost:3200/ready` must return `ready`.
2. Is the sampling decision positive? `always_off` exports nothing.
3. Does the response carry a `TraceId`? If yes, tracing is active in the app even
   if export fails.
4. Check `otelcol_exporter_send_failed_spans_total` and
   `otelcol_exporter_queue_size{data_type="traces"}`.
5. Check Tempo ingestion:
   `curl -s localhost:3200/metrics | grep -E 'tempo_(distributor|receiver).*span'`.
6. Recreate the collector after recreating Tempo: the gRPC client caches DNS and
   can keep dialling a stale container IP.

### No profiles in Pyroscope

1. `curl -s http://localhost:4040/ready` is 200 and
   `curl -s http://localhost:12345/-/healthy` reports all components healthy.
2. Check the pprof listener from the Compose network:
   `podman run --rm --network runcodes-react_runcodes-network curlimages/curl -s -o /dev/null -w '%{http_code}\n' http://backend:6060/debug/pprof/`
   (use `docker run` with Docker Compose).
   A 404/connection refused means `PPROF_ADDR` is empty or the binary predates
   the listener.
3. `curl -s http://localhost:12345/api/v0/web/components` shows
   `pyroscope.scrape.runcodes` and `pyroscope.write.default`; both must be
   healthy.
4. Check `pyroscope_write_sent_profiles_total` in Alloy and
   `pyroscope_distributor_profiles_received_total` in Pyroscope. If Alloy sends
   but Pyroscope rejects, inspect its logs.
5. A trace-to-profile flame graph needs `pyroscope.profile.id` on the span.
   Profiles are sampled every 15s, so a short span can have a valid id but no
   samples.
6. Right after a Pyroscope restart, `no segment writer instances available` is
   transient; Alloy retries.

### Browser traces or the frontend OTLP request fail

1. The SPA traces only when built with `VITE_OTEL_ENABLED=true` and a
   browser-reachable `VITE_OTEL_EXPORTER_OTLP_ENDPOINT`. Rebuild after changing
   them — Vite bakes them in.
2. Check the browser console for CORS or mixed-content errors. The collector
   allows `http://localhost:8080` and `http://localhost:5173` by default; add
   your real origin in `observability/otel-collector/config.yaml`.
3. The endpoint must be reachable from the **browser**, not just the container.
4. The CORS preflight and a synthetic browser span are checked by the smoke
   test; for a real browser use `make observability-browser-smoke-test`.
5. The Playwright test asserts a `traceparent` and both service names in the same
   trace. If it fails earlier, check the browser console and collector CORS.

### The judge queue is growing

- `runcodes_judge_queue_depth` above zero with a rising
  `runcodes_judge_queue_oldest_age_seconds` means workers are not draining.
- Check judge readiness (`curl -s localhost:9000/readyz`), Podman availability,
  image pulls and judge logs. `RunCodesJudgeQueueStalled` fires after five
  minutes with a queued commit older than five minutes.
- If the metric is missing, check the judge image, `OTEL_EXPORTER_OTLP_ENDPOINT`
  and the collector target.

### Trace/log correlation links do not appear

- The log record must carry `trace_id` as **structured metadata**. Loki's OTLP
  ingestion does that; the Loki datasource derived field must use
  `matcherType: label` and `matcherRegex: trace_id` (configured).
- The app must log with a context containing the span. Request logs and
  context-aware `slog` calls do; plain `slog.Info` in background code with no
  span cannot be correlated.
- Tempo's trace-to-logs query uses `{service_name="..."} | trace_id="..."`.
- Grafana 12.4 is required for the `matcherType: label` derived field.

### Alerts fire in Prometheus but never reach a receiver

1. `curl -s localhost:9093/-/ready`.
2. `curl -s localhost:9093/api/v2/alerts | jq` shows active alerts and
   `status.inhibitedBy`.
3. Check the route/receiver and Alertmanager logs.
4. If `ALERTMANAGER_CONFIG_FILE` points at the Gmail copy, check the `smtp_*`
   settings and `/etc/alertmanager/secrets/gmail_app_password`. A
   `535 5.7.8 Username and Password not accepted` means a wrong app password,
   spaces left in it, or 2-Step Verification disabled.
5. Otherwise test the sink directly: `curl -s localhost:8085/healthz`.
6. A 4xx from a webhook receiver usually means a bad URL, token or payload.

### Host disk or memory pressure

- `make observability-status` / `podman stats` (or `docker stats`) shows which
  container grows.
- Prometheus keeps 15d/8GB; Loki/Tempo/Pyroscope 7d. They fill volumes under
  high traffic or raised retention.
- The dashboard's resource row and `RunCodesHostDiskSpaceLow` give early warning.
- To reclaim local data, stop the stack and run `make observability-destroy`
  (deletes telemetry history). Reduce retention first if you want to keep data.
- Memory limits are `mem_limit` values in the overlay; the collector's memory
  limiter is 80% of 512 MiB with a 25% spike allowance. Raise both together.

### Ports are already in use

Set `OBSERVABILITY_BIND_ADDRESS` and the per-service `*_PORT` variables, then
update the smoke-test URLs. Binding observability to `0.0.0.0` exposes
unauthenticated services; only do it behind an authenticating proxy.

### The judge is ready but submissions do not run

This is not an observability failure. The backend's `/readyz` reports
`judge: unavailable` without failing readiness. Check the judge and the rootless
Podman socket. Traces for attempted submissions stop at the backend's outbound
judge span when the judge is down.

## Testing alerts

### Inject an alert through the Alertmanager API

```bash
curl -s -X POST http://localhost:9093/api/v2/alerts \
  -H 'Content-Type: application/json' \
  -d '[{
    "labels": {"alertname": "RunbookTest", "severity": "critical", "service_name": "runcodes-backend"},
    "annotations": {"summary": "runbook test", "description": "manual injection"},
    "startsAt": "'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'"
  }]'
curl -s http://localhost:8085/alerts | jq '.payloads[0].payload.status'
```

The alert is delivered to `alert-sink` after the group wait. It stays active
until resolved; send the same labels with an `endsAt` in the past, or use the
Alertmanager UI.

### Trigger a real rule

The simplest is `RunCodesObservabilityTargetDown` (stop one target) or
`RunCodesHTTP5xxRateHigh`. To avoid breaking the app, use Prometheus's expression
UI or a temporary rule, then reload:

```bash
curl -s -X POST http://localhost:9090/-/reload
```

Validate before reloading:

```bash
make observability-validate
```

Grafana-managed example rules are visible under **Alerting → Alert rules** and
route through the Alertmanager contact point in
`observability/grafana/provisioning/alerting/alerting.yaml`. The Prometheus rules
are the production path.

### Silences

Use the Alertmanager UI (<http://localhost:9093>) or the API:

```bash
curl -s -X POST http://localhost:9093/api/v2/silences \
  -H 'Content-Type: application/json' \
  -d '{"matchers":[{"name":"alertname","value":"RunbookTest","isRegex":false}],
       "startsAt":"'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'",
       "endsAt":"'"$(date -u -d '+1 hour' +%Y-%m-%dT%H:%M:%SZ)"'",
       "createdBy":"runbook","comment":"testing"}'
```

## Adding a service to observability

The collector already accepts OTLP from anything on the Compose network, so a new
service usually needs no collector change.

1. **Instrument it** with an OpenTelemetry SDK or auto-instrumentation. Set
   `service.name` (stable, low-cardinality), `service.namespace=runcodes`,
   `service.version` and `deployment.environment.name`.
2. **Point it at the collector**:
   `OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318`,
   `OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf`.
3. **Propagate context** with W3C `traceparent`/`tracestate`. In Go use
   `otelhttp`/`otelchi` and wrap the database driver with `otelsql`.
4. **Log with trace context**: use `slog` context methods; the backend telemetry
   package adds `trace_id`/`span_id` and the OTel log SDK attaches the native
   trace context.
5. **If it only exposes Prometheus metrics**, add a job to
   `observability/prometheus/prometheus.yml` and a scrape target. Prefer OTLP
   for new services.
6. **Make it visible**: the dashboard's `service` variable is populated from
   `http_server_request_duration_seconds_count{service_name=...}`, so a new
   service appears automatically once it emits HTTP metrics.
7. **Alert on it**: add rules with a `severity` label and a runbook link.
8. **Document it** here and in the architecture diagram.

Example Compose environment:

```yaml
environment:
  OTEL_EXPORTER_OTLP_ENDPOINT: http://otel-collector:4318
  OTEL_EXPORTER_OTLP_PROTOCOL: http/protobuf
  OTEL_SERVICE_NAME: runcodes-new-service
  DEPLOYMENT_ENVIRONMENT: development
  OTEL_TRACES_SAMPLER: parentbased_traceidratio
  OTEL_TRACES_SAMPLER_ARG: "0.1"
```

## Adding a service to continuous profiling

1. Expose `net/http/pprof` on a dedicated internal listener (never the public API
   listener), as the backend and judge do through `PPROF_ADDR`.
2. Add a `pyroscope.scrape` target in `observability/alloy/config.alloy` with a
   stable `service_name` label, on the internal network.
3. Add it to the `RunCodesProfilesNotArriving` expectation.
4. For span-level flame graphs, add the language-specific `pyroscope.profile.id`
   bridge. For Go, wrap the tracer provider with `otel-profiling-go`.
5. Select the Pyroscope datasource in Tempo's **Traces to profiles** settings.

Never expose pprof publicly: it can leak memory contents and is unauthenticated.

## Upgrading the stack

1. Change image tags in `docker-compose.observability.yml` and, for validation
   images, `scripts/observability/validate.sh`.
2. `make observability-validate`.
3. `make observability-up`, then `make observability-smoke-test`.
4. Check datasource health, the dashboard, and `up{job=~"..."}` for every target.
5. If a component fails, revert the tag and recreate it. Data volumes are not
   migrated automatically; check release notes before downgrading.

## Retention and storage

- Prometheus: `PROMETHEUS_RETENTION_TIME` / `_SIZE` (15d/8GB).
- Loki: configured in `observability/loki/loki.yaml` (7d).
- Tempo: 7d block retention.
- Pyroscope: `limits.retention_period: 168h` on the local filesystem.

For production, set `PYROSCOPE_CONFIG_FILE` to a secret-mounted config based on
`observability/pyroscope/config.s3.example.yaml`, point it at object storage, and
set the bucket lifecycle to match retention. The example is not mounted by
default because it needs real credentials.

## Configuring Gmail notifications

The default local receiver is the in-stack `alert-sink` webhook. To email
alerts through Gmail instead, use the Gmail example config. Gmail SMTP requires
a Google app password, which requires 2-Step Verification on the sending
account; the ordinary account password is rejected.

1. Create an app password at **Google Account -> Security -> 2-Step
   Verification -> App passwords**.
2. Copy the example and set the sending account and recipients. It routes the
   smoke-test alert to `alert-sink` and everything else to Gmail:

   ```bash
   cp observability/alertmanager/alertmanager.gmail.example.yml \
      .observability/alertmanager.yml
   $EDITOR .observability/alertmanager.yml   # smtp_from, smtp_auth_username, to
   # Rootless Alertmanager reads it as a mapped subuid; keep it world-readable.
   chmod 0644 .observability/alertmanager.yml
   ```

   Set `smtp_from` and `smtp_auth_username` to the same plain address
   (`alerts@gmail.com`, not `RunCodes <alerts@gmail.com>`); that account is the
   SMTP user. The `to` field is a comma-separated string. For a Google Workspace
   SMTP relay, change `smtp_smarthost` in the copy.
3. Write the 16-character app password, without the spaces Google displays:

   ```bash
   printf '%s\n' 'abcdefghijklmnop' > .observability/alertmanager_gmail_app_password
   ```

4. Point the stack at the copy. Add this to `.env` (or export it for one run):

   ```bash
   ALERTMANAGER_CONFIG_FILE=./.observability/alertmanager.yml
   ```

5. Recreate Alertmanager so it reads the new config and secret:

   ```bash
   podman compose -f docker-compose.yml -f docker-compose.observability.yml \
     up -d --force-recreate alertmanager
   ```

6. Send a test alert and check the log:

   ```bash
   curl -s -X POST http://localhost:9093/api/v2/alerts \
     -H 'Content-Type: application/json' \
     -d '[{"labels":{"alertname":"GmailTest","severity":"warning","service_name":"runcodes-backend"},"annotations":{"summary":"Gmail test"}}]'
   podman compose -f docker-compose.yml -f docker-compose.observability.yml \
     logs alertmanager
   ```

   The smoke test (`make observability-smoke-test`) still passes because
   `ObservabilitySmokeTest` is routed to `alert-sink`.

A Google Workspace domain can disable app passwords. If yours does, use a
Workspace SMTP relay in the copied config instead. The app password is read
from the mounted file for each notification, so rotating it means replacing
`.observability/alertmanager_gmail_app_password` and recreating Alertmanager;
the YAML never contains the password.

## Rotating the Alertmanager webhook URL

The receiver URL is read from `.observability/alertmanager_webhook_url` (mounted
at `/etc/alertmanager/secrets/webhook_url`). Replace its contents and recreate
Alertmanager:

```bash
printf '%s\n' 'https://alerts.example.invalid/webhook' \
  > .observability/alertmanager_webhook_url
podman compose -f docker-compose.yml -f docker-compose.observability.yml \
  up -d --force-recreate alertmanager
```

Never commit the real URL. The production template
`observability/alertmanager/alertmanager.production.example.yml` shows Gmail,
Slack and PagerDuty receivers with `*_file` settings.

## Rotating the Grafana admin password

```bash
rm .observability/grafana_admin_password
make observability-init
podman compose -f docker-compose.yml -f docker-compose.observability.yml \
  up -d --force-recreate grafana
```

Production should use a secret manager and
`GF_SECURITY_ADMIN_PASSWORD__FILE`.

## First-five-minutes triage

1. `make observability-status` — which container is unhealthy?
2. `curl -s localhost:13133/health/status` — is the collector up?
3. `curl -s localhost:9090/-/ready`, `3100/ready`, `3200/ready`, `9093/-/ready`,
   `3000/api/health` — which backend is down?
4. `podman compose ... logs --tail=100 <service>` — the first error, not the last
   restart-loop message.
5. Prometheus `/targets` — is the failing component a scrape target?
6. Tempo `/api/search` and Loki `query_range` with a known recent trace id — is
   the data path or only the UI broken?
7. Alertmanager `/api/v2/alerts` — are alerts firing and inhibited?
8. Pyroscope `4040/ready` and Alloy `12345/-/healthy` — are profiles arriving?
   Check the judge queue metric for a backlog.
9. If disk or memory is the cause, stop the stack and use
   `make observability-destroy` only after deciding what history you can lose.

## Useful queries

PromQL:

```promql
# Request rate by service
sum by (service_name) (rate(http_server_request_duration_seconds_count[$__rate_interval]))

# 5xx ratio
sum by (service_name) (rate(http_server_request_duration_seconds_count{http_response_status_code=~"5.."}[5m]))
/
clamp_min(sum by (service_name) (rate(http_server_request_duration_seconds_count[5m])), 0.001)

# p95 latency
histogram_quantile(0.95, sum by (le, service_name) (rate(http_server_request_duration_seconds_bucket[5m])))

# Trace-derived error ratio
sum by (service_name) (rate(traces_spanmetrics_calls_total{status_code="STATUS_CODE_ERROR"}[5m]))
/
clamp_min(sum by (service_name) (rate(traces_spanmetrics_calls_total[5m])), 0.001)

# Collector backpressure
otelcol_exporter_queue_size / otelcol_exporter_queue_capacity

# Judge queue
runcodes_judge_queue_depth
runcodes_judge_queue_oldest_age_seconds

# Profiles
sum(rate(pyroscope_distributor_profiles_received_total[10m]))
```

LogQL:

```logql
{service_name="runcodes-backend"} |= "error"
{service_name=~"runcodes-.*"} | detected_level=~"error|warn"
sum by (service_name) (count_over_time({service_name=~"runcodes-.*"}[5m]))
{service_name="runcodes-backend"} | trace_id="<32-hex-trace-id>"
```

TraceQL:

```traceql
{ resource.service.name = "runcodes-backend" && duration > 1s }
{ resource.service.name = "runcodes-backend" && status = error }
{ span.http.route = "/api/v1/submissions" }
```

## Related pages

- [Local observability stack](./local.md)
- [Four-machine observability](./production.md)
- [Ansible operations](../deployment/ansible-operations.md)
