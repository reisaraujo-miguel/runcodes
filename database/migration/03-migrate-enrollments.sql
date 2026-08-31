-- Enrollments.
--
-- user_email -> user_id via the users map; role int -> enum:
--   0 student | 1 monitor | 2 professor
--
-- The old surrogate id is dropped (the new PK is (user_id, offering_id)).
-- Old enrollments have no creation timestamp; created_at/updated_at are set
-- to the migration time.

INSERT INTO public.enrollments (
  user_id, offering_id, role, banned, created_at, updated_at
)
SELECT
  um.new_id,
  e.offering_id,
  CASE e.role
    WHEN 0 THEN 'student'
    WHEN 1 THEN 'monitor'
    WHEN 2 THEN 'professor'
    ELSE 'student'   -- unreachable (legacy CHECK allowed only 0..2)
  END::public.enrollment_role_t,
  COALESCE(e.banned, FALSE),
  NOW(),
  NOW()
FROM old.enrollments e
JOIN _migration.users_map um ON um.old_email = e.user_email
ON CONFLICT (user_id, offering_id) DO NOTHING;
