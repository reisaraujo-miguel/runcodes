#!/bin/bash
#
# Authenticated submission smoke test: logs in as an admin, creates a disposable
# offering/exercise/test case, submits a solution, waits for the judge, and
# verifies that Tempo contains the judge's per-test-case span for that commit.
#
# This is deliberately separate from smoke-test.sh because it needs an admin
# password. Bootstrap the seeded admin first:
#
#   RUNCODES_ADMIN_PASSWORD='...' \
#     PGHOST=127.0.0.1 PGUSER=runcodes PGDATABASE=runcodes \
#     PGPASSWORD="$RUNCODES_DB_PASSWORD" ./database/bootstrap-admin.sh
#
# then:
#
#   SMOKE_ADMIN_PASSWORD='...' make observability-submission-smoke-test
#
# Optional:
#   SMOKE_ADMIN_EMAIL      default admin@admin.com
#   SMOKE_KEEP=1           keep the temporary offering for debugging
#   BACKEND_URL            default http://127.0.0.1:8443
#   TEMPO_URL              default http://127.0.0.1:3200
#   SMOKE_TIMEOUT          seconds to wait for the judge/trace (default 120)

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BACKEND_URL="${BACKEND_URL:-http://127.0.0.1:8443}"
TEMPO_URL="${TEMPO_URL:-http://127.0.0.1:3200}"
ADMIN_EMAIL="${SMOKE_ADMIN_EMAIL:-admin@admin.com}"
ADMIN_PASSWORD="${SMOKE_ADMIN_PASSWORD:-}"
TIMEOUT="${SMOKE_TIMEOUT:-120}"
KEEP="${SMOKE_KEEP:-0}"

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
CYAN=$'\033[0;36m'
RESET=$'\033[0m'

PASS=0
FAIL=0
OFFERING_ID=""
TOKEN=""
TMP_DIR="$(mktemp -d)"

fail() {
  printf '%sFAIL%s %s\n' "$RED" "$RESET" "$*" >&2
  exit 1
}

pass() {
  PASS=$((PASS + 1))
  printf '%sPASS%s %s\n' "$GREEN" "$RESET" "$*"
}

info() {
  printf '%sINFO%s %s\n' "$CYAN" "$RESET" "$*"
}

cleanup() {
  rm -rf "$TMP_DIR"
  if [[ -n "$OFFERING_ID" && "$KEEP" != "1" && -n "$TOKEN" ]]; then
    curl -sS -o /dev/null -X DELETE \
      -H "Authorization: Bearer $TOKEN" \
      "$BACKEND_URL/api/v1/offerings/$OFFERING_ID" 2>/dev/null || true
  fi
  if [[ "$KEEP" == "1" ]]; then
    printf '%sINFO%s kept temporary offering id=%s\n' "$CYAN" "$RESET" "$OFFERING_ID"
  fi
}
trap cleanup EXIT

if [[ -z "$ADMIN_PASSWORD" ]]; then
  cat >&2 <<'MSG'
SMOKE_ADMIN_PASSWORD is not set.

The seeded admin account has no password. Bootstrap it first:

  RUNCODES_ADMIN_PASSWORD='...' \
    PGHOST=127.0.0.1 PGUSER=runcodes PGDATABASE=runcodes \
    PGPASSWORD="$RUNCODES_DB_PASSWORD" ./database/bootstrap-admin.sh

Then run:

  SMOKE_ADMIN_PASSWORD='...' make observability-submission-smoke-test

The test creates and deletes a disposable offering; it does not use any
pre-existing exercise or submission.
MSG
  exit 1
fi

for tool in curl jq date; do
  command -v "$tool" >/dev/null 2>&1 || fail "required tool '$tool' is not installed"
done

for url in "$BACKEND_URL/healthz" "$TEMPO_URL/ready"; do
  code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 5 "$url" 2>/dev/null || true)"
  [[ "$code" == "200" ]] || fail "endpoint $url returned ${code:-no response}"
done
pass "backend and Tempo are reachable"

