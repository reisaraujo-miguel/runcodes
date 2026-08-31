-- Offerings owners.
--
-- Backfill offerings.owner_id with the first professor of each offering.
-- "First" = the earliest enrollment row (legacy enrollments.id is
-- insertion-ordered). Offerings without any professor keep owner_id = NULL.

UPDATE public.offerings o
SET owner_id = owners.user_id
FROM (
  SELECT DISTINCT ON (e.offering_id)
         e.offering_id,
         um.new_id AS user_id
  FROM old.enrollments e
  JOIN _migration.users_map um ON um.old_email = e.user_email
  WHERE e.role = 2   -- professor
  ORDER BY e.offering_id, e.id
) owners
WHERE o.id = owners.offering_id;
