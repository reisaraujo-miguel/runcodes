-- Offerings.
--
-- The old model split offerings across two tables (`courses` + `offerings`);
-- the new one has a single table. The display name is composed as:
--     <code> - <title> (<year>/<term>)
--
-- owner_id is backfilled later (04-migrate-offering-owners.sql) from the first
-- professor enrollment.
--
-- Dropped columns with no new equivalent: classroom, max_students,
-- max_exercises. Old offerings have no creation timestamp; created_at and
-- updated_at are set to the migration time.
--
-- enrollment_code is UNIQUE in the new schema but was not in the old one;
-- duplicates are dropped (set to NULL) instead of failing the migration.

WITH duplicated_codes AS (
  SELECT enrollment_code
  FROM old.offerings
  WHERE enrollment_code IS NOT NULL
  GROUP BY enrollment_code
  HAVING count(*) > 1
)
INSERT INTO public.offerings (
  id, name, owner_id, end_date, visible_to_enroll, enrollment_code,
  description, created_at, updated_at
)
SELECT
  o.id,
  c.code || ' - ' || c.title || ' (' || o.year || '/' || o.term || ')',
  NULL, -- 04-migrate-offering-owners.sql
  _migration.to_tstz(o.end_date::timestamp),
  COALESCE(o.visible_to_enroll, TRUE),
  CASE WHEN d.enrollment_code IS NULL THEN o.enrollment_code END,
  NULL, -- no equivalent in the old schema
  NOW(),
  NOW()
FROM old.offerings o
JOIN old.courses c ON c.id = o.course_id
LEFT JOIN duplicated_codes d ON d.enrollment_code = o.enrollment_code;
