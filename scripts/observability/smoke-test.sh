#!/usr/bin/env bash
#
# End-to-end smoke test for the RunCodes observability stack.
#
# It verifies, in order:
#   1. every service is reachable (or starts the stack with SMOKE_START=1);
#   2. traffic produces a TraceId header and spans/metrics/logs;
#   3. Prometheus has application metrics;
#   4. Loki returns the application logs (and their trace_id metadata);
#   5. Tempo has the exact trace id returned by the API;
#   6. Grafana datasources are healthy and the provisioned dashboard exists;
#   7. Alertmanager accepts an alert and delivers it to the webhook sink.
#
# Usage:
#   ./scripts/observability/smoke-test.sh
#   SMOKE_START=0 ./scripts/observability/smoke-test.sh   # stack already running
#
# Environment overrides: BACKEND_URL, FRONTEND_URL, JUDGE_URL, PROMETHEUS_URL,
# LOKI_URL, TEMPO_URL, GRAFANA_URL, ALERTMANAGER_URL, ALERT_SINK_URL,
# GRAFANA_ADMIN_USER, GRAFANA_ADMIN_PASSWORD, SMOKE_START, SMOKE_TIMEOUT,
# SMOKE_BUILD.
set -Eeuo pipefail

ROOT="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

BACKEND_URL="${BACKEND_URL:-http://127.0.0.1:8443}"
FRONTEND_URL="${FRONTEND_URL:-http://127.0.0.1:8080}"
JUDGE_URL="${JUDGE_URL:-http://127.0.0.1:9000}"
PROMETHEUS_URL="${PROMETHEUS_URL:-http://127.0.0.1:9090}"
LOKI_URL="${LOKI_URL:-http://127.0.0.1:3100}"
TEMPO_URL="${TEMPO_URL:-http://127.0.0.1:3200}"
GRAFANA_URL="${GRAFANA_URL:-http://127.0.0.1:3000}"
ALERTMANAGER_URL="${ALERTMANAGER_URL:-http://127.0.0.1:9093}"
ALERT_SINK_URL="${ALERT_SINK_URL:-http://127.0.0.1:8085}"
COLLECTOR_HEALTH_URL="${COLLECTOR_HEALTH_URL:-http://127.0.0.1:13133/health/status}"
COLLECTOR_HTTP_URL="${COLLECTOR_HTTP_URL:-http://127.0.0.1:4318}"
PYROSCOPE_URL="${PYROSCOPE_URL:-http://127.0.0.1:4040}"
ALLOY_URL="${ALLOY_URL:-http://127.0.0.1:12345}"
BROWSER_ORIGIN="${BROWSER_ORIGIN:-http://localhost:8080}"

GRAFANA_ADMIN_USER="${GRAFANA_ADMIN_USER:-admin}"
GRAFANA_ADMIN_PASSWORD="${GRAFANA_ADMIN_PASSWORD:-}"
if [[ -z "$GRAFANA_ADMIN_PASSWORD" && -r "$ROOT/.observability/grafana_admin_password" ]]; then
  GRAFANA_ADMIN_PASSWORD="$(<"$ROOT/.observability/grafana_admin_password")"
fi

SMOKE_START="${SMOKE_START:-1}"
SMOKE_BUILD="${SMOKE_BUILD:-1}"
SMOKE_TIMEOUT="${SMOKE_TIMEOUT:-180}"

COMPOSE=(docker compose -f docker-compose.yml -f docker-compose.observability.yml)
SMOKE_ID="smoke-$(date +%s)-${RANDOM}"
START_EPOCH="$(date +%s)"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/runcodes-smoke.XXXXXX")"
DIAGNOSING=0

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

pass() { printf '\033[0;32mPASS\033[0m %s\n' "$*"; }
info() { printf '\033[0;36mINFO\033[0m %s\n' "$*"; }
warn() { printf '\033[0;33mWARN\033[0m %s\n' "$*"; }
fail() {
  printf '\033[0;31mFAIL\033[0m %s\n' "$*" >&2
  # Explicit assertions bypass the ERR trap; collect the same diagnostics here
  # so a failed check always says which container/endpoint is unhealthy.
  diagnostics >&2 || true
  exit 1
}

