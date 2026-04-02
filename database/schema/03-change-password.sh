#!/bin/bash
set -e

psql_rc_password="${RUNCODES_PASSWORD:-asdasd33}"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
	ALTER USER runcodes WITH ENCRYPTED PASSWORD '$psql_rc_password';
EOSQL
