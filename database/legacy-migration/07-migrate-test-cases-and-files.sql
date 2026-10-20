-- Test cases and exercise files.
--
-- Column renames: output -> expected_output, output_type -> expected_output_type.
--
-- input_type/output_type mapping (legacy ints): 0 = text, 1 = file.
-- Any other value falls back to 'text'.
--
-- Resource limits (cpu_time_limit_seconds, mem_usage_limit_bytes,
-- stack_limit_bytes, file_size_limit_bytes) are discarded: the old monitoring
-- data was unreliable, so they are inserted as 0 to be configured later.
-- abs_error and the input/output md5 checksums are dropped entirely.
-- last_update becomes both created_at and updated_at (falling back to the
-- migration time when NULL).

INSERT INTO public.exercises_test_cases (
  id, exercise_id, input, input_type, show_input,
  expected_output, expected_output_type, show_expected_output, show_user_output,
  cpu_time_limit_seconds, mem_usage_limit_bytes, stack_limit_bytes,
  file_size_limit_bytes, created_at, updated_at
)
SELECT
  ec.id,
  ec.exercise_id,
  COALESCE(ec.input, ''),
  CASE ec.input_type WHEN 1 THEN 'file' ELSE 'text' END::public.input_output_type_t,
  COALESCE(ec.show_input, TRUE),
  COALESCE(ec.output, ''),
  CASE ec.output_type WHEN 1 THEN 'file' ELSE 'text' END::public.input_output_type_t,
  COALESCE(ec.show_expected_output, TRUE),
  COALESCE(ec.show_user_output, TRUE),
  0, 0, 0, 0,
  COALESCE(_migration.to_tstz(ec.last_update), NOW()),
  COALESCE(_migration.to_tstz(ec.last_update), NOW())
FROM old.exercise_cases ec;

-- Attached files (shown/downloadable on the exercise page).
INSERT INTO public.exercises_attached_files (id, exercise_id, path)
SELECT id, exercise_id, path FROM old.exercise_files;

-- Compilation files.
INSERT INTO public.exercises_compilation_files (id, exercise_id, path)
SELECT id, exercise_id, path FROM old.compilation_files;

-- Files attached to test cases (input/output files).
INSERT INTO public.exercises_test_cases_files (id, exercise_test_case_id, path)
SELECT id, exercise_case_id, path FROM old.exercise_case_files;
