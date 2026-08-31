-- Allowed file types.
--
-- The old table stored per-language compile/run commands, which the new
-- compiler engine configures itself; those columns are dropped.
--
-- Existing new rows are reused whenever possible: an old row is matched by
-- (name, extension) first, then by extension alone (extension is UNIQUE in
-- the new schema and effectively identifies the language). Only rows without
-- any match are inserted, keeping the old id — the legacy sequence started at
-- 100, so ids never collide with the seeded defaults (1..25).
--
-- The old_id -> new_id mapping is recorded for the join table below.

WITH exact AS (
  SELECT af.id AS old_id, aft.id AS new_id
  FROM old.allowed_files af
  JOIN public.allowed_file_types aft
    ON aft.name = af.name AND aft.extension = af.extension
),
by_extension AS (
  SELECT af.id AS old_id, aft.id AS new_id
  FROM old.allowed_files af
  JOIN public.allowed_file_types aft
    ON aft.extension = af.extension
),
inserted AS (
  INSERT INTO public.allowed_file_types (id, name, extension, is_compilable, is_available)
  SELECT DISTINCT ON (af.extension)
         af.id,
         af.name,
         af.extension,
         COALESCE(af.compilable, TRUE),
         COALESCE(af.available, TRUE)
  FROM old.allowed_files af
  WHERE NOT EXISTS (
    SELECT 1 FROM public.allowed_file_types aft WHERE aft.extension = af.extension
  )
  ORDER BY af.extension, af.id
  ON CONFLICT DO NOTHING
  RETURNING id
)
INSERT INTO _migration.file_types_map (old_id, new_id)
SELECT
  af.id,
  COALESCE(exact.new_id, by_extension.new_id, inserted.id)
FROM old.allowed_files af
LEFT JOIN exact ON exact.old_id = af.id
LEFT JOIN by_extension ON by_extension.old_id = af.id
LEFT JOIN inserted ON inserted.id = af.id
WHERE COALESCE(exact.new_id, by_extension.new_id, inserted.id) IS NOT NULL;

-- Exercises x allowed file types.
-- Legacy allowed_file_id was nullable; rows without a file type are dropped.

INSERT INTO public.exercises_allowed_file_types (exercise_id, allowed_file_type_id)
SELECT DISTINCT
  afe.exercise_id,
  ftm.new_id
FROM old.allowed_files_exercises afe
JOIN _migration.file_types_map ftm ON ftm.old_id = afe.allowed_file_id
ON CONFLICT DO NOTHING;
