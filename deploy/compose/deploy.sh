#!/usr/bin/env bash
#
# Deploy the image-based single-host stack in this directory.
#
# The script reads .env (or ENV_FILE), resolves the host paths Compose cannot
# derive, checks the rootless Podman socket the judge needs, then pulls and
# starts the stack. It never builds from source.
#
# Compose can run on Podman (the default) or Docker. CONTAINER_ENGINE selects
# the engine; "auto" prefers Podman and falls back to Docker.
#
# The observability stack is opt-in: `./deploy.sh observability` adds the
# repository-root docker-compose.observability.yml overlay together with the
# telemetry configuration in observability/. Those assets live in the checkout,
# so the observability commands need more than this directory.

set -Eeuo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$script_dir"

# Keep an explicit engine choice ahead of the one in .env, so
# `CONTAINER_ENGINE=docker ./deploy.sh up` overrides the file.
requested_engine="${CONTAINER_ENGINE:-}"

env_file="${ENV_FILE:-$script_dir/.env}"
if [[ ! -f "$env_file" ]]; then
  printf 'deploy.sh: %s not found; copy .env.example to .env and fill it\n' \
    "$env_file" >&2
  exit 1
fi

# .env is shell-compatible so the values needed for path resolution are
# available here. Compose receives the same file via --env-file.
set -a
# shellcheck disable=SC1090
source "$env_file"
set +a

die() {
  printf 'deploy.sh: %s\n' "$*" >&2
  exit 1
}

engine="${requested_engine:-${CONTAINER_ENGINE:-auto}}"
engine="$(printf '%s' "$engine" | tr '[:upper:]' '[:lower:]')"

has_podman_compose() {
  command -v podman >/dev/null 2>&1 &&
    podman compose version >/dev/null 2>&1
}

has_docker_compose() {
  command -v docker >/dev/null 2>&1 &&
    docker compose version >/dev/null 2>&1
}

case "$engine" in
  podman)
    command -v podman >/dev/null 2>&1 ||
      die "podman is not installed; install Podman or set CONTAINER_ENGINE=docker"
    podman compose version >/dev/null 2>&1 ||
      die "podman compose is required; install a Compose provider or set CONTAINER_ENGINE=docker"
    ;;
  docker)
    command -v docker >/dev/null 2>&1 ||
      die "docker is not installed; install Docker or set CONTAINER_ENGINE=podman"
    docker compose version >/dev/null 2>&1 ||
      die "docker compose v2 is required; install the Compose plugin or set CONTAINER_ENGINE=podman"
    ;;
  auto)
    if has_podman_compose; then
      engine=podman
    elif has_docker_compose; then
      engine=docker
    else
      die "neither 'podman compose' nor 'docker compose' is available; install Podman with a Compose provider, or Docker with Compose v2"
    fi
    ;;
  *)
    die "CONTAINER_ENGINE must be 'podman', 'docker' or 'auto' (got '$engine')"
    ;;
esac

CONTAINER_ENGINE="$engine"
export CONTAINER_ENGINE
printf 'deploy.sh: using %s compose\n' "$engine" >&2

export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-runcodes}"
export RUNCODES_IMAGE_REGISTRY="${RUNCODES_IMAGE_REGISTRY:-ghcr.io}"
export RUNCODES_IMAGE_NAMESPACE="${RUNCODES_IMAGE_NAMESPACE:-runcodes-icmc}"
export RUNCODES_IMAGE_TAG="${RUNCODES_IMAGE_TAG:-latest}"
export RUNCODES_RUNNER_IMAGE_PREFIX="${RUNCODES_RUNNER_IMAGE_PREFIX:-runcodes-icmc/runcodes-runner}"
export RUNCODES_RUNNER_IMAGE_TAG="${RUNCODES_RUNNER_IMAGE_TAG:-latest}"

# Compose needs an absolute host path for JUDGE_EXEC_DIR_REMOTE: Podman mounts
# it from the host, not from the judge container. This applies with either
# Compose engine because the judge always drives Podman.
exec_dir="${RUNCODES_EXEC_DIR:-$script_dir/exec}"
mkdir -p "$exec_dir"
RUNCODES_EXEC_DIR="$(cd -- "$exec_dir" && pwd -P)"
export RUNCODES_EXEC_DIR

podman_socket="${PODMAN_SOCKET:-/run/user/$(id -u)/podman/podman.sock}"
PODMAN_SOCKET="$podman_socket"
export PODMAN_SOCKET

require_podman_socket() {
  [[ -S "$podman_socket" ]] || die \
    "podman socket not found at $podman_socket; the judge needs it even when Compose runs on Docker. Run 'systemctl --user start podman.socket' or set PODMAN_SOCKET"
}

compose=("$engine" compose --env-file "$env_file" -f "$script_dir/docker-compose.yml")
action="${1:-up}"

# --- Optional observability stack -----------------------------------------
# The telemetry services are the repository-root docker-compose.observability.yml
# overlay plus the configuration in observability/. A deployment directory that
# was copied out of a checkout cannot start them: clone the repository, or point
# the variables below at a copy of those assets.
repo_root="$(cd -- "$script_dir/../.." && pwd -P)"
observability_compose=()
observability_services=(
  otel-collector prometheus alertmanager alert-sink loki tempo pyroscope
  alloy grafana node-exporter postgres-exporter redis-exporter blackbox-exporter
)

observability_available() {
  local dir="${RUNCODES_OBSERVABILITY_DIR:-$repo_root/observability}"
  local overlay="${RUNCODES_OBSERVABILITY_COMPOSE:-$repo_root/docker-compose.observability.yml}"
  [[ -f "$dir/otel-collector/config.yaml" && -f "$overlay" ]]
}