compose() {
  (cd "$ROOT" && "${COMPOSE[@]}" "$@")
}

http_code() {
  curl -sS -o /dev/null -w '%{http_code}' --max-time 5 "$@" 2>/dev/null || true
}

is_2xx_or_3xx() {
  [[ "$1" =~ ^2[0-9][0-9]$ || "$1" =~ ^3[0-9][0-9]$ ]]
}

wait_http() {
  local name="$1" url="$2" timeout="${3:-$SMOKE_TIMEOUT}"
  local deadline=$((SECONDS + timeout)) code
  while (( SECONDS < deadline )); do
    code="$(http_code "$url")"
    if is_2xx_or_3xx "$code"; then
      pass "$name is reachable ($url)"
      return 0
    fi
    sleep 2
  done
  fail "$name did not become reachable at $url within ${timeout}s (last HTTP status: ${code:-none})"
}

wait_prom_positive() {
  local name="$1" query="$2" timeout="${3:-90}" deadline value
  deadline=$((SECONDS + timeout))
  while (( SECONDS < deadline )); do
    value="$(curl -fsS -G "$PROMETHEUS_URL/api/v1/query" \
      --data-urlencode "query=$query" 2>/dev/null \
      | jq -r '.data.result[0].value[1] // empty' 2>/dev/null || true)"
    if [[ -n "$value" ]] && awk -v v="$value" 'BEGIN { exit !(v > 0) }'; then
      pass "$name (value: $value)"
      return 0
    fi
    sleep 3
  done
  fail "Prometheus has no positive value for \"$name\" (query: $query) within ${timeout}s"
}

wait_prom_exists() {
  local name="$1" query="$2" timeout="${3:-90}" deadline
  deadline=$((SECONDS + timeout))
  while (( SECONDS < deadline )); do
    if curl -fsS -G "$PROMETHEUS_URL/api/v1/query" \
      --data-urlencode "query=$query" 2>/dev/null \
      | jq -e '.data.result | length > 0' >/dev/null 2>&1; then
      pass "$name"
      return 0
    fi
    sleep 3
  done
  fail "Prometheus has no series for \"$name\" (query: $query) within ${timeout}s"
}

wait_loki_logs() {
  local name="$1" query="$2" timeout="${3:-90}" deadline
  local start end count
  deadline=$((SECONDS + timeout))
  start=$(( $(date +%s) - 1800 ))
  while (( SECONDS < deadline )); do
    end="$(date +%s)"
    count="$(curl -fsS -G "$LOKI_URL/loki/api/v1/query_range" \
      --data-urlencode "query=$query" \
      --data-urlencode "start=$start" \
      --data-urlencode "end=$end" \
      --data-urlencode "limit=50" 2>/dev/null \
      | jq -r '[.data.result[]?.values[]?] | length' 2>/dev/null || true)"
    if [[ -n "$count" ]] && (( count > 0 )); then
      pass "$name ($count log line(s))"
      return 0
    fi
    sleep 3
  done
  fail "Loki returned no logs for \"$name\" (query: $query) within ${timeout}s"
}

wait_grafana_datasource() {
  local uid="$1" name="$2" timeout="${3:-$SMOKE_TIMEOUT}"
  local deadline=$((SECONDS + timeout)) status
  while (( SECONDS < deadline )); do
    status="$(curl -sS -u "$GRAFANA_ADMIN_USER:$GRAFANA_ADMIN_PASSWORD" \
      "$GRAFANA_URL/api/datasources/uid/$uid/health" 2>/dev/null \
      | jq -r '.status // empty' 2>/dev/null || true)"
    if [[ "$status" == "OK" ]]; then
      pass "Grafana datasource $name is healthy"
      return 0
    fi
    sleep 2
  done
  fail "Grafana datasource $name ($uid) is not healthy (last status: ${status:-none})"
}

