#!/bin/bash
set -e

if [ -z "${RUNCODES_PASSWORD}" ]; then
	echo "ERROR: RUNCODES_PASSWORD environment variable is not set." >&2
	echo "       Set it before starting the database container." >&2
	exit 1
fi

psql -v ON_ERROR_STOP=1 -v rc_password="$RUNCODES_PASSWORD" --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-'EOSQL'
	    ALTER USER runcodes WITH ENCRYPTED PASSWORD :'rc_password';
EOSQL
