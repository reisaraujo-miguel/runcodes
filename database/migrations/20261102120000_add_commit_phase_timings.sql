-- Phase timings and the per-case wall clock.
--
-- A submission now reports where its time went: queue, workspace preparation and
-- its sub-steps, image ensure, container create/start, startup, compile, run,
-- grading and artifact building. The compile row carries the monitor's wall and
-- CPU times; the run rows carry the per-case wall time in the results table.
--
-- Rows arrive from the judge's `timings` event through the backend, so the judge
-- role does not need access to this table.
ALTER TABLE commits_exercise_test_cases_results
  ALTER COLUMN mem_usage TYPE bigint,
  ADD COLUMN wall_time numeric(15, 4);

CREATE TYPE commit_phase_t AS ENUM (
  'queue',
  'workspace_prepare',
  'source_download',
  'source_resolve',
  'compilation_files',
  'test_cases',
  'test_case_files',
  'container_config',
  'image_ensure',
  'container_create',
  'container_start',
  'startup',
  'compile',
  'run',
  'grade',
  'artifact'
);

CREATE TABLE commit_phase_timings (
  commit_id int NOT NULL REFERENCES commits (id) ON DELETE CASCADE,
  phase commit_phase_t NOT NULL,
  duration_ms int NOT NULL CHECK (duration_ms >= 0),
  cpu_ms int CHECK (cpu_ms IS NULL OR cpu_ms >= 0),
  mem_peak_bytes bigint CHECK (mem_peak_bytes IS NULL OR mem_peak_bytes >= 0),
  started_at TIMESTAMP WITH TIME ZONE,
  finished_at TIMESTAMP WITH TIME ZONE,
  source text DEFAULT 'judge' NOT NULL CHECK (source IN ('judge', 'monitor', 'harness')),
  detail jsonb,
  PRIMARY KEY (commit_id, phase)
);

CREATE INDEX idx_commit_phase_timings_phase ON commit_phase_timings (phase);

ALTER TABLE "commit_phase_timings" ENABLE ROW LEVEL SECURITY;
CREATE POLICY "runcodes_app_only" ON "commit_phase_timings" TO PUBLIC
  USING (current_user = 'runcodes_app')
  WITH CHECK (current_user = 'runcodes_app');