# ---------------------------------------------------------------------------
# Login. The session cookie is Secure in the default (non-debug) configuration,
# so extract the JWT from Set-Cookie and send it as a bearer token over HTTP.
# ---------------------------------------------------------------------------
login_headers="$TMP_DIR/login.headers"
login_code="$(curl -sS -D "$login_headers" -o "$TMP_DIR/login.body" -w '%{http_code}' \
  -X POST "$BACKEND_URL/api/v1/user/login" \
  -H 'Content-Type: application/json' \
  --data "$(jq -nc --arg email "$ADMIN_EMAIL" --arg password "$ADMIN_PASSWORD" \
    '{email:$email,password:$password}')" 2>/dev/null || true)"
[[ "$login_code" == "200" ]] || fail "admin login returned HTTP ${login_code:-none}: $(head -c 300 "$TMP_DIR/login.body" 2>/dev/null)"
TOKEN="$(grep -i '^Set-Cookie: jwt=' "$login_headers" | sed -E 's/^[Ss]et-[Cc]ookie: jwt=([^;]+).*/\1/' | head -1)"
[[ -n "$TOKEN" ]] || fail "login succeeded but no jwt cookie was returned"
pass "authenticated as $ADMIN_EMAIL"

api() {
  local method="$1" path="$2"
  shift 2
  curl -sS -X "$method" -H "Authorization: Bearer $TOKEN" "$@" "$BACKEND_URL$path"
}

# ---------------------------------------------------------------------------
# Disposable class, exercise and test case.
# ---------------------------------------------------------------------------
end_date="$(date -u -d '+2 days' +%Y-%m-%dT%H:%M:%SZ)"
offering="$(api POST /api/v1/offerings/create \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc --arg name "observability-smoke-$(date +%s)-$$" \
    --arg end "$end_date" \
    '{name:$name,end_date:$end,description:"temporary authenticated submission smoke"}')")"
OFFERING_ID="$(jq -r '.id // empty' <<<"$offering")"
[[ -n "$OFFERING_ID" ]] || fail "offering creation failed: $(head -c 300 <<<"$offering")"
pass "created temporary offering id=$OFFERING_ID"

python_type_id="$(api GET /api/v1/allowed-file-types \
  | jq -r '.[] | select(.name=="Python 3") | .id' | head -1)"
[[ -n "$python_type_id" ]] || fail "Python 3 is not an allowed file type"
open_date="$(date -u -d '-5 minutes' +%Y-%m-%dT%H:%M:%SZ)"
deadline="$(date -u -d '+1 hour' +%Y-%m-%dT%H:%M:%SZ)"
exercise="$(api POST "/api/v1/offerings/$OFFERING_ID/exercises" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc --arg deadline "$deadline" --arg open "$open_date" \
    --argjson type "$python_type_id" \
    '{title:"observability sum smoke",description:"read two integers and print their sum",deadline:$deadline,open_date:$open,show_before_open_date:true,allowed_file_type_ids:[$type]}')")"
exercise_id="$(jq -r '.id // empty' <<<"$exercise")"
[[ -n "$exercise_id" ]] || fail "exercise creation failed: $(head -c 300 <<<"$exercise")"
pass "created exercise id=$exercise_id"

test_case="$(api POST "/api/v1/exercises/$exercise_id/test-cases" \
  -F 'input_type=text' -F 'input=3 4' \
  -F 'expected_output_type=text' -F 'expected_output=7' \
  -F 'show_input=true' -F 'show_expected_output=true' -F 'show_user_output=true' \
  -F 'cpu_time_limit_seconds=2' -F 'mem_usage_limit_bytes=67108864' \
  -F 'stack_limit_bytes=8388608' -F 'file_size_limit_bytes=1048576')"
test_case_id="$(jq -r '.id // empty' <<<"$test_case")"
[[ -n "$test_case_id" ]] || fail "test case creation failed: $(head -c 300 <<<"$test_case")"
pass "created test case id=$test_case_id (input '3 4' -> '7')"

printf 'a, b = map(int, input().split())\nprint(a + b)\n' > "$TMP_DIR/main.py"

submission_headers="$TMP_DIR/submission.headers"
submission_code="$(curl -sS -D "$submission_headers" -o "$TMP_DIR/submission.json" \
  -w '%{http_code}' -H "Authorization: Bearer $TOKEN" \
  -X POST "$BACKEND_URL/api/v1/submissions" \
  -F "exercise_id=$exercise_id" \
  -F "file=@$TMP_DIR/main.py;filename=main.py;type=text/x-python" 2>/dev/null || true)"
