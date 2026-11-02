-- Record the language/format a submission chose.
--
-- The extension was already validated against the exercise's allowed file
-- types when the commit was created, but only the S3 key kept the filename.
-- Storing the matched catalog row makes the language statistics a plain join
-- instead of a per-commit filename parse, and gives ZIP submissions a stable
-- identity of their own.
ALTER TABLE commits
  ADD COLUMN allowed_file_type_id int
    REFERENCES allowed_file_types (id) ON DELETE SET NULL;

-- Backfill historical submissions. Each commit is matched against the allowed
-- types of its own exercise; the longest matching extension wins so that
-- `main.omp.c` maps to C (OpenMP) rather than plain C, mirroring the judge's
-- compound-extension rule. A commit whose exercise had no allowed types stays
-- NULL and is reported as unclassified.
UPDATE commits AS c
SET allowed_file_type_id = (
  SELECT eaft.allowed_file_type_id
  FROM exercises_allowed_file_types eaft
  JOIN allowed_file_types aft ON aft.id = eaft.allowed_file_type_id
  WHERE eaft.exercise_id = c.exercise_id
    AND (
      lower(regexp_replace(c.s3_key, '^.*/', '')) = lower(aft.extension)
      OR lower(regexp_replace(c.s3_key, '^.*/', '')) LIKE '%.' || lower(aft.extension)
    )
  ORDER BY length(aft.extension) DESC, aft.id
  LIMIT 1
)
WHERE c.allowed_file_type_id IS NULL
  AND c.s3_key IS NOT NULL;

CREATE INDEX idx_commits_allowed_file_type_id
  ON commits (allowed_file_type_id);
