#!/bin/bash
#
# Browser tracing smoke test. It loads the real SPA in Chromium, issues a fetch,
# extracts the traceparent that FetchInstrumentation injected, and verifies the
# same trace contains the frontend and backend spans in Tempo.
#
# Prerequisites:
#   make observability-up
#   cd frontend && bun run test:e2e:install   # first run only
#
# Environment:
#   FRONTEND_URL   default http://localhost:8080
#   TEMPO_URL      default http://127.0.0.1:3200
#   OTEL_HTTP_URL  default http://127.0.0.1:4318 (checked for CORS/readiness)

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FRONTEND_URL="${FRONTEND_URL:-http://localhost:8080}"
TEMPO_URL="${TEMPO_URL:-http://127.0.0.1:3200}"
OTEL_HTTP_URL="${OTEL_HTTP_URL:-http://127.0.0.1:4318}"

for url in "$FRONTEND_URL/" "$TEMPO_URL/ready" "$OTEL_HTTP_URL/v1/traces"; do
  code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 5 "$url" 2>/dev/null || true)"
  # The collector may return 405/415 for a GET on the OTLP endpoint; any HTTP
  # response proves it is listening. The other checks must be 200.
  case "$url" in
    "$OTEL_HTTP_URL"*) [[ -n "$code" && "$code" != "000" ]] || {
      echo "browser-smoke: collector is not reachable at $OTEL_HTTP_URL" >&2; exit 1; } ;;
    *) [[ "$code" == "200" ]] || {
      echo "browser-smoke: $url returned ${code:-no response}" >&2; exit 1; } ;;
  esac
done

cd "$ROOT/frontend"
FRONTEND_URL="$FRONTEND_URL" TEMPO_URL="$TEMPO_URL" bun run test:e2e
