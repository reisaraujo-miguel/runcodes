-- Exercises.
--
-- user_email -> creator_id (NULL if the legacy user is somehow missing).
-- type is always 'programming' (the only value in the new enum).
--
-- Dropped columns: cases_change, public, markdown.
-- open_date was nullable in the old schema but is NOT NULL in the new one, so
-- it falls back to the deadline when missing.
-- Old exercises have no creation timestamp; created_at/updated_at are set to
-- the migration time.

INSERT INTO public.exercises (
  id, offering_id, creator_id, title, description, deadline, type,
  open_date, show_before_open_date, removed, ghost, real_id,
  created_at, updated_at
)
SELECT
  e.id,
  e.offering_id,
  um.new_id,
  e.title,
  e.description,
  _migration.to_tstz(e.deadline),
  'programming'::public.exercise_type_t,
  COALESCE(_migration.to_tstz(e.open_date), _migration.to_tstz(e.deadline)),
  COALESCE(e.show_before_opening, FALSE),
  COALESCE(e.removed, FALSE),
  COALESCE(e.ghost, FALSE),
  e.real_id,
  NOW(),
  NOW()
FROM old.exercises e
LEFT JOIN _migration.users_map um ON um.old_email = e.user_email;
