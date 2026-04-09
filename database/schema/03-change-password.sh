#!/bin/bash
set -eu

POSTGRES_PASSWORD="${POSTGRES_PASSWORD:?POSTGRES_PASSWORD must be set}"

psql -v ON_ERROR_STOP=1 \
	--username "$POSTGRES_USER" \
	--dbname "$POSTGRES_DB" \
	--set rc_password="$POSTGRES_PASSWORD" <<-'EOSQL'
		ALTER ROLE runcodes WITH ENCRYPTED PASSWORD :'rc_password';
	EOSQL
