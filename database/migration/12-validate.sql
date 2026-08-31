-- Migration validation.
--
-- 1. Row-count assertions: every old row must have arrived in the new schema.
--    Failing here means a step above silently dropped data (e.g. deduped
--    rows) and must be investigated before going to production.
-- 2. Foreign-key orphan checks.
-- 3. Informative summaries (statuses/roles distributions) to eyeball.

DO $$
DECLARE
  v_old bigint;
  v_new bigint;
BEGIN
  -- users: compared via the map, since the new table may have seeded rows.
  SELECT count(*) INTO v_old FROM old.users;
  SELECT count(*) INTO v_new FROM _migration.users_map;
  IF v_old <> v_new THEN
    RAISE EXCEPTION 'users: % old rows but only % mapped', v_old, v_new;
  END IF;

  SELECT count(*) INTO v_old FROM old.offerings;
  SELECT count(*) INTO v_new FROM public.offerings;
  IF v_old <> v_new THEN
    RAISE EXCEPTION 'offerings: % old vs % new', v_old, v_new;
  END IF;

  SELECT count(*) INTO v_old FROM old.enrollments;
  SELECT count(*) INTO v_new FROM public.enrollments;
  IF v_old <> v_new THEN
    RAISE EXCEPTION 'enrollments: % old vs % new (duplicates were dropped?)', v_old, v_new;
  END IF;

  SELECT count(*) INTO v_old FROM old.exercises;
  SELECT count(*) INTO v_new FROM public.exercises;
  IF v_old <> v_new THEN
    RAISE EXCEPTION 'exercises: % old vs % new', v_old, v_new;
  END IF;

  SELECT count(*) INTO v_old FROM old.commits;
  SELECT count(*) INTO v_new FROM public.commits;
  IF v_old <> v_new THEN
    RAISE EXCEPTION 'commits: % old vs % new', v_old, v_new;
  END IF;

  SELECT count(*) INTO v_old FROM old.alerts;
  SELECT count(*) INTO v_new FROM public.alerts;
  IF v_old <> v_new THEN
    RAISE EXCEPTION 'alerts: % old vs % new', v_old, v_new;
  END IF;

  SELECT count(*) INTO v_old FROM old.exercise_cases;
  SELECT count(*) INTO v_new FROM public.exercises_test_cases;
  IF v_old <> v_new THEN
    RAISE EXCEPTION 'exercise_cases: % old vs % new', v_old, v_new;
  END IF;

  SELECT count(*) INTO v_old FROM old.commits_exercise_cases;
  SELECT count(*) INTO v_new FROM public.commits_exercise_test_cases_results;
  IF v_old <> v_new THEN
    RAISE EXCEPTION
      'commits_exercise_cases: % old vs % new (legacy duplicates were dropped; review them)',
      v_old, v_new;
  END IF;

  SELECT count(*) INTO v_old FROM old.exercise_files;
  SELECT count(*) INTO v_new FROM public.exercises_attached_files;
  IF v_old <> v_new THEN
    RAISE EXCEPTION 'exercise_files: % old vs % new', v_old, v_new;
  END IF;

  SELECT count(*) INTO v_old FROM old.compilation_files;
  SELECT count(*) INTO v_new FROM public.exercises_compilation_files;
  IF v_old <> v_new THEN
    RAISE EXCEPTION 'compilation_files: % old vs % new', v_old, v_new;
  END IF;

  SELECT count(*) INTO v_old FROM old.exercise_case_files;
  SELECT count(*) INTO v_new FROM public.exercises_test_cases_files;
  IF v_old <> v_new THEN
    RAISE EXCEPTION 'exercise_case_files: % old vs % new', v_old, v_new;
  END IF;

  SELECT count(*) INTO v_old FROM old.logs;
  SELECT count(*) INTO v_new FROM public.system_logs;
  IF v_old <> v_new THEN
    RAISE EXCEPTION 'logs: % old vs % new', v_old, v_new;
  END IF;

  SELECT count(*) INTO v_old FROM old.mail_logs;
  SELECT count(*) INTO v_new FROM public.mail_logs;
  IF v_old <> v_new THEN
    RAISE EXCEPTION 'mail_logs: % old vs % new', v_old, v_new;
  END IF;

  SELECT count(*) INTO v_old FROM old.messages;
  SELECT count(*) INTO v_new FROM public.user_messages;
  IF v_old <> v_new THEN
    RAISE EXCEPTION 'messages: % old vs % new', v_old, v_new;
  END IF;

  -- allowed file types: every old row must have been mapped to a new one.
  SELECT count(*) INTO v_old FROM old.allowed_files;
  SELECT count(*) INTO v_new FROM _migration.file_types_map;
  IF v_old <> v_new THEN
    RAISE EXCEPTION 'allowed_files: % old rows but only % mapped', v_old, v_new;
  END IF;

  SELECT count(*) INTO v_old
  FROM (SELECT DISTINCT exercise_id, allowed_file_id
        FROM old.allowed_files_exercises
        WHERE allowed_file_id IS NOT NULL) s;
  SELECT count(*) INTO v_new FROM public.exercises_allowed_file_types;
  IF v_old <> v_new THEN
    RAISE EXCEPTION 'allowed_files_exercises: % old pairs vs % new rows', v_old, v_new;
  END IF;
