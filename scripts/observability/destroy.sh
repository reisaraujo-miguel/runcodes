#!/usr/bin/env bash
#
# Destroys the observability stack's containers AND only the observability
# volumes. The application volumes (Postgres, Redis, SeaweedFS, Caddy and
# smtp4dev) are left untouched: `compose down -v` would delete those too,
# which is not what this command promises.
#
# Usage:
#   ./scripts/observability/destroy.sh           # prompt before deleting
#   FORCE=1 ./scripts/observability/destroy.sh   # no prompt (CI/automation)
#
# CONTAINER_ENGINE selects Podman (default), Docker, or auto.
set -Eeuo pipefail

ROOT="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

# shellcheck source=scripts/observability/engine.sh
source "$ROOT/scripts/observability/engine.sh"
runcodes_select_engine || exit 1

VOLUME_SUFFIXES=(
  prometheus-data
  alertmanager-data
  loki-data
  tempo-data
  grafana-data
  pyroscope-data
  alloy-data
)

if [[ "${FORCE:-0}" != "1" ]]; then
  read -r -p "Delete all observability containers and telemetry volumes? [y/N] " answer
  [[ "$answer" == "y" || "$answer" == "Y" ]] || { echo "Aborted."; exit 1; }
fi

# Resolve the project name the same way Compose does, so custom
# COMPOSE_PROJECT_NAME values work too.
project="$("${COMPOSE[@]}" config --format json 2>/dev/null | jq -r '.name // empty' 2>/dev/null || true)"
if [[ -z "$project" ]]; then
  project="$(basename "$ROOT" | tr '[:upper:]' '[:lower:]' | tr -cd 'a-z0-9_-')"
fi

echo "observability: stopping and removing containers"
"${COMPOSE[@]}" down

for suffix in "${VOLUME_SUFFIXES[@]}"; do
  volume="${project}_${suffix}"
  if "$CONTAINER_ENGINE" volume inspect "$volume" >/dev/null 2>&1; then
    echo "observability: removing volume $volume"
    "$CONTAINER_ENGINE" volume rm "$volume" >/dev/null
  fi
done

echo "observability: removed only the observability volumes; application data was kept"
