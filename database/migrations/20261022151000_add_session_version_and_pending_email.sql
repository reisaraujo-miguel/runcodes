-- Add per-account session invalidation and a verified email-change flow.
--
-- token_version is carried in the session JWT. Bumping it invalidates every
-- session issued for the account, which is what makes password changes, admin
-- role changes and email changes revoke stale credentials.
--
-- pending_email holds a requested-but-unconfirmed new address. The current email
-- stays active until the link sent to pending_email is consumed, so an email
-- change cannot lock the owner out before they prove control of the new address.

-- Add "token_version" column to "users"
ALTER TABLE "users" ADD COLUMN "token_version" integer NOT NULL DEFAULT 1;

-- Add "pending_email" column to "users"
ALTER TABLE "users" ADD COLUMN "pending_email" text NULL;

-- At most one in-flight email change per address.
CREATE UNIQUE INDEX "users_pending_email_key" ON "users" ("pending_email")
  WHERE "pending_email" IS NOT NULL;