END;
$$;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM public.enrollments e
    LEFT JOIN public.users u ON u.id = e.user_id WHERE u.id IS NULL
  ) THEN RAISE EXCEPTION 'orphan enrollments.user_id'; END IF;

  IF EXISTS (
    SELECT 1 FROM public.enrollments e
    LEFT JOIN public.offerings o ON o.id = e.offering_id WHERE o.id IS NULL
  ) THEN RAISE EXCEPTION 'orphan enrollments.offering_id'; END IF;

  IF EXISTS (
    SELECT 1 FROM public.exercises e
    LEFT JOIN public.offerings o ON o.id = e.offering_id WHERE o.id IS NULL
  ) THEN RAISE EXCEPTION 'orphan exercises.offering_id'; END IF;

  IF EXISTS (
    SELECT 1 FROM public.exercises e
    LEFT JOIN public.users u ON u.id = e.creator_id
    WHERE e.creator_id IS NOT NULL AND u.id IS NULL
  ) THEN RAISE EXCEPTION 'orphan exercises.creator_id'; END IF;

  IF EXISTS (
    SELECT 1 FROM public.commits c
    LEFT JOIN public.users u ON u.id = c.user_id WHERE u.id IS NULL
  ) THEN RAISE EXCEPTION 'orphan commits.user_id'; END IF;

  IF EXISTS (
    SELECT 1 FROM public.commits c
    LEFT JOIN public.exercises e ON e.id = c.exercise_id WHERE e.id IS NULL
  ) THEN RAISE EXCEPTION 'orphan commits.exercise_id'; END IF;

  IF EXISTS (
    SELECT 1 FROM public.commits_exercise_test_cases_results r
    LEFT JOIN public.commits c ON c.id = r.commit_id WHERE c.id IS NULL
  ) THEN RAISE EXCEPTION 'orphan results.commit_id'; END IF;

  IF EXISTS (
    SELECT 1 FROM public.commits_exercise_test_cases_results r
    LEFT JOIN public.exercises_test_cases t ON t.id = r.exercise_test_case_id
    WHERE t.id IS NULL
  ) THEN RAISE EXCEPTION 'orphan results.exercise_test_case_id'; END IF;

  IF EXISTS (
    SELECT 1 FROM public.exercises_test_cases t
    LEFT JOIN public.exercises e ON e.id = t.exercise_id WHERE e.id IS NULL
  ) THEN RAISE EXCEPTION 'orphan test_cases.exercise_id'; END IF;

  IF EXISTS (
    SELECT 1 FROM public.alerts a
    LEFT JOIN public.users u ON u.id = a.user_id
    WHERE a.user_id IS NOT NULL AND u.id IS NULL
  ) THEN RAISE EXCEPTION 'orphan alerts.user_id'; END IF;

  IF EXISTS (
    SELECT 1 FROM public.alerts a
    LEFT JOIN public.offerings o ON o.id = a.offering_id
    WHERE a.offering_id IS NOT NULL AND o.id IS NULL
  ) THEN RAISE EXCEPTION 'orphan alerts.offering_id'; END IF;

  IF EXISTS (
    SELECT 1 FROM public.system_logs l
    LEFT JOIN public.users u ON u.id = l.user_id
    WHERE l.user_id IS NOT NULL AND u.id IS NULL
  ) THEN RAISE EXCEPTION 'orphan system_logs.user_id'; END IF;

  IF EXISTS (
    SELECT 1 FROM public.commits c WHERE c.score < 0 OR c.score > 100
  ) THEN RAISE EXCEPTION 'commits.score out of range'; END IF;
END;
$$;

-- Informative summaries (not assertions).
SELECT 'users'        AS migrated_table, count(*) AS rows FROM public.users
UNION ALL SELECT 'offerings',  count(*) FROM public.offerings
UNION ALL SELECT 'enrollments', count(*) FROM public.enrollments
UNION ALL SELECT 'exercises', count(*) FROM public.exercises
UNION ALL SELECT 'commits', count(*) FROM public.commits
UNION ALL SELECT 'alerts', count(*) FROM public.alerts
UNION ALL SELECT 'test_cases', count(*) FROM public.exercises_test_cases
UNION ALL SELECT 'case_results', count(*) FROM public.commits_exercise_test_cases_results
UNION ALL SELECT 'system_logs', count(*) FROM public.system_logs
UNION ALL SELECT 'mail_logs', count(*) FROM public.mail_logs
UNION ALL SELECT 'user_messages', count(*) FROM public.user_messages
ORDER BY migrated_table;

SELECT role, count(*) AS users FROM public.users GROUP BY role ORDER BY role;
SELECT role, count(*) AS enrollments FROM public.enrollments GROUP BY role ORDER BY role;
SELECT status, count(*) AS commits FROM public.commits GROUP BY status ORDER BY status;
SELECT status, count(*) AS case_results
FROM public.commits_exercise_test_cases_results GROUP BY status ORDER BY status;
