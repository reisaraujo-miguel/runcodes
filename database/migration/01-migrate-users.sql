-- Users.
--
-- The old table was keyed by email; the new one uses a surrogate id, so new
-- ids are assigned here and the email -> id mapping is recorded in
-- _migration.users_map for every table that used to reference users.email.
--
-- role mapping (legacy users.type):
--   0 student | 1 assistant professor (unused in practice; demoted to
--   student) | 2 professor | 3 admin | 4 dev
--
-- org_id only gets a value when the legacy identifier is non-NULL and unique
-- across migrated users, because users.org_id has a UNIQUE constraint and the
-- old column had none. Rows with duplicate identifiers get org_id = NULL.
--
-- Passwords: the old hashes (SHA-1 of a fixed salt + password) cannot be
-- converted to bcrypt offline, so they are stored prefixed with
-- 'legacy-sha1$'. The backend detects the prefix on login, verifies the old
-- hash and rehashes with bcrypt (lazy migration).
--
-- If a row with the same email already exists (e.g. the seeded admin user),
-- it is kept as-is, including its bcrypt password_hash.

WITH duplicated_ids AS (
  SELECT identifier
  FROM old.users
  WHERE identifier IS NOT NULL
  GROUP BY identifier
  HAVING count(*) > 1
)
INSERT INTO public.users (
  name, email, org_id, password_hash, role, confirmed, created_at, updated_at
)
SELECT
  u.name,
  u.email,
  CASE WHEN d.identifier IS NULL THEN u.identifier END,
  'legacy-sha1$' || u.password,
  CASE u.type
    WHEN 0 THEN 'student'
    WHEN 1 THEN 'student'   -- legacy 'assistant professor', unused; demoted
    WHEN 2 THEN 'professor'
    WHEN 3 THEN 'admin'
    WHEN 4 THEN 'dev'
    ELSE 'student'          -- unreachable (legacy CHECK allowed only 0..4)
  END::public.user_t,
  COALESCE(u.confirmed, FALSE),
  _migration.to_tstz(u.creation),
  _migration.to_tstz(u.creation)
FROM old.users u
LEFT JOIN duplicated_ids d ON d.identifier = u.identifier
ON CONFLICT (email) DO NOTHING;

INSERT INTO _migration.users_map (old_email, new_id)
SELECT u.email, nu.id
FROM old.users u
JOIN public.users nu ON nu.email = u.email;
