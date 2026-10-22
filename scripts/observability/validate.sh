#!/usr/bin/env bash
#
# Static validation for the observability stack.
#
# Runs without starting the application stack:
#   * compose file resolution (docker compose config)
#   * Prometheus config and alert rules (promtool)
#   * Alertmanager config (amtool)
#   * OpenTelemetry Collector config (collector validate)
#   * dashboard JSON and shell syntax
#
# Docker is required for the tool-specific checks; use SKIP_DOCKER=1 to run only
# the file checks (useful in an environment without a container runtime).
set -Eeuo pipefail

ROOT="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

PROMETHEUS_IMAGE="prom/prometheus:v3.15.1"
ALERTMANAGER_IMAGE="prom/alertmanager:v0.34.1"
COLLECTOR_IMAGE="otel/opentelemetry-collector-contrib:0.163.2"

SKIP_DOCKER="${SKIP_DOCKER:-0}"
pass() { printf '\033[0;32mPASS\033[0m %s\n' "$*"; }
info() { printf '\033[0;36mINFO\033[0m %s\n' "$*"; }
fail() { printf '\033[0;31mFAIL\033[0m %s\n' "$*" >&2; exit 1; }

# Compose interpolates required application secrets even for a config parse.
# These are throwaway values used only by `docker compose config`; the real
# stack continues to use .env or the caller's environment.
export RUNCODES_JWT_SECRET="${RUNCODES_JWT_SECRET:-validation-only-not-a-secret}"
export RUNCODES_DB_PASSWORD="${RUNCODES_DB_PASSWORD:-validation-only}"
export RUNCODES_DB_APP_PASSWORD="${RUNCODES_DB_APP_PASSWORD:-validation-only}"
export RUNCODES_REDIS_PASSWORD="${RUNCODES_REDIS_PASSWORD:-validation-only}"
export RUNCODES_JUDGE_TOKEN="${RUNCODES_JUDGE_TOKEN:-validation-only}"
export RUNCODES_S3_CREDENTIALS_KEY="${RUNCODES_S3_CREDENTIALS_KEY:-validation-only}"
export RUNCODES_S3_CREDENTIALS_SECRET="${RUNCODES_S3_CREDENTIALS_SECRET:-validation-only}"

info "checking shell syntax"
for script in scripts/observability/*.sh; do
  bash -n "$script" || fail "shell syntax error in $script"
  pass "$script"
done

info "checking JSON files"
while IFS= read -r file; do
  jq empty "$file" || fail "invalid JSON: $file"
  pass "$file"
done < <(find observability -type f -name '*.json')

info "checking dashboard invariants"
jq -e '.uid == "runcodes-overview" and (.panels | length) > 0' \
  observability/grafana/dashboards/runcodes-overview.json >/dev/null \
  || fail "runcodes-overview.json must have uid runcodes-overview and panels"
pass "dashboard uid and panels"

./scripts/observability/init-secrets.sh >/dev/null

if [[ "$SKIP_DOCKER" == "1" ]]; then
  info "SKIP_DOCKER=1: skipping compose resolution, promtool, amtool and collector validation"
  exit 0
fi

info "resolving compose configuration"
docker compose -f docker-compose.yml -f docker-compose.observability.yml config -q \
  || fail "docker compose config failed"
pass "compose configuration resolves"

docker info >/dev/null 2>&1 || fail "Docker daemon is not reachable; set SKIP_DOCKER=1 to skip container-based checks"

info "validating Prometheus config and rules"
docker run --rm --security-opt label=disable --entrypoint promtool \
  -v "$ROOT/observability/prometheus:/etc/prometheus:ro" \
  "$PROMETHEUS_IMAGE" \
  check config /etc/prometheus/prometheus.yml
pass "promtool check config"

info "validating Alertmanager config"
docker run --rm --security-opt label=disable --entrypoint amtool \
  -v "$ROOT/observability/alertmanager:/etc/alertmanager:ro" \
  "$ALERTMANAGER_IMAGE" \
  check-config /etc/alertmanager/alertmanager.yml
pass "amtool check-config"

info "validating the Alertmanager example configs"
for config in alertmanager.gmail.example.yml alertmanager.production.example.yml; do
  docker run --rm --security-opt label=disable --entrypoint amtool \
    -v "$ROOT/observability/alertmanager:/etc/alertmanager:ro" \
    "$ALERTMANAGER_IMAGE" \
    check-config "/etc/alertmanager/$config"
  pass "amtool check-config ($config)"
done

info "validating OpenTelemetry Collector config"
docker run --rm --security-opt label=disable \
  -e DEPLOYMENT_ENVIRONMENT=development \
  -v "$ROOT/observability/otel-collector/config.yaml:/etc/otelcol/config.yaml:ro" \
  "$COLLECTOR_IMAGE" \
  validate --config=/etc/otelcol/config.yaml
pass "collector validate"

info "all observability configuration checks passed"
