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
#
# Lifecycle: a stack started from an SSH session dies with that session, because
# rootless containers live under the user manager and logind stops it when the
# last session closes. `./deploy.sh linger` keeps that manager alive and
# `./deploy.sh install-service` moves the stack into a user unit that starts at
# boot; `up` refuses to run without lingering unless asked to.

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

# Rootless containers live in the user manager's cgroup (user@UID.service), not
# in the SSH session's. Keeping that manager up with nobody logged in is what
# systemd calls lingering: with it off, logind stops the manager when the last
# session closes and SIGTERMs everything inside. That is the whole stack exiting
# 0/143 about ten seconds after a logout, with no OOM and no application error:
# it looks like a crash and is not one. Lingering is also what starts the user
# unit at boot, so it is a prerequisite for both halves of the fix.
linger_state() {
  loginctl show-user "$(id -un)" -p Linger 2>/dev/null | head -n 1
}

require_linger() {
  [[ "${RUNCODES_SKIP_LINGER_CHECK:-0}" == "1" ]] && return 0
  # A rootful engine has no user manager in the path.
  [[ "$(id -u)" -ne 0 ]] || return 0

  [[ "$(linger_state)" == "Linger=yes" ]] && return 0

  die "systemd lingering is off for $(id -un): when this SSH session ends, logind
stops the user manager and SIGTERMs every container (exit 0/143, no OOM, no crash
log). Fix it once per host and let the stack start at boot:

  ./deploy.sh linger            # or: sudo loginctl enable-linger $(id -un)
  ./deploy.sh install-service

Set RUNCODES_SKIP_LINGER_CHECK=1 to start the stack anyway; it will not survive
the next logout."
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

  # Alertmanager and Pyroscope take their configuration from host paths. A
  # relative override is written against the repository root, which is where the
  # runbook puts .observability/alertmanager.yml; Compose would otherwise anchor
  # it at this directory (the project directory) and mount the wrong path.
  local var value
  for var in ALERTMANAGER_CONFIG_FILE PYROSCOPE_CONFIG_FILE; do
    value="${!var:-}"
    case "$value" in
      "" | /*) ;;
      *) export "$var=$repo_root/${value#./}" ;;
    esac
  done

  export ALERTMANAGER_CONFIG_FILE="${ALERTMANAGER_CONFIG_FILE:-$RUNCODES_OBSERVABILITY_DIR/alertmanager/alertmanager.yml}"
  export PYROSCOPE_CONFIG_FILE="${PYROSCOPE_CONFIG_FILE:-$RUNCODES_OBSERVABILITY_DIR/pyroscope/config.yaml}"

  # A missing bind source is not an error to the container engine: it creates an
  # empty directory in its place and the container then fails to mount it over
  # the file the image expects. Catch it here instead.
  for var in ALERTMANAGER_CONFIG_FILE PYROSCOPE_CONFIG_FILE; do
    value="${!var}"
    [[ -f "$value" ]] || die \
      "$var points at $value, which is not a file; fix the path or unset it to use the default configuration"
  done

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

# The application is always part of the lifecycle. The telemetry overlay is
# opt-in and shares the Compose project, so the unit records whether it owns it:
# `install-service --observability` sets this, and `up`/`stop` then cover both
# halves instead of coming back with only the application running.
with_observability() {
  [[ "${RUNCODES_WITH_OBSERVABILITY:-0}" == "1" ]]
}

resolve_lifecycle_compose() {
  if with_observability; then
    observability_available || die \
      "RUNCODES_WITH_OBSERVABILITY=1 but the observability overlay or its configuration is missing; run this from a repository checkout (or set RUNCODES_OBSERVABILITY_DIR) and re-run './deploy.sh install-service --observability'"
    resolve_observability
    lifecycle_compose=("${observability_compose[@]}")
  else
    lifecycle_compose=("${compose[@]}")
  fi
}

case "$action" in
  up)
    require_podman_socket
    require_linger
    resolve_lifecycle_compose
    # The installed unit starts the images the host already has: a boot must not
    # depend on the registry or on the pull credentials still being valid.
    if [[ "${RUNCODES_SKIP_PULL:-0}" == "1" ]]; then
      printf 'deploy.sh: RUNCODES_SKIP_PULL=1, using the images already on the host\n' >&2
    else
      "${lifecycle_compose[@]}" pull
    fi
    # Only a unit that owns every service of the project may prune: for a
    # base-only `up`, the telemetry containers of the same project look like
    # orphans and would be removed rather than left alone.
    if with_observability; then
      "${lifecycle_compose[@]}" up -d --remove-orphans
    else
      "${lifecycle_compose[@]}" up -d
    fi
    "${lifecycle_compose[@]}" ps
    ;;
  observability | observability-up)
    require_podman_socket
    require_linger
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
  recreate)
    # Apply an edited bind mount (a collector config, a Prometheus rule, a
    # changed ALERTMANAGER_CONFIG_FILE, a new OTEL variable) without touching the
    # rest of the stack. The overlay is applied when it is available so the
    # configuration paths are the same absolute ones `observability` used;
    # --no-deps keeps dependencies from restarting with it.
    shift
    (($# > 0)) || die \
      "recreate needs at least one service, for example: ./deploy.sh recreate alertmanager"
    if observability_available; then
      resolve_observability
      services_compose=("${observability_compose[@]}")
    else
      services_compose=("${compose[@]}")
    fi
    "${services_compose[@]}" up -d --force-recreate --no-deps "$@"
    "${services_compose[@]}" ps "$@"
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
    require_linger
    "${compose[@]}" restart
    ;;
  stop)
    # Used by the installed unit's ExecStop: stop the containers but keep them,
    # so `systemctl --user start` brings the same stack back.
    resolve_lifecycle_compose
    "${lifecycle_compose[@]}" stop
    ;;
  linger)
    [[ "$(id -u)" -ne 0 ]] || die \
      "run this as the deployment user, not root: it lingers the current user"
    loginctl enable-linger "$(id -un)" || die \
      "loginctl could not enable lingering for $(id -un); run 'sudo loginctl enable-linger $(id -un)' as root"
    [[ "$(linger_state)" == "Linger=yes" ]] || die \
      "loginctl accepted the request but $(id -un) is still '$(linger_state)'"
    printf 'deploy.sh: %s now lingers; the stack survives logout and starts at boot\n' "$(id -un)"
    ;;
  install-service)
    require_linger
    with_obs=0
    shift
    while (($# > 0)); do
      case "$1" in
        --observability) with_obs=1 ;;
        *) die "install-service does not take '$1'; the only option is --observability" ;;
      esac
      shift
    done
    if [[ "$with_obs" == "1" ]]; then
      observability_available || die \
        "the observability overlay or its configuration is not available; run this from a repository checkout (or set RUNCODES_OBSERVABILITY_DIR) before installing a unit that owns the telemetry stack"
      resolve_observability
      init_observability_secrets
    fi
    # The unit starts with RUNCODES_SKIP_PULL=1 so a boot never waits on (or
    # fails because of) the registry. Make sure the images exist now, while an
    # operator is here to see a pull failure.
    if [[ "${RUNCODES_SKIP_PULL:-0}" != "1" ]]; then
      if [[ "$with_obs" == "1" ]]; then
        "${observability_compose[@]}" pull
      else
        "${compose[@]}" pull
      fi
    fi
    template="$script_dir/runcodes-compose.service.in"
    [[ -f "$template" ]] || die "unit template not found at $template"
    unit_dir="$HOME/.config/systemd/user"
    unit="$unit_dir/runcodes-compose.service"
    mkdir -p "$unit_dir"
    # Rewrite the placeholders with this deployment's path and stack choice.
    sed -e "s|@DEPLOY_DIR@|${script_dir//&/\\&}|g" \
        -e "s|@OBSERVABILITY@|${with_obs}|g" "$template" >"$unit"
    systemctl --user daemon-reload
    systemctl --user enable --now runcodes-compose.service
    printf 'deploy.sh: installed %s\n' "$unit"
    if [[ "$with_obs" == "1" ]]; then
      printf 'deploy.sh: the unit owns the application and the telemetry stack\n'
    else
      printf 'deploy.sh: the unit owns the application only; add --observability to include\n'
      printf '  the telemetry stack before it is started by systemd\n'
    fi
    printf 'deploy.sh: the stack now runs under the user manager; verify with\n'
    printf '  systemctl --user status runcodes-compose.service\n'
    ;;
  uninstall-service)
    # Stopping the unit runs its ExecStop, so the stack goes down with the
    # supervisor that owned it. A unit that was never installed is not an error.
    if systemctl --user disable --now runcodes-compose.service 2>/dev/null; then
      printf 'deploy.sh: stopped and disabled runcodes-compose.service\n'
    fi
    rm -f "$HOME/.config/systemd/user/runcodes-compose.service"
    systemctl --user daemon-reload 2>/dev/null || \
      printf 'deploy.sh: warning: could not reload the user manager; the removed unit disappears at its next reload\n' >&2
    printf 'deploy.sh: removed the unit; start the stack in this session with ./deploy.sh up\n'
    ;;
  db-grants)
    # The database image runs 05-app-role.sh only while initializing an empty
    # volume, so an existing volume keeps the grants it was created with. The
    # script is idempotent: it re-applies the table/sequence grants and rotates
    # both role passwords to the values in .env. The container already carries
    # RUNCODES_DB_APP_PASSWORD and RUNCODES_DB_JUDGE_PASSWORD, and psql reaches
    # the server over the local socket.
    "${compose[@]}" exec -T database /docker-entrypoint-initdb.d/05-app-role.sh
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
  stop                Stop the containers but keep them (used by the systemd unit)
  restart             Restart the existing containers
  logs                Follow the stack logs
  status              Show container status
  config              Render the final Compose configuration
  observability       Start the stack plus Prometheus, Grafana, Tempo, Loki,
                      Alertmanager and the OpenTelemetry collector
  observability-logs  Follow the observability stack logs
  recreate <service>  Recreate the named services in place, with the observability
                      overlay when available (no dependencies, no pull)
  linger              Enable systemd lingering for this user (needs sudo if polkit refuses)
  install-service     Install and start the user unit that owns the stack, so it
                      survives logout and starts at boot (requires linger).
                      Add --observability to make the unit own the telemetry
                      stack as well; otherwise it starts the application only
  uninstall-service   Disable and remove that unit, which also stops the stack
  db-grants           Re-apply the service-role grants and passwords to an existing
                      database volume (the image only runs them on an empty one)

Environment:
  CONTAINER_ENGINE                Compose engine: auto (default; Podman first), podman, docker
  ENV_FILE                        environment file to read (default: ./.env)
  PODMAN_SOCKET                   rootless Podman socket used by the judge
  RUNCODES_SKIP_PULL              start the images already on the host (the unit sets it)
  RUNCODES_WITH_OBSERVABILITY     1 makes up/stop cover the telemetry overlay too
                                  (install-service --observability writes it)
  RUNCODES_SKIP_LINGER_CHECK      start without lingering; the stack stops at the next logout
  RUNCODES_OBSERVABILITY_DIR      configuration directory (default: <repo>/observability)
  RUNCODES_OBSERVABILITY_STATE    generated secrets (default: <repo>/.observability)
  RUNCODES_OBSERVABILITY_COMPOSE  overlay file (default: <repo>/docker-compose.observability.yml)

The observability commands mount configuration from the repository, so they need
a checkout. `up` never starts the telemetry stack; it is opt-in.

`up`, `restart` and `observability` refuse to run without systemd lingering when
this is a rootless deployment: logind kills every container about ten seconds
after the last SSH session closes, which looks like a crash (exit 0/143) and is
not one. Run `./deploy.sh linger` and `./deploy.sh install-service` once per host.
EOF
    exit 2
    ;;
esac
