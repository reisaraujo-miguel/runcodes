-- OPTIONAL: archive the legacy tables that have no equivalent in the new
-- schema (or whose migration is lossy), before dropping the `old` schema.
--
-- Note: the `old` schema itself keeps everything as long as it is not
-- dropped; this script only matters if you plan to drop it and still want
-- these rows around. Structures are copied with constraints; data is copied
-- verbatim (ids included).

-- This script copies rows without a natural key for every table, so a second
-- run would silently duplicate the archive. Fail early instead; drop the
-- schema explicitly if a partial run has to be redone.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.schemata WHERE schema_name = 'legacy'
  ) THEN
    RAISE EXCEPTION
      'legacy schema already exists; drop schema legacy cascade before rerunning 99-optional-archive-legacy.sql';
  END IF;
END
$$;

CREATE SCHEMA legacy;

CREATE TABLE legacy.courses (LIKE old.courses INCLUDING ALL);
INSERT INTO legacy.courses SELECT * FROM old.courses;

CREATE TABLE legacy.universities (LIKE old.universities INCLUDING ALL);
INSERT INTO legacy.universities SELECT * FROM old.universities;

CREATE TABLE legacy.blacklist_mails (LIKE old.blacklist_mails INCLUDING ALL);
INSERT INTO legacy.blacklist_mails SELECT * FROM old.blacklist_mails;

CREATE TABLE legacy.cria_schools (LIKE old.cria_schools INCLUDING ALL);
INSERT INTO legacy.cria_schools SELECT * FROM old.cria_schools;

CREATE TABLE legacy.cria_students (LIKE old.cria_students INCLUDING ALL);
INSERT INTO legacy.cria_students SELECT * FROM old.cria_students;

CREATE TABLE legacy.disk_reports (LIKE old.disk_reports INCLUDING ALL);
INSERT INTO legacy.disk_reports SELECT * FROM old.disk_reports;

CREATE TABLE legacy.droplets (LIKE old.droplets INCLUDING ALL);
INSERT INTO legacy.droplets SELECT * FROM old.droplets;

CREATE TABLE legacy.ganglia (LIKE old.ganglia INCLUDING ALL);
INSERT INTO legacy.ganglia SELECT * FROM old.ganglia;

CREATE TABLE legacy.jail_status (LIKE old.jail_status INCLUDING ALL);
INSERT INTO legacy.jail_status SELECT * FROM old.jail_status;

CREATE TABLE legacy.public_exercises (LIKE old.public_exercises INCLUDING ALL);
INSERT INTO legacy.public_exercises SELECT * FROM old.public_exercises;

CREATE TABLE legacy.questions (LIKE old.questions INCLUDING ALL);
INSERT INTO legacy.questions SELECT * FROM old.questions;

CREATE TABLE legacy.tickets (LIKE old.tickets INCLUDING ALL);
INSERT INTO legacy.tickets SELECT * FROM old.tickets;

-- Migrated lossily: the new rows dropped these columns' data.
CREATE TABLE legacy.allowed_files (LIKE old.allowed_files INCLUDING ALL);
INSERT INTO legacy.allowed_files SELECT * FROM old.allowed_files;

CREATE TABLE legacy.exercise_cases (LIKE old.exercise_cases INCLUDING ALL);
INSERT INTO legacy.exercise_cases SELECT * FROM old.exercise_cases;