[[ "$submission_code" == "201" ]] || fail "submission returned HTTP ${submission_code:-none}: $(head -c 300 "$TMP_DIR/submission.json" 2>/dev/null)"
commit_id="$(jq -r '.commit_id // empty' "$TMP_DIR/submission.json")"
[[ -n "$commit_id" ]] || fail "submission response has no commit_id"
pass "submission queued as commit id=$commit_id"

start_epoch="$(date +%s)"
status=""
for _ in $(seq 1 "$TIMEOUT"); do
  history="$(api GET "/api/v1/exercises/$exercise_id/submissions")"
  status="$(jq -r --argjson id "$commit_id" '.[] | select(.id == $id) | .status' <<<"$history" | head -1)"
  case "$status" in
    completed|uncompleted|compilation_error|server_error|timeout) break ;;
    queued|compiling|running|"") ;;
    *) fail "unexpected submission status '$status'" ;;
  esac
  sleep 1
done

case "$status" in
  completed|uncompleted) ;;
  *) fail "submission did not reach a terminal status within ${TIMEOUT}s (last: ${status:-unknown})" ;;
esac
[[ "$status" == "completed" ]] || fail "submission finished with status '$status', expected completed"
pass "judge completed commit $commit_id"

# ---------------------------------------------------------------------------
# Find the judge trace for this commit and assert the per-test-case span.
# ---------------------------------------------------------------------------
trace_id=""
for _ in $(seq 1 60); do
  search="$(curl -sS -G "$TEMPO_URL/api/search" \
    --data-urlencode 'tags=name="judge.process_commit"' \
    --data-urlencode 'limit=50' \
    --data-urlencode "start=$((start_epoch - 60))" \
    --data-urlencode "end=$(( $(date +%s) + 60 ))" 2>/dev/null || true)"
  for candidate in $(jq -r '.traces[]?.traceID // empty' <<<"$search"); do
    trace_json="$(curl -sS "$TEMPO_URL/api/traces/$candidate" 2>/dev/null || true)"
    if jq -e --argjson id "$commit_id" '
      [.batches[].scopeSpans[].spans[]
       | select(.name == "judge.process_commit")
       | .attributes[]?
       | select(.key == "runcodes.commit.id")
       | (.value.intValue // .value.stringValue | tostring)
       | select(. == ($id | tostring))] | length > 0
    ' <<<"$trace_json" >/dev/null 2>&1; then
      trace_id="$candidate"
      break 2
    fi
  done
  sleep 2
done
[[ -n "$trace_id" ]] || fail "no judge.process_commit trace for commit $commit_id in Tempo"
pass "judge trace for commit $commit_id is in Tempo ($trace_id)"

trace_json="$(curl -sS "$TEMPO_URL/api/traces/$trace_id")"
case_span="$(jq -c --argjson id "$commit_id" '
  [.batches[].scopeSpans[].spans[]
   | select(.name == "judge.grade_test_case")
   | select(any(.attributes[]?; .key == "runcodes.commit.id" and ((.value.intValue // .value.stringValue | tostring) == ($id | tostring))))]
  | first // empty
' <<<"$trace_json")"
[[ -n "$case_span" ]] || fail "judge.grade_test_case span is missing from trace $trace_id"
case_status="$(jq -r '[.attributes[]? | select(.key=="runcodes.test_case.status") | .value.stringValue] | first // empty' <<<"$case_span")"
[[ "$case_status" == "correct" ]] || fail "test case span verdict is '${case_status:-unknown}', expected correct"
pass "per-test-case span is present and graded correct"

profile_id="$(jq -r '
  [.batches[].scopeSpans[].spans[]
   | select(.name == "judge.grade_test_case")
   | .attributes[]?
   | select(.key == "pyroscope.profile.id")
   | .value.stringValue] | first // empty
' <<<"$trace_json")"
[[ -n "$profile_id" ]] || fail "pyroscope.profile.id is missing from the test-case span"
pass "span-profile bridge attribute is present (profile id ${profile_id:0:12}...)"

info "trace: $TEMPO_URL/api/traces/$trace_id"

printf '\n%sAll authenticated submission smoke checks passed.%s\n' "$GREEN" "$RESET"
printf '  Commit:     %s\n' "$commit_id"
printf '  Trace:      %s/api/traces/%s\n' "$TEMPO_URL" "$trace_id"
printf '  Test case:  %s\n' "$test_case_id"
