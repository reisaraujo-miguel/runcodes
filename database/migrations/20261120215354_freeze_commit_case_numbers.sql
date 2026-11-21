-- Freeze each commit's case numbering with its limit snapshot.
--
-- Generated files are named after a case's position inside the exercise
-- (`test_v2_<number>/...`). Deriving that position from the live rows renumbers
-- the remaining cases as soon as a test case is deleted (the limit row
-- cascades), which would attribute a stored file to another case -- and
-- authorize it with that case's hidden-output rule. The number is written when
-- the commit is queued and never changes afterwards.
ALTER TABLE "commit_test_case_limits" ADD COLUMN "case_number" integer;

-- Existing snapshots keep the numbering the backend already used when it read
-- the positions: the case's place in the (exercise_test_case_id) order.
UPDATE "commit_test_case_limits" AS cl
SET case_number = ranked.position
FROM (
    SELECT commit_id,
           exercise_test_case_id,
           row_number() OVER (
               PARTITION BY commit_id ORDER BY exercise_test_case_id
           ) AS position
    FROM "commit_test_case_limits"
) AS ranked
WHERE cl.commit_id = ranked.commit_id
  AND cl.exercise_test_case_id = ranked.exercise_test_case_id;

ALTER TABLE "commit_test_case_limits"
  ALTER COLUMN "case_number" SET NOT NULL,
  ADD CONSTRAINT "commit_test_case_limits_case_number_check"
    CHECK (case_number > 0),
  ADD CONSTRAINT "commit_test_case_limits_commit_id_case_number_key"
    UNIQUE ("commit_id", "case_number");
