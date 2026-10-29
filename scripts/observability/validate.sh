#!/usr/bin/env bash
#
# Static validation for the observability stack.
#
# Runs without starting the application stack:
#   * compose file resolution (podman compose config)
#   * Prometheus config and alert rules (promtool)
#   * Alertmanager config (amtool)
#   * OpenTelemetry Collector config (collector validate)
#   * dashboard JSON and shell syntax
#
# A container engine is required for the tool-specific checks; Podman is the
# default and Docker Compose v2 is supported. Use SKIP_DOCKER=1 to run only the
# file checks (useful in an environment without a container runtime).
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
# These are throwaway values used only by the compose config check; the real
# stack continues to use .env or the caller's environment.
export RUNCODES_JWT_SECRET="${RUNCODES_JWT_SECRET:-validation-only-not-a-secret}"
export RUNCODES_DB_PASSWORD="${RUNCODES_DB_PASSWORD:-validation-only}"
export RUNCODES_DB_APP_PASSWORD="${RUNCODES_DB_APP_PASSWORD:-validation-only}"
export RUNCODES_DB_JUDGE_PASSWORD="${RUNCODES_DB_JUDGE_PASSWORD:-validation-only}"
export RUNCODES_REDIS_PASSWORD="${RUNCODES_REDIS_PASSWORD:-validation-only}"
export RUNCODES_JUDGE_TOKEN="${RUNCODES_JUDGE_TOKEN:-validation-only}"
export RUNCODES_S3_CREDENTIALS_KEY="${RUNCODES_S3_CREDENTIALS_KEY:-validation-only}"
export RUNCODES_S3_CREDENTIALS_SECRET="${RUNCODES_S3_CREDENTIALS_SECRET:-validation-only}"

info "checking shell syntax"
for script in scripts/observability/*.sh; do
  bash -n "$script" || fail "shell syntax error in $script"
  pass "$script"
done
bash -n deploy/compose/deploy.sh || fail "shell syntax error in deploy/compose/deploy.sh"
pass "deploy/compose/deploy.sh"

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

# shellcheck source=scripts/observability/engine.sh
source "$ROOT/scripts/observability/engine.sh"
runcodes_select_engine || exit 1

info "resolving compose configuration"
"${COMPOSE[@]}" config -q || fail "compose config failed"
pass "compose configuration resolves"

# The image-based deployment reuses the overlay from the repository root. Its
# Compose file owns EXEC_DIR and PODMAN_SOCKET and resolves configuration paths
# from the environment, so exercise the merge with the same overrides that
# deploy/compose/deploy.sh exports.
info "resolving the image-based deployment with the overlay"
image_compose=(
  "$CONTAINER_ENGINE" compose
  -f "$ROOT/deploy/compose/docker-compose.yml"
  -f "$ROOT/docker-compose.observability.yml"
)
RUNCODES_EXEC_DIR="$ROOT/exec" \
PODMAN_SOCKET="/run/user/$(id -u)/podman/podman.sock" \
RUNCODES_OBSERVABILITY_DIR="$ROOT/observability" \
RUNCODES_OBSERVABILITY_STATE="$ROOT/.observability" \
ALERTMANAGER_CONFIG_FILE="$ROOT/observability/alertmanager/alertmanager.yml" \
PYROSCOPE_CONFIG_FILE="$ROOT/observability/pyroscope/config.yaml" \
  "${image_compose[@]}" config -q || fail "image-based compose config failed"
pass "image-based compose configuration resolves"

"$CONTAINER_ENGINE" info >/dev/null 2>&1 || fail "container engine is not reachable; set SKIP_DOCKER=1 to skip container-based checks"

info "validating Prometheus config and rules"
"$CONTAINER_ENGINE" run --rm --security-opt label=disable --entrypoint promtool \
  -v "$ROOT/observability/prometheus:/etc/prometheus:ro" \
  "$PROMETHEUS_IMAGE" \
  check config /etc/prometheus/prometheus.yml
pass "promtool check config"

info "validating Alertmanager config"
"$CONTAINER_ENGINE" run --rm --security-opt label=disable --entrypoint amtool \
  -v "$ROOT/observability/alertmanager:/etc/alertmanager:ro" \
  "$ALERTMANAGER_IMAGE" \
  check-config /etc/alertmanager/alertmanager.yml
pass "amtool check-config"

info "validating the Alertmanager example configs"
for config in alertmanager.gmail.example.yml alertmanager.production.example.yml; do
  "$CONTAINER_ENGINE" run --rm --security-opt label=disable --entrypoint amtool \
    -v "$ROOT/observability/alertmanager:/etc/alertmanager:ro" \
    "$ALERTMANAGER_IMAGE" \
    check-config "/etc/alertmanager/$config"
  pass "amtool check-config ($config)"
done

info "validating OpenTelemetry Collector config"
"$CONTAINER_ENGINE" run --rm --security-opt label=disable \
  -e DEPLOYMENT_ENVIRONMENT=development \
  -v "$ROOT/observability/otel-collector/config.yaml:/etc/otelcol/config.yaml:ro" \
  "$COLLECTOR_IMAGE" \
  validate --config=/etc/otelcol/config.yaml
pass "collector validate"

info "all observability configuration checks passed"
