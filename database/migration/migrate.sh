#!/bin/sh
# Loads the old production schema + data into an `old` schema next to the new
# one, then runs the migration scripts in order.
#
# The connection is taken from the standard PG* environment variables and must
# point at the server holding the NEW database (the one created by
# docker-compose / database/Dockerfile), connecting as the database owner
# (runcodes).
#
# Usage:
#   ./migrate.sh OLD_SCHEMA.sql OLD_DATA.sql
#
#   OLD_SCHEMA.sql  schema dump of the old database
#                   (e.g. ../old_schema/schema.old.sql)
#   OLD_DATA.sql    data-only dump from production:
#                     pg_dump --data-only --inserts -h <prod> -U <user> -d runcodes -f old-data.sql

set -eu

OLD_SCHEMA=${1:?usage: ./migrate.sh OLD_SCHEMA.sql OLD_DATA.sql}
OLD_DATA=${2:?usage: ./migrate.sh OLD_SCHEMA.sql OLD_DATA.sql}
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

# Each step below runs inside one transaction, so a failure leaves the database
# exactly as it was and the migration can simply be re-run.
PSQL="psql -v ON_ERROR_STOP=1 -q"

# 1. Load the old schema into `old`.
#    The condensed dump creates its objects unqualified, so routing them into
#    `old` is done with search_path alone. Ownership and privilege statements are
#    dropped instead of rewritten: they name the `public` schema and the `public`
#    role, neither of which should be touched here, and `public` is also a real
#    column of the legacy `exercises` table and the prefix of `public_exercises`.
#    A blanket substitution would corrupt those names and then fail on the
#    REVOKE that targets the non-existent role `old`.
#
#    CREATE SCHEMA is part of the same transaction so a failed load does not
#    leave an empty `old` schema behind to block a retry.
{
  echo 'BEGIN;'
  echo 'CREATE SCHEMA old;'
  sed \
    -e '/^CREATE USER /d' \
    -e '/^CREATE DATABASE /d' \
    -e '/^GRANT /d' \
    -e '/^REVOKE /d' \
    -e '/^\\c /d' \
    "$OLD_SCHEMA"
  echo 'COMMIT;'
} | PGOPTIONS='-c search_path=old' $PSQL

# 2. Fail loudly if the legacy objects the migration reads did not land in
#    `old`. A dump whose naming or qualification changed would otherwise surface
#    much later as a silently empty migration.
for tbl in users offerings courses exercises enrollments commits public_exercises; do
  found=$($PSQL -tAc "SELECT to_regclass('old.$tbl') IS NOT NULL;")
  if [ "$found" != "t" ]; then
    echo "error: legacy table old.$tbl is missing after loading the schema" >&2
    exit 1
  fi
done

# 3. Load the production data into `old`, atomically.
#    pg_dump qualifies names with the public schema and emits setval() calls
#    for the old sequences; remap the qualifiers to old and skip the setval
#    lines (old sequences are irrelevant to the new schema). The dump should
#    be taken with --disable-triggers so rows load regardless of table order.
{
  echo 'BEGIN;'
  sed \
    -e 's/^COPY public\./COPY old./' \
    -e 's/^INSERT INTO public\./INSERT INTO old./' \
    -e 's/^ALTER TABLE public\./ALTER TABLE old./' \
    -e '/^SELECT pg_catalog.setval(/d' \
    "$OLD_DATA"
  echo 'COMMIT;'
} | PGOPTIONS='-c search_path=old' $PSQL

# 4. Run the migration steps in order, in a single transaction: the validation
#    step can then roll the whole migration back instead of leaving half of it
#    applied. The scripts contain no top-level transaction control of their own
#    (their BEGIN/END pairs are plpgsql function bodies), so this is the only
#    transaction in play.
{
  echo 'BEGIN;'
  for f in "$SCRIPT_DIR"/*.sql; do
    echo "-- ==> $(basename "$f")"
    cat "$f"
  done
  echo 'COMMIT;'
} | $PSQL

echo
echo "Migration complete and committed. If you want the legacy-only tables"
echo "preserved, run 99-optional-archive-legacy.sql."