diagnostics() {
  [[ "$DIAGNOSING" == "1" ]] && return 0
  DIAGNOSING=1
  set +e
  echo
  echo "==================== diagnostics ===================="
  echo "Compose status:"
  compose ps 2>&1 | tail -60
  for service in backend judge otel-collector prometheus loki tempo grafana alertmanager alert-sink; do
    echo
    echo "--- last 40 log lines: $service ---"
    compose logs --no-color --tail=40 "$service" 2>&1 | tail -45
  done
  echo
  echo "Last responses (TMP_DIR=$TMP_DIR):"
  for file in "$TMP_DIR"/*.json; do
    [[ -e "$file" ]] || continue
    echo "--- $file ---"
    head -c 2000 "$file" 2>/dev/null
    echo
  done
  echo "====================================================="
  set -e
}

on_error() {
  local status=$?
  echo >&2
  echo "Smoke test failed with exit code $status." >&2
  diagnostics >&2
  exit "$status"
}
trap on_error ERR

for command in curl jq; do
  command -v "$command" >/dev/null 2>&1 || fail "required command '$command' is not installed"
done
[[ -n "$GRAFANA_ADMIN_PASSWORD" ]] || fail "Grafana admin password not found; run scripts/observability/init-secrets.sh or set GRAFANA_ADMIN_PASSWORD"

info "smoke id: $SMOKE_ID"

# ---------------------------------------------------------------------------
# 1. Reachability / optional start
# ---------------------------------------------------------------------------
if ! is_2xx_or_3xx "$(http_code "$BACKEND_URL/healthz")"; then
  if [[ "$SMOKE_START" != "1" ]]; then
    fail "the stack is not running at $BACKEND_URL and SMOKE_START=$SMOKE_START"
  fi
  info "stack is not running; starting it (this builds the backend/judge images)"
  if [[ "$SMOKE_BUILD" == "1" ]]; then
    compose up -d --build
  else
    compose up -d
  fi
fi

wait_http "backend liveness" "$BACKEND_URL/healthz"
wait_http "judge liveness" "$JUDGE_URL/healthz"
wait_http "frontend" "$FRONTEND_URL/" 60
wait_http "OpenTelemetry Collector" "$COLLECTOR_HEALTH_URL" 90
wait_http "Prometheus" "$PROMETHEUS_URL/-/ready"
wait_http "Loki" "$LOKI_URL/ready"
wait_http "Tempo" "$TEMPO_URL/ready"
wait_http "Alertmanager" "$ALERTMANAGER_URL/-/ready"
wait_http "Grafana" "$GRAFANA_URL/api/health"
wait_http "Pyroscope" "$PYROSCOPE_URL/ready" 90
wait_http "Alloy" "$ALLOY_URL/-/ready" 60
wait_http "alert sink" "$ALERT_SINK_URL/healthz" 60

backend_ready="$(http_code "$BACKEND_URL/readyz")"
if [[ "$backend_ready" != "200" ]]; then
  fail "backend /readyz returned $backend_ready (database is expected to be ready; runbook has diagnostics)"
fi
pass "backend readiness (database reachable)"
judge_ready="$(http_code "$JUDGE_URL/readyz")"
if [[ "$judge_ready" == "200" ]]; then
  pass "judge readiness (database and podman reachable)"
else
  warn "judge /readyz returned $judge_ready; traces and logs still work, but submissions cannot run"
fi

# ---------------------------------------------------------------------------
# 2. Generate traffic, including a request whose trace id we can follow
# ---------------------------------------------------------------------------
info "generating traffic ($SMOKE_ID)"
for i in {1..8}; do
  curl -sS -o /dev/null --max-time 5 "$BACKEND_URL/healthz" || true
  curl -sS -o /dev/null --max-time 5 "$BACKEND_URL/api/v1/settings/public?smoke=$SMOKE_ID&i=$i" || true
  curl -sS -o /dev/null --max-time 5 "$BACKEND_URL/readyz" || true
  curl -sS -o /dev/null --max-time 5 -X POST "$BACKEND_URL/api/v1/user/login" \
    -H 'Content-Type: application/json' \
    --data "{\"email\":\"$SMOKE_ID@example.com\",\"password\":\"not-the-password\"}" || true
  curl -sS -o /dev/null --max-time 5 "$BACKEND_URL/api/v1/user/profile" || true
  curl -sS -o /dev/null --max-time 5 "$JUDGE_URL/healthz" || true
  curl -sS -o /dev/null --max-time 5 "$FRONTEND_URL/" || true
done
pass "sent smoke traffic"

trace_headers="$TMP_DIR/trace-headers.txt"
curl -fsS -D "$trace_headers" -o /dev/null --max-time 5 \
  "$BACKEND_URL/api/v1/settings/public?smoke=$SMOKE_ID&trace=1"
TRACE_ID="$(awk 'tolower($1) == "traceid:" { gsub("\r", ""); print $2 }' "$trace_headers" | tail -n 1)"
if [[ ! "$TRACE_ID" =~ ^[0-9a-f]{32}$ ]]; then
  fail "backend did not return a 32-hex TraceId header (got '${TRACE_ID:-empty}')"
fi
pass "backend returned TraceId $TRACE_ID"

# ---------------------------------------------------------------------------
# 3. Prometheus
# ---------------------------------------------------------------------------
wait_prom_positive "backend HTTP metrics in Prometheus" \
  'sum(http_server_request_duration_seconds_count{service_name="runcodes-backend"})'
wait_prom_positive "judge HTTP metrics in Prometheus" \
  'sum(http_server_request_duration_seconds_count{service_name="runcodes-judge"})'
wait_prom_positive "OpenTelemetry Collector target is up" \
  'up{job="otel-collector"}'
wait_prom_exists "judge queue depth metric" 'runcodes_judge_queue_depth'
wait_prom_positive "profiles received by Pyroscope" \
  'sum(pyroscope_distributor_profiles_received_total)'
wait_prom_positive "Pyroscope and Alloy targets are up" \
  'sum(up{job=~"pyroscope|alloy"})'

# ---------------------------------------------------------------------------
# 4. Loki
# ---------------------------------------------------------------------------
wait_loki_logs "backend logs in Loki" \
  "{service_name=\"runcodes-backend\"} |= \"$SMOKE_ID\""
wait_loki_logs "judge logs in Loki" \
  '{service_name="runcodes-judge"}'
wait_loki_logs "trace-correlated log line in Loki" \
  "{service_name=\"runcodes-backend\"} | trace_id=\"$TRACE_ID\""

# ---------------------------------------------------------------------------
# 5. Tempo: the exact trace id that the API returned in the response header
# ---------------------------------------------------------------------------
wait_http "trace $TRACE_ID in Tempo" "$TEMPO_URL/api/traces/$TRACE_ID" "$SMOKE_TIMEOUT"

# ---------------------------------------------------------------------------
# 5b. Browser OTLP path: CORS preflight + a browser-origin trace into Tempo
# ---------------------------------------------------------------------------
cors_headers="$TMP_DIR/cors-headers.txt"
cors_status="$(curl -sS -D "$cors_headers" -o /dev/null -w '%{http_code}' --max-time 5 \
  -X OPTIONS "$COLLECTOR_HTTP_URL/v1/traces" \
  -H "Origin: $BROWSER_ORIGIN" \
  -H 'Access-Control-Request-Method: POST' \
  -H 'Access-Control-Request-Headers: content-type' 2>/dev/null || true)"
if [[ "$cors_status" != "204" && "$cors_status" != "200" ]]; then
  fail "collector OTLP HTTP CORS preflight returned ${cors_status:-none} for $BROWSER_ORIGIN"
fi
if ! grep -qi "^Access-Control-Allow-Origin: $BROWSER_ORIGIN" "$cors_headers"; then
  fail "collector CORS did not allow origin $BROWSER_ORIGIN"
fi
pass "OpenTelemetry Collector allows browser OTLP origin $BROWSER_ORIGIN"

browser_trace_id="$(head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n')"
browser_span_id="$(head -c 8 /dev/urandom | od -An -tx1 | tr -d ' \n')"
browser_start_ns="$(date +%s)000000000"
browser_end_ns="$(( $(date +%s) + 1 ))000000000"
browser_payload="$(jq -nc \
  --arg trace "$browser_trace_id" --arg span "$browser_span_id" \
  --arg start "$browser_start_ns" --arg end "$browser_end_ns" \
  --arg id "$SMOKE_ID" \
  '{resourceSpans:[{resource:{attributes:[
      {key:"service.name",value:{stringValue:"runcodes-frontend"}},
      {key:"deployment.environment.name",value:{stringValue:"development"}}]},
    scopeSpans:[{scope:{name:"runcodes-smoke"},spans:[
      {traceId:$trace,spanId:$span,name:"browser.fetch",kind:3,
       startTimeUnixNano:$start,endTimeUnixNano:$end,
       attributes:[
         {key:"http.request.method",value:{stringValue:"GET"}},
         {key:"url.full",value:{stringValue:"http://localhost:8080/api/v1/settings/public"}},
         {key:"runcodes.smoke_id",value:{stringValue:$id}}]}]}]}]}')"

browser_ingest_status="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 5 \
  -X POST "$COLLECTOR_HTTP_URL/v1/traces" \
  -H "Origin: $BROWSER_ORIGIN" -H 'Content-Type: application/json' \
  --data-binary "$browser_payload" 2>/dev/null || true)"
if [[ "$browser_ingest_status" != "200" ]]; then
  fail "collector rejected the browser-origin OTLP span (HTTP ${browser_ingest_status:-none})"
fi
pass "browser-origin OTLP span was accepted by the collector"
wait_http "browser trace in Tempo" "$TEMPO_URL/api/traces/$browser_trace_id" "$SMOKE_TIMEOUT"

# ---------------------------------------------------------------------------
# 6. Grafana
# ---------------------------------------------------------------------------
wait_grafana_datasource prometheus "Prometheus"
wait_grafana_datasource loki "Loki"
wait_grafana_datasource tempo "Tempo"
wait_grafana_datasource pyroscope "Pyroscope"

# Grafana's generic datasource health endpoint has no health route for the
# external Alertmanager datasource ("plugin unavailable"), so exercise its proxy
# API instead. This is the same path Grafana's Alerting UI uses.
alertmanager_proxy_deadline=$((SECONDS + SMOKE_TIMEOUT))
until [[ "$(http_code -u "$GRAFANA_ADMIN_USER:$GRAFANA_ADMIN_PASSWORD" \
  "$GRAFANA_URL/api/alertmanager/alertmanager/api/v2/status")" == "200" ]]; do
  (( SECONDS < alertmanager_proxy_deadline )) \
    || fail "Grafana cannot proxy to the Alertmanager datasource"
  sleep 2
done
pass "Grafana Alertmanager datasource proxy is working"

dashboard_title="$(curl -fsS -u "$GRAFANA_ADMIN_USER:$GRAFANA_ADMIN_PASSWORD" \
  "$GRAFANA_URL/api/dashboards/uid/runcodes-overview" \
  | jq -r '.dashboard.title // empty')"
if [[ "$dashboard_title" != "RunCodes — Observability Overview" ]]; then
  fail "provisioned dashboard runcodes-overview not found (title: '${dashboard_title:-none}')"
fi
pass "Grafana dashboard is provisioned: $dashboard_title"

alert_rule_count="$(curl -fsS -u "$GRAFANA_ADMIN_USER:$GRAFANA_ADMIN_PASSWORD" \
  "$GRAFANA_URL/api/v1/provisioning/alert-rules" | jq 'length' 2>/dev/null || echo 0)"
if (( alert_rule_count > 0 )); then
  pass "Grafana has $alert_rule_count provisioned alert rule(s)"
else
  warn "Grafana reports no provisioned alert rules; check the alerting provisioning logs"
fi

contact_type="$(curl -fsS -u "$GRAFANA_ADMIN_USER:$GRAFANA_ADMIN_PASSWORD" \
  "$GRAFANA_URL/api/v1/provisioning/contact-points" \
  | jq -r '.[] | select(.name == "Alertmanager") | .type' 2>/dev/null | head -n1)"
if [[ "$contact_type" == "prometheus-alertmanager" ]]; then
  pass "Grafana has an Alertmanager contact point"
else
  fail "Grafana's Alertmanager contact point is missing or has type '${contact_type:-none}'"
fi

# ---------------------------------------------------------------------------
# 7. Alertmanager -> alert sink
# ---------------------------------------------------------------------------
curl -fsS -X DELETE "$ALERT_SINK_URL/alerts" >/dev/null || true

starts_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
alert_payload="$(jq -nc \
  --arg id "$SMOKE_ID" \
  --arg starts "$starts_at" \
  '[{
    "labels": {
      "alertname": "ObservabilitySmokeTest",
      "severity": "warning",
      "service_name": "runcodes-backend",
      "smoke_id": $id
    },
    "annotations": {
      "summary": "End-to-end observability smoke test",
      "description": "Injected by scripts/observability/smoke-test.sh"
    },
    "startsAt": $starts
  }]')"

curl -fsS -X POST "$ALERTMANAGER_URL/api/v2/alerts" \
  -H 'Content-Type: application/json' -d "$alert_payload" >/dev/null
pass "Alertmanager accepted the test alert"

alertmanager_deadline=$((SECONDS + 60))
until curl -fsS -G "$ALERTMANAGER_URL/api/v2/alerts" \
  --data-urlencode 'active=true' \
  --data-urlencode 'filter=alertname="ObservabilitySmokeTest"' 2>/dev/null \
  | jq -e --arg id "$SMOKE_ID" \
      '[.[] | select(.labels.smoke_id == $id)] | length > 0' >/dev/null; do
  (( SECONDS < alertmanager_deadline )) || fail "Alertmanager did not return the injected alert"
  sleep 2
done
pass "Alertmanager shows the injected alert"

sink_deadline=$((SECONDS + 60))
until curl -fsS "$ALERT_SINK_URL/alerts" 2>/dev/null \
  | jq -e --arg id "$SMOKE_ID" \
      '[.payloads[]?.payload.alerts[]? | select(.labels.smoke_id == $id)] | length > 0' >/dev/null; do
  (( SECONDS < sink_deadline )) || fail "Alertmanager did not deliver the webhook to the alert sink"
  sleep 2
done
pass "Alertmanager delivered the alert to the webhook sink"

# Resolve the test alert so it does not stay active between runs. The resolved
# webhook is best-effort. startsAt must be before endsAt; both are in the past,
# which tells Alertmanager to mark the alert resolved immediately.
resolved_starts_at="$(date -u -d '5 minutes ago' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || true)"
resolved_at="$(date -u -d '1 minute ago' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || true)"
if [[ -z "$resolved_starts_at" ]]; then
  resolved_starts_at="$(date -u -v-5M +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || true)"
fi
if [[ -z "$resolved_at" ]]; then
  resolved_at="$(date -u -v-1M +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || true)"
fi
if [[ -n "$resolved_starts_at" && -n "$resolved_at" ]]; then
  jq -nc --arg id "$SMOKE_ID" --arg starts "$resolved_starts_at" --arg ends "$resolved_at" \
    '[{
      "labels": {"alertname": "ObservabilitySmokeTest", "severity": "warning", "service_name": "runcodes-backend", "smoke_id": $id},
      "annotations": {"summary": "End-to-end observability smoke test"},
      "startsAt": $starts,
      "endsAt": $ends
    }]' | curl -fsS -X POST "$ALERTMANAGER_URL/api/v2/alerts" \
      -H 'Content-Type: application/json' --data-binary @- >/dev/null 2>&1 || true
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
echo "All observability smoke checks passed."
echo "  Grafana:      $GRAFANA_URL  (user: $GRAFANA_ADMIN_USER, password: .observability/grafana_admin_password)"
echo "  Prometheus:   $PROMETHEUS_URL"
echo "  Alertmanager: $ALERTMANAGER_URL"
echo "  Loki:         $LOKI_URL"
echo "  Tempo:        $TEMPO_URL"
echo "  Pyroscope:    $PYROSCOPE_URL"
echo "  Alloy:        $ALLOY_URL"
echo "  Trace:        $TRACE_ID"
echo "  Elapsed:      $(( $(date +%s) - START_EPOCH ))s"
echo
echo "Open the dashboard at $GRAFANA_URL/d/runcodes-overview"
