#!/bin/bash
#
# Sets the password of the admin account created by schema/03-seed-data.sql.
#
# The seed starts that account without a usable password, deliberately: a bcrypt
# hash committed to a public repository is a published credential, and only the
# operator can know what the password should be. Until this script runs, nobody
# can log in as the seeded admin.
#
# Usage against the compose deployment (the same variables the services use):
#
#   RUNCODES_ADMIN_PASSWORD='...' \
#     PGHOST=localhost PGPORT=5432 PGUSER=runcodes PGDATABASE=runcodes \
#     PGPASSWORD="$RUNCODES_DB_PASSWORD" ./database/bootstrap-admin.sh
#
# The password is hashed by the server (bcrypt cost 12 — the cost the backend
# uses), so the plaintext is only ever the argument of a single statement: run
# this on a connection you are willing to see the SQL on, not one that logs every
# statement. bcrypt reads at most 72 bytes of it.

set -eu

password="${RUNCODES_ADMIN_PASSWORD:?set RUNCODES_ADMIN_PASSWORD to the password to set}"
email="${RUNCODES_ADMIN_EMAIL:-admin@admin.com}"

psql --no-psqlrc --set ON_ERROR_STOP=1 \
	--set email="$email" --set password="$password" <<'EOSQL'
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Hand the email to the block below through a session setting: psql quotes a
-- variable as a SQL literal, but does not substitute inside the dollar quotes.
SELECT set_config('runcodes.bootstrap_email', :'email', false);

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1 FROM users WHERE email = current_setting('runcodes.bootstrap_email')
	) THEN
		RAISE EXCEPTION 'no user with email %',
			current_setting('runcodes.bootstrap_email');
	END IF;
END
$$;

UPDATE users
   SET password_hash = crypt(:'password', gen_salt('bf', 12)),
       updated_at    = NOW()
 WHERE email = :'email';
EOSQL

echo "bootstrap-admin: password set for ${email}"
