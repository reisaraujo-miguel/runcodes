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

PSQL="psql -v ON_ERROR_STOP=1 -q"

# 1. Create the `old` schema (fails if it already exists: the migration is
#    meant to run against a fresh copy of the new database).
$PSQL -c 'CREATE SCHEMA old;'

# 2. Load the old schema into `old`.
#    The dump targets the `public` schema and creates a role/database; strip
#    those statements and route every unqualified CREATE into `old` via
#    search_path (the sed public->old remap also covers the GRANT/REVOKE
#    statements).
sed \
  -e 's/public/old/g' \
  -e '/^CREATE USER /d' \
  -e '/^CREATE DATABASE /d' \
  -e '/^GRANT ALL ON DATABASE /d' \
  -e '/^\\c /d' \
  "$OLD_SCHEMA" | PGOPTIONS='-c search_path=old' $PSQL

# 3. Load the production data into `old`.
#    pg_dump qualifies names with the public schema and emits setval() calls
#    for the old sequences; remap the qualifiers to old and skip the setval
#    lines (old sequences are irrelevant to the new schema). The dump should
#    be taken with --disable-triggers so rows load regardless of table order.
sed \
  -e 's/^COPY public\./COPY old./' \
  -e 's/^INSERT INTO public\./INSERT INTO old./' \
  -e 's/^ALTER TABLE public\./ALTER TABLE old./' \
  -e '/^SELECT pg_catalog.setval(/d' \
  "$OLD_DATA" | PGOPTIONS='-c search_path=old' $PSQL

# 4. Run the migration steps in order.
for f in "$SCRIPT_DIR"/*.sql; do
  echo "==> $(basename "$f")"
  $PSQL -f "$f"
done

echo
echo "Migration complete. Review the 12-validate.sql output above;"
echo "if you want the legacy-only tables preserved, run 99-optional-archive-legacy.sql."
