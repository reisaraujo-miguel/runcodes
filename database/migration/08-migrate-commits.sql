-- Commits and their per-case results.
--
-- user_email -> user_id; commit_time -> created_at.
--
-- Legacy integer status -> new enum:
--   0 queued | 1 compiling | 2 compiled -> running | 3 running
--   4 uncompleted | 5 completed | 6 compilation_error | 7 pending
--   8 plagiarism | 9 server_error | 10 running | 11 initializing -> queued
--   anything else -> queued (fallback for unknown legacy values)
--
-- Dropped: hash, compiled_signal. aws_key becomes s3_key.
-- The legacy score had no bounds; the new one is CHECKed 0..100, so values
-- are clamped.

INSERT INTO public.commits (
  id, user_id, exercise_id, created_at, status, num_correct_cases, score,
  compiled, compilation_message, compilation_started, compilation_finished,
  compilation_error, ip, s3_key
)
SELECT
  c.id,
  um.new_id,
  c.exercise_id,
  _migration.to_tstz(c.commit_time),
  CASE c.status
    WHEN 0  THEN 'queued'
    WHEN 1  THEN 'compiling'
    WHEN 2  THEN 'running'           -- legacy 'compiled' is gone; treat as running
    WHEN 3  THEN 'running'
    WHEN 4  THEN 'uncompleted'
    WHEN 5  THEN 'completed'
    WHEN 6  THEN 'compilation_error'
    WHEN 7  THEN 'pending'
    WHEN 8  THEN 'plagiarism'
    WHEN 9  THEN 'server_error'
    WHEN 10 THEN 'running'
    WHEN 11 THEN 'queued'            -- legacy 'initializing' is gone; treat as queued
    ELSE 'queued'
  END::public.commit_status_t,
  COALESCE(c.corrects, 0),
  LEAST(100, GREATEST(0, COALESCE(c.score, 0)))::numeric(5, 2),
  c.compiled,
  c.compiled_message,
  _migration.to_tstz(c.compilation_started),
  _migration.to_tstz(c.compilation_finished),
  c.compiled_error,
  _migration.safe_inet(c.ip),
  c.aws_key
FROM old.commits c
LEFT JOIN _migration.users_map um ON um.old_email = c.user_email;

-- Per-case results. output -> user_output, error -> error_message.
--
-- Legacy semantics: any status other than 1 (correct) or 2
-- (bad_formatted_output) meant the process was killed by a signal (described
-- in status_message); the new schema represents that as 'killed_with_signal'.
--
-- The legacy table had no unique key on (commit, case), so duplicates are
-- deduplicated here; 12-validate.sql reports when this actually happened.

INSERT INTO public.commits_exercise_test_cases_results (
  commit_id, exercise_test_case_id, cpu_time, mem_usage,
  user_output, user_output_type, status, status_message, error_message
)
SELECT
  r.commit_id,
  r.exercise_case_id,
  r.cputime,
  r.memused,
  r.output,
  CASE r.output_type WHEN 1 THEN 'file' ELSE 'text' END::public.input_output_type_t,
  CASE r.status
    WHEN 1 THEN 'correct'
    WHEN 2 THEN 'bad_formatted_output'
    ELSE 'killed_with_signal'
  END::public.commit_exercise_test_case_status_t,
  r.status_message,
  r.error
FROM old.commits_exercise_cases r
ON CONFLICT DO NOTHING;
