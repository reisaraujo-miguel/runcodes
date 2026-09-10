#!/usr/bin/env bash
#
# End-to-end harness for the judge integration test.
#
# Starts ephemeral PostgreSQL and SeaweedFS containers, loads the platform
# schema, and runs the `integration`-tagged Go test, which drives the real
# engine against a real language image through rootless podman. Everything is
# torn down afterwards unless --keep is passed.
#
# Requirements: docker, curl, a running rootless podman API socket, and the
# language image available locally (the test pulls it if needed).
#
# Usage:
#   integration/run.sh [--keep]
#
# Environment overrides:
#   ITEST_DB_PORT, ITEST_S3_PORT, RUNCODES_S3_BUCKET_PREFIX, JUDGE_PODMAN_URI,
#   JUDGE_TEST_IMAGE_FORMAT, TAGS

set -euo pipefail

KEEP=0
if [[ "${1:-}" == "--keep" ]]; then
  KEEP=1
fi

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
judge_dir="$(cd "$here/.." && pwd)"
schema_dir="$(cd "$judge_dir/../database/schema" && pwd)"

DB_NAME=runcodes-judge-itest
SEA_NAME=runcodes-judge-itest-seaweed
DB_PORT="${ITEST_DB_PORT:-55432}"
SEA_PORT="${ITEST_S3_PORT:-18333}"
DB_USER=runcodes
DB_PASS=itest
DB_DB=runcodes

cleanup() {
  if [[ "$KEEP" == "1" ]]; then
    echo ">> keeping containers: $DB_NAME $SEA_NAME"
    return
  fi
  docker rm -f "$DB_NAME" "$SEA_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo ">> starting $DB_NAME on :$DB_PORT"
docker rm -f "$DB_NAME" >/dev/null 2>&1 || true
docker run -d --name "$DB_NAME" \
  -e POSTGRES_USER="$DB_USER" \
  -e POSTGRES_PASSWORD="$DB_PASS" \
  -e POSTGRES_DB="$DB_DB" \
  -p "${DB_PORT}:5432" \
  postgres:18-alpine >/dev/null

echo ">> starting $SEA_NAME on :$SEA_PORT"
docker rm -f "$SEA_NAME" >/dev/null 2>&1 || true
docker run -d --name "$SEA_NAME" \
  -p "${SEA_PORT}:8333" \
  chrislusf/seaweedfs:4.24 \
  server -s3 -dir=/data -master.volumeSizeLimitMB=16 >/dev/null

echo ">> waiting for postgres"
for _ in $(seq 1 60); do
  if docker exec "$DB_NAME" pg_isready -U "$DB_USER" -d "$DB_DB" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

echo ">> loading schema"
for file in 01-init.sql 02-schema.sql 03-seed-data.sql; do
  docker exec -i "$DB_NAME" psql -v ON_ERROR_STOP=1 -U "$DB_USER" -d "$DB_DB" \
    < "$schema_dir/$file" >/dev/null
done

echo ">> waiting for seaweedfs"
for _ in $(seq 1 60); do
  code="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${SEA_PORT}/" 2>/dev/null || true)"
  if [[ -n "$code" && "$code" != "000" ]]; then
    break
  fi
  sleep 1
done

default_tags="remote containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_overlay exclude_graphdriver_devicemapper exclude_graphdriver_zfs btrfs_noversion"
TAGS="integration ${TAGS:-$default_tags}"

echo ">> running integration test"
cd "$judge_dir"
JUDGE_TEST_DB_DSN="host=127.0.0.1 port=${DB_PORT} user=${DB_USER} password=${DB_PASS} dbname=${DB_DB} sslmode=disable" \
RUNCODES_S3_ENDPOINT="http://127.0.0.1:${SEA_PORT}" \
RUNCODES_S3_CREDENTIALS_KEY="test_key" \
RUNCODES_S3_CREDENTIALS_SECRET="test_secret" \
RUNCODES_S3_BUCKET_PREFIX="${RUNCODES_S3_BUCKET_PREFIX:-runcodes-itest}" \
go test -tags "$TAGS" -count=1 -v -run TestJudgeRunsLanguageImageEndToEnd .