resolve_observability() {
  local dir overlay

  dir="${RUNCODES_OBSERVABILITY_DIR:-$repo_root/observability}"
  [[ -f "$dir/otel-collector/config.yaml" ]] || die \
    "observability configuration not found at $dir; run this from a repository checkout or set RUNCODES_OBSERVABILITY_DIR"
  RUNCODES_OBSERVABILITY_DIR="$(cd -- "$dir" && pwd -P)"
  export RUNCODES_OBSERVABILITY_DIR

  dir="${RUNCODES_OBSERVABILITY_STATE:-$repo_root/.observability}"
  if [[ -d "$dir" ]]; then
    dir="$(cd -- "$dir" && pwd -P)"
  fi
  RUNCODES_OBSERVABILITY_STATE="$dir"
  export RUNCODES_OBSERVABILITY_STATE

  # The overlay defaults these to paths relative to the repository root, which
  # is not the Compose project directory here.
  export ALERTMANAGER_CONFIG_FILE="${ALERTMANAGER_CONFIG_FILE:-$RUNCODES_OBSERVABILITY_DIR/alertmanager/alertmanager.yml}"
  export PYROSCOPE_CONFIG_FILE="${PYROSCOPE_CONFIG_FILE:-$RUNCODES_OBSERVABILITY_DIR/pyroscope/config.yaml}"

  overlay="${RUNCODES_OBSERVABILITY_COMPOSE:-$repo_root/docker-compose.observability.yml}"
  [[ -f "$overlay" ]] || die "observability overlay not found at $overlay"

  observability_compose=("${compose[@]}" -f "$overlay")
}

init_observability_secrets() {
  local script="$repo_root/scripts/observability/init-secrets.sh"
  [[ -f "$script" ]] || die \
    "observability secret helper not found at $script; run this from a repository checkout"
  mkdir -p "$RUNCODES_OBSERVABILITY_STATE"
  OBSERVABILITY_STATE_DIR="$RUNCODES_OBSERVABILITY_STATE" bash "$script"
}

print_observability_urls() {
  local bind="${OBSERVABILITY_BIND_ADDRESS:-127.0.0.1}"
  printf 'observability: Grafana      http://%s:%s   (admin password: %s/grafana_admin_password)\n' \
    "$bind" "${GRAFANA_PORT:-3000}" "$RUNCODES_OBSERVABILITY_STATE"
  printf 'observability: Prometheus   http://%s:%s\n' "$bind" "${PROMETHEUS_PORT:-9090}"
  printf 'observability: Tempo        http://%s:%s\n' "$bind" "${TEMPO_PORT:-3200}"
  printf 'observability: Loki         http://%s:%s\n' "$bind" "${LOKI_PORT:-3100}"
  printf 'observability: Alertmanager http://%s:%s\n' "$bind" "${ALERTMANAGER_PORT:-9093}"
  printf 'observability: collector    http://%s:%s/health/status\n' \
    "$bind" "${OTEL_HEALTH_PORT:-13133}"
}

case "$action" in
  up)
    require_podman_socket
    "${compose[@]}" pull
    "${compose[@]}" up -d --remove-orphans
    "${compose[@]}" ps
    ;;
  observability | observability-up)
    require_podman_socket
    resolve_observability
    init_observability_secrets
    "${observability_compose[@]}" pull
    "${observability_compose[@]}" up -d --remove-orphans
    "${observability_compose[@]}" ps
    print_observability_urls
    ;;
  observability-logs)
    resolve_observability
    "${observability_compose[@]}" logs -f --tail=100 \
      "${observability_services[@]}"
    ;;
  pull)
    "${compose[@]}" pull
    ;;
  down)
    # The telemetry containers are part of the same project; a base-only `down`
    # would leave them running as orphans.
    if observability_available; then
      resolve_observability
      "${observability_compose[@]}" down
    else
      "${compose[@]}" down
    fi
    ;;
  restart)
    require_podman_socket
    "${compose[@]}" restart
    ;;
  logs)
    "${compose[@]}" logs -f --tail=100
    ;;
  status | ps)
    if observability_available; then
      resolve_observability
      "${observability_compose[@]}" ps
    else
      "${compose[@]}" ps
    fi
    ;;
  config)
    "${compose[@]}" config
    ;;
  *)
    cat >&2 <<'EOF'
Usage: ./deploy.sh [command]

Commands:
  up                  Pull images, start or update the stack, then show its status (default)
  pull                Pull the configured images without starting anything
  down                Stop and remove the containers, including the observability
                      stack when it is available (volumes are kept)
  restart             Restart the existing containers
  logs                Follow the stack logs
  status              Show container status
  config              Render the final Compose configuration
  observability       Start the stack plus Prometheus, Grafana, Tempo, Loki,
                      Alertmanager and the OpenTelemetry collector
  observability-logs  Follow the observability stack logs

Environment:
  CONTAINER_ENGINE                Compose engine: auto (default; Podman first), podman, docker
  ENV_FILE                        environment file to read (default: ./.env)
  PODMAN_SOCKET                   rootless Podman socket used by the judge
  RUNCODES_OBSERVABILITY_DIR      configuration directory (default: <repo>/observability)
  RUNCODES_OBSERVABILITY_STATE    generated secrets (default: <repo>/.observability)
  RUNCODES_OBSERVABILITY_COMPOSE  overlay file (default: <repo>/docker-compose.observability.yml)

The observability commands mount configuration from the repository, so they need
a checkout. `up` never starts the telemetry stack; it is opt-in.
EOF
    exit 2
    ;;
esac
