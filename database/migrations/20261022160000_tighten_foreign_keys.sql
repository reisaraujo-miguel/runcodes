-- Make the foreign keys the application always writes non-nullable. They were
-- declared nullable in the baseline, which let a bug or a manual insert create
-- rows detached from their offering/exercise/user even though every code path
-- supplies them.
--
-- Intentionally left nullable:
--   alerts.offering_id / alerts.user_id       (global or targeted alerts)
--   system_logs.user_id                       (system-originated log entries)
--   user_messages.user_id                     (broadcast messages)
--
-- There are no production deployments yet, so this migration does not need a
-- backfill. A deployment with legacy nulls must clean them up first; PostgreSQL
-- names the offending rows when ALTER ... SET NOT NULL fails.

ALTER TABLE offerings ALTER COLUMN owner_id SET NOT NULL;

ALTER TABLE exercises ALTER COLUMN offering_id SET NOT NULL;
ALTER TABLE exercises ALTER COLUMN creator_id SET NOT NULL;

ALTER TABLE exercises_attached_files ALTER COLUMN exercise_id SET NOT NULL;
ALTER TABLE exercises_compilation_files ALTER COLUMN exercise_id SET NOT NULL;
ALTER TABLE exercises_test_cases ALTER COLUMN exercise_id SET NOT NULL;
ALTER TABLE exercises_test_cases_files ALTER COLUMN exercise_test_case_id SET NOT NULL;

ALTER TABLE exercises_allowed_file_types ALTER COLUMN exercise_id SET NOT NULL;
ALTER TABLE exercises_allowed_file_types ALTER COLUMN allowed_file_type_id SET NOT NULL;

ALTER TABLE commits ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE commits ALTER COLUMN exercise_id SET NOT NULL;

ALTER TABLE commits_exercise_test_cases_results ALTER COLUMN commit_id SET NOT NULL;
ALTER TABLE commits_exercise_test_cases_results ALTER COLUMN exercise_test_case_id SET NOT NULL;
