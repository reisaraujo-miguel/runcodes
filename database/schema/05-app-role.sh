#!/bin/bash
#
# Creates the role the services log in as, and grants it what it needs.
#
# The backend and the judge must not connect as the database owner: `runcodes` is
# the superuser the postgres image creates from POSTGRES_USER, so a SQL injection
# in either service would be able to read every database on the host, disable
# constraints, and drop the schema. This role owns nothing and cannot change the
# schema; a CI check asserts exactly that (see .github/workflows/tests.yml).
#
# Runs as the database image's init step (as POSTGRES_USER), and is idempotent so
# it can also be re-run against a live database to rotate the password:
#
#   RUNCODES_DB_APP_PASSWORD='...' PGHOST=localhost PGPORT=5432 PGUSER=runcodes \
#     PGPASSWORD="$RUNCODES_DB_PASSWORD" PGDATABASE=runcodes \
#     ./database/schema/05-app-role.sh
#
# Keep it after 02-schema.sql: the grants cover the tables that exist by then, and
# ALTER DEFAULT PRIVILEGES covers the ones a later migration adds.

set -eu

app_password="${RUNCODES_DB_APP_PASSWORD:?RUNCODES_DB_APP_PASSWORD must be set}"

db_user="${POSTGRES_USER:-${PGUSER:-postgres}}"
db_name="${POSTGRES_DB:-${PGDATABASE:-${db_user}}}"

psql -v ON_ERROR_STOP=1 \
	--username "${db_user}" --dbname "${db_name}" \
	--set app_password="${app_password}" <<'EOSQL'
-- Create the role unless it is already there. \gexec runs the row the query
-- returns as SQL, which is how this avoids a DO block around a utility command.
SELECT 'CREATE ROLE runcodes_app'
 WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'runcodes_app') \gexec

-- Attributes and password are set unconditionally, so re-running this file
-- completes a role that was created by an older version of it, or rotates the
-- password.
ALTER ROLE runcodes_app WITH
	LOGIN
	NOSUPERUSER
	NOCREATEDB
	NOCREATEROLE
	NOINHERIT
	NOBYPASSRLS
	PASSWORD :'app_password';

-- Row access, and nothing else: no ownership, and no CREATE on the schema
-- (which PostgreSQL 15+ stopped granting to PUBLIC too).
REVOKE ALL ON SCHEMA public FROM runcodes_app;
GRANT USAGE ON SCHEMA public TO runcodes_app;

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO runcodes_app;
-- The identity columns the services insert into need their sequences.
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO runcodes_app;

-- Migrations run as `runcodes`, the owner. These defaults make every table and
-- sequence a migration creates carry the same grants, so a migration never has
-- to remember them (see database/migration/README.md).
ALTER DEFAULT PRIVILEGES FOR ROLE runcodes IN SCHEMA public
	GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO runcodes_app;
ALTER DEFAULT PRIVILEGES FOR ROLE runcodes IN SCHEMA public
	GRANT USAGE, SELECT ON SEQUENCES TO runcodes_app;
EOSQL

echo "app-role: runcodes_app reads and writes rows; it cannot change the schema"
