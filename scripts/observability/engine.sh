# shellcheck shell=bash
#
# Shared Compose engine selection for the observability helper scripts.
#
# Podman is the default. CONTAINER_ENGINE can be "auto", "podman" or "docker";
# "auto" prefers Podman and falls back to Docker Compose v2.
#
# Source this file after ROOT is set and call runcodes_select_engine. On success
# it sets and exports CONTAINER_ENGINE to "podman" or "docker" and defines the
# COMPOSE array for the application stack plus the observability overlay.

_runcodes_has_podman_compose() {
  command -v podman >/dev/null 2>&1 &&
    podman compose version >/dev/null 2>&1
}

_runcodes_has_docker_compose() {
  command -v docker >/dev/null 2>&1 &&
    docker compose version >/dev/null 2>&1
}

runcodes_select_engine() {
  local requested="${CONTAINER_ENGINE:-auto}" engine

  requested="$(printf '%s' "$requested" | tr '[:upper:]' '[:lower:]')"

  case "$requested" in
    podman)
      command -v podman >/dev/null 2>&1 || {
        printf 'observability: podman is not installed; install Podman or set CONTAINER_ENGINE=docker\n' >&2
        return 1
      }
      podman compose version >/dev/null 2>&1 || {
        printf 'observability: podman compose is required; install a Compose provider or set CONTAINER_ENGINE=docker\n' >&2
        return 1
      }
      engine=podman
      ;;
    docker)
      command -v docker >/dev/null 2>&1 || {
        printf 'observability: docker is not installed; install Docker or set CONTAINER_ENGINE=podman\n' >&2
        return 1
      }
      docker compose version >/dev/null 2>&1 || {
        printf 'observability: docker compose v2 is required; install the Compose plugin or set CONTAINER_ENGINE=podman\n' >&2
        return 1
      }
      engine=docker
      ;;
    auto)
      if _runcodes_has_podman_compose; then
        engine=podman
      elif _runcodes_has_docker_compose; then
        engine=docker
      else
        printf 'observability: neither podman compose nor docker compose is available; install one of them or set CONTAINER_ENGINE\n' >&2
        return 1
      fi
      ;;
    *)
      printf 'observability: CONTAINER_ENGINE must be auto, podman or docker (got %s)\n' "$requested" >&2
      return 1
      ;;
  esac

  CONTAINER_ENGINE="$engine"
  export CONTAINER_ENGINE
  COMPOSE=(
    "$engine" compose
    -f "$ROOT/docker-compose.yml"
    -f "$ROOT/docker-compose.observability.yml"
    -f "$ROOT/docker-compose.observability.build.yml"
  )
  printf 'observability: using %s compose\n' "$engine" >&2
}
