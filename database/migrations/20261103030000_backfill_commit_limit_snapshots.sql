-- The judge now grades a commit against its frozen limit snapshot, and its
-- FetchTestCases query joins commit_test_case_limits. A commit that was already
-- queued when the snapshot table was introduced has no rows there, so the judge
-- would load no test case for it and finalize the run without grading anything.
--
-- Backfill those commits from the layers that are still readable at migration
-- time: the case's own override (moved to test_case_language_limits by the
-- previous migration) and the exercise's per-language default. The platform
-- layer is not in the database, so a field neither layer sets falls back to the
-- documented platform defaults below. Only commits that were in flight at
-- upgrade time are affected; every commit queued from now on is snapshotted by
-- the backend inside the transaction that creates it.
INSERT INTO "commit_test_case_limits"
    ("commit_id", "exercise_test_case_id",
     "cpu_time_limit_seconds", "mem_usage_limit_bytes",
     "stack_limit_bytes", "file_size_limit_bytes")
SELECT c."id", tc."id",
       COALESCE(tcl."cpu_time_limit_seconds",
                ell."cpu_time_limit_seconds", 5),
       COALESCE(tcl."mem_usage_limit_bytes",
                ell."mem_usage_limit_bytes", 536870912),
       COALESCE(tcl."stack_limit_bytes",
                ell."stack_limit_bytes", 0),
       COALESCE(tcl."file_size_limit_bytes",
                ell."file_size_limit_bytes", 268435456)
FROM "commits" c
JOIN "exercises" e ON e."id" = c."exercise_id"
JOIN "exercises_test_cases" tc
  ON tc."exercise_id" = CASE
       WHEN e."ghost" AND e."real_id" IS NOT NULL THEN e."real_id"
       ELSE e."id"
     END
LEFT JOIN "test_case_language_limits" tcl
  ON tcl."exercise_test_case_id" = tc."id"
 AND tcl."allowed_file_type_id" = c."allowed_file_type_id"
LEFT JOIN "exercise_language_limits" ell
  ON ell."exercise_id" = tc."exercise_id"
 AND ell."allowed_file_type_id" = c."allowed_file_type_id"
WHERE c."status" IN ('pending', 'queued', 'compiling', 'running')
ON CONFLICT DO NOTHING;
