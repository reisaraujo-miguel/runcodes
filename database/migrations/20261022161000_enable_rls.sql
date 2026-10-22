-- Row-level security at the service-role boundary.
--
-- The backend connects as `runcodes_app` and still relies on the handlers for
-- per-user authorization; these policies do not filter individual rows. What
-- they add is a database-enforced boundary around the judge: the judge role is
-- denied every table in this list even if a future migration or manual grant is
-- too broad. The backend and the table owner (`runcodes`, used by migrations and
-- the seed) keep their existing full access.
--
-- Tables the judge needs (commits, exercises and the test-case tables) are not
-- enabled here, because the judge must read/update rows across all users.

ALTER TABLE "users" ENABLE ROW LEVEL SECURITY;
CREATE POLICY "runcodes_app_only" ON "users" TO PUBLIC
  USING (current_user = 'runcodes_app')
  WITH CHECK (current_user = 'runcodes_app');

ALTER TABLE "email_verification_tokens" ENABLE ROW LEVEL SECURITY;
CREATE POLICY "runcodes_app_only" ON "email_verification_tokens" TO PUBLIC
  USING (current_user = 'runcodes_app')
  WITH CHECK (current_user = 'runcodes_app');

ALTER TABLE "password_reset_tokens" ENABLE ROW LEVEL SECURITY;
CREATE POLICY "runcodes_app_only" ON "password_reset_tokens" TO PUBLIC
  USING (current_user = 'runcodes_app')
  WITH CHECK (current_user = 'runcodes_app');

ALTER TABLE "enrollments" ENABLE ROW LEVEL SECURITY;
CREATE POLICY "runcodes_app_only" ON "enrollments" TO PUBLIC
  USING (current_user = 'runcodes_app')
  WITH CHECK (current_user = 'runcodes_app');

ALTER TABLE "offerings" ENABLE ROW LEVEL SECURITY;
CREATE POLICY "runcodes_app_only" ON "offerings" TO PUBLIC
  USING (current_user = 'runcodes_app')
  WITH CHECK (current_user = 'runcodes_app');

ALTER TABLE "allowed_file_types" ENABLE ROW LEVEL SECURITY;
CREATE POLICY "runcodes_app_only" ON "allowed_file_types" TO PUBLIC
  USING (current_user = 'runcodes_app')
  WITH CHECK (current_user = 'runcodes_app');

ALTER TABLE "exercises_attached_files" ENABLE ROW LEVEL SECURITY;
CREATE POLICY "runcodes_app_only" ON "exercises_attached_files" TO PUBLIC
  USING (current_user = 'runcodes_app')
  WITH CHECK (current_user = 'runcodes_app');

ALTER TABLE "exercises_allowed_file_types" ENABLE ROW LEVEL SECURITY;
CREATE POLICY "runcodes_app_only" ON "exercises_allowed_file_types" TO PUBLIC
  USING (current_user = 'runcodes_app')
  WITH CHECK (current_user = 'runcodes_app');

ALTER TABLE "commits_exercise_test_cases_results" ENABLE ROW LEVEL SECURITY;
CREATE POLICY "runcodes_app_only" ON "commits_exercise_test_cases_results" TO PUBLIC
  USING (current_user = 'runcodes_app')
  WITH CHECK (current_user = 'runcodes_app');

ALTER TABLE "platform_settings" ENABLE ROW LEVEL SECURITY;
CREATE POLICY "runcodes_app_only" ON "platform_settings" TO PUBLIC
  USING (current_user = 'runcodes_app')
  WITH CHECK (current_user = 'runcodes_app');

ALTER TABLE "alerts" ENABLE ROW LEVEL SECURITY;
CREATE POLICY "runcodes_app_only" ON "alerts" TO PUBLIC
  USING (current_user = 'runcodes_app')
  WITH CHECK (current_user = 'runcodes_app');

ALTER TABLE "system_logs" ENABLE ROW LEVEL SECURITY;
CREATE POLICY "runcodes_app_only" ON "system_logs" TO PUBLIC
  USING (current_user = 'runcodes_app')
  WITH CHECK (current_user = 'runcodes_app');

ALTER TABLE "user_messages" ENABLE ROW LEVEL SECURITY;
CREATE POLICY "runcodes_app_only" ON "user_messages" TO PUBLIC
  USING (current_user = 'runcodes_app')
  WITH CHECK (current_user = 'runcodes_app');
