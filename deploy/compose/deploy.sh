#!/usr/bin/env bash
#
# Deploy the image-based single-host stack in this directory.
#
# The script reads .env (or ENV_FILE), resolves the host paths Compose cannot
# derive, checks the rootless Podman socket, then pulls and starts the stack.
# It never builds from source.

set -Eeuo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$script_dir"

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

command -v docker >/dev/null 2>&1 || die "docker is not installed"
docker compose version >/dev/null 2>&1 || die "docker compose v2 is required"

export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-runcodes}"
export RUNCODES_IMAGE_REGISTRY="${RUNCODES_IMAGE_REGISTRY:-ghcr.io}"
export RUNCODES_IMAGE_NAMESPACE="${RUNCODES_IMAGE_NAMESPACE:-runcodes-icmc}"
export RUNCODES_IMAGE_TAG="${RUNCODES_IMAGE_TAG:-latest}"
export RUNCODES_RUNNER_IMAGE_PREFIX="${RUNCODES_RUNNER_IMAGE_PREFIX:-runcodes-icmc/runcodes-runner}"
export RUNCODES_RUNNER_IMAGE_TAG="${RUNCODES_RUNNER_IMAGE_TAG:-latest}"

# Compose needs an absolute host path for JUDGE_EXEC_DIR_REMOTE: Podman mounts
# it from the host, not from the judge container.
exec_dir="${RUNCODES_EXEC_DIR:-$script_dir/exec}"
mkdir -p "$exec_dir"
RUNCODES_EXEC_DIR="$(cd -- "$exec_dir" && pwd -P)"
export RUNCODES_EXEC_DIR

podman_socket="${PODMAN_SOCKET:-/run/user/$(id -u)/podman/podman.sock}"
PODMAN_SOCKET="$podman_socket"
export PODMAN_SOCKET

require_podman_socket() {
  [[ -S "$podman_socket" ]] || die \
    "podman socket not found at $podman_socket; run 'systemctl --user start podman.socket' or set PODMAN_SOCKET"
}

compose=(podman compose --env-file "$env_file" -f "$script_dir/docker-compose.yml")
action="${1:-up}"

case "$action" in
  up)
    require_podman_socket
    "${compose[@]}" pull
    "${compose[@]}" up -d --remove-orphans
    "${compose[@]}" ps
    ;;
  pull)
    "${compose[@]}" pull
    ;;
  down)
    "${compose[@]}" down
    ;;
  restart)
    require_podman_socket
    "${compose[@]}" restart
    ;;
  logs)
    "${compose[@]}" logs -f --tail=100
    ;;
  status | ps)
    "${compose[@]}" ps
    ;;
  config)
    "${compose[@]}" config
    ;;
  *)
    cat >&2 <<'EOF'
Usage: ./deploy.sh [command]

Commands:
  up       Pull images, start or update the stack, then show its status (default)
  pull     Pull the configured images without starting anything
  down     Stop and remove the containers (volumes are kept)
  restart  Restart the existing containers
  logs     Follow the stack logs
  status   Show container status
  config   Render the final Compose configuration
EOF
    exit 2
    ;;
esac
