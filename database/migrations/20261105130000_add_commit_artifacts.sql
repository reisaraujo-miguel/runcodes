-- Per-case generated-file upload state.
--
-- The judge uploads each case's files as the suite advances and announces the
-- outcome with an `artifact` event. Persisting the state lets a reloading page
-- show a pending upload, or explain why a download will never exist (the commit
-- artifact budget was reached, object storage failed, the upload was
-- interrupted by the end of the run).
CREATE TYPE commit_artifact_state_t AS ENUM ('pending', 'ready', 'failed');

CREATE TABLE commit_test_case_artifacts (
  commit_id int NOT NULL REFERENCES commits (id) ON DELETE CASCADE,
  exercise_test_case_id int NOT NULL REFERENCES exercises_test_cases (id) ON DELETE CASCADE,
  state commit_artifact_state_t NOT NULL,
  reason text,
  limit_bytes bigint CHECK (limit_bytes IS NULL OR limit_bytes > 0),
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
  PRIMARY KEY (commit_id, exercise_test_case_id)
);

ALTER TABLE "commit_test_case_artifacts" ENABLE ROW LEVEL SECURITY;
CREATE POLICY "runcodes_app_only" ON "commit_test_case_artifacts" TO PUBLIC
  USING (current_user = 'runcodes_app')
  WITH CHECK (current_user = 'runcodes_app');
