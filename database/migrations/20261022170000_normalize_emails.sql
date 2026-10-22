-- Make email uniqueness and lookup case-insensitive.
--
-- Email is treated as case-insensitive throughout the application: a user who
-- signed up as Ada@Example.com must be able to sign in as ada@example.com. The
-- original UNIQUE constraints were case-sensitive, so two case variants could
-- become separate accounts. Normalise the stored addresses first, aborting
-- rather than silently choosing one of two colliding accounts, then add the
-- functional unique indexes the services rely on.

DO $$
DECLARE
    duplicate_count integer;
BEGIN
    SELECT count(*) INTO duplicate_count
      FROM (
          SELECT lower(email)
            FROM users
           GROUP BY lower(email)
          HAVING count(*) > 1
      ) duplicates;

    IF duplicate_count > 0 THEN
        RAISE EXCEPTION
            'users contain % case-insensitive duplicate email address(es); resolve them before applying this migration',
            duplicate_count;
    END IF;

    SELECT count(*) INTO duplicate_count
      FROM (
          SELECT lower(pending_email)
            FROM users
           WHERE pending_email IS NOT NULL
           GROUP BY lower(pending_email)
          HAVING count(*) > 1
      ) duplicates;

    IF duplicate_count > 0 THEN
        RAISE EXCEPTION
            'users contain % case-insensitive duplicate pending email address(es); resolve them before applying this migration',
            duplicate_count;
    END IF;
END
$$;

UPDATE users
   SET email = lower(email),
       pending_email = lower(pending_email),
       updated_at = now()
 WHERE email <> lower(email)
    OR (pending_email IS NOT NULL AND pending_email <> lower(pending_email));

CREATE UNIQUE INDEX "users_email_lower_key" ON "users" (lower("email"));
CREATE UNIQUE INDEX "users_pending_email_lower_key" ON "users" (lower("pending_email"))
  WHERE "pending_email" IS NOT NULL;
