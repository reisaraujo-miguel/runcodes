-- Migration helpers and mapping tables.
--
-- Assumptions shared by every script in this folder:
--   * the new schema lives in `public` (where the new app reads from)
--   * the old production schema + data have been restored into the `old` schema
--   * scripts run as the database owner (`runcodes`)
--
-- Each script may be re-run on a fresh database; this helper is idempotent.

DROP SCHEMA IF EXISTS _migration CASCADE;
CREATE SCHEMA _migration;

-- The old app stored naive `timestamp` values in America/Sao_Paulo local time.
-- The new schema uses timestamptz everywhere, so old timestamps are
-- interpreted with that zone.
CREATE OR REPLACE FUNCTION _migration.to_tstz(ts timestamp)
RETURNS timestamptz
LANGUAGE sql IMMUTABLE PARALLEL SAFE
AS $$ SELECT ts AT TIME ZONE 'America/Sao_Paulo' $$;

-- Cast free-form text IPs to inet, returning NULL when the old value is not a
-- valid address (the old schema used varchar columns, so garbage is possible).
CREATE OR REPLACE FUNCTION _migration.safe_inet(s text)
RETURNS inet
LANGUAGE plpgsql IMMUTABLE
AS $$
BEGIN
  IF s IS NULL OR btrim(s) = '' THEN
    RETURN NULL;
  END IF;
  RETURN s::inet;
EXCEPTION WHEN OTHERS THEN
  RETURN NULL;
END;
$$;

-- email (old users PK) -> new users.id, filled by 01-migrate-users.sql
CREATE TABLE _migration.users_map (
  old_email text PRIMARY KEY,
  new_id int NOT NULL
);

-- old allowed_files.id -> new allowed_file_types.id, filled by
-- 06-migrate-allowed-file-types.sql
CREATE TABLE _migration.file_types_map (
  old_id int PRIMARY KEY,
  new_id int NOT NULL
);
