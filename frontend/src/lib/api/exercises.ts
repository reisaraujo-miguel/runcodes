import { apiDelete, apiGet, apiPost, apiPut, apiRequest } from "./client";

/** A file type the platform can accept for submissions. */
export interface AllowedFileType {
  id: number;
  name: string;
  extension: string;
  is_compilable: boolean;
  is_available: boolean;
}

/** An exercise as returned by the API. */
export interface Exercise {
  id: number;
  offering_id: number;
  title: string;
  description: string;
  deadline: string;
  open_date: string;
  show_before_open_date: boolean;
  removed: boolean;
  created_at: string;
  updated_at: string;
  /**
   * Optional: the exercise's own allowed types, when the endpoint embeds them.
   * Used for client-side extension validation; absent on the base contract.
   */
  allowed_file_types?: AllowedFileType[];
  allowed_file_type_ids?: number[];
}

/** Payload accepted when creating or updating an exercise. */
export interface ExercisePayload {
  title: string;
  description: string;
  deadline: string;
  open_date: string;
  show_before_open_date?: boolean;
  allowed_file_type_ids?: number[];
}

/** A test case as returned by the API. */
export interface TestCase {
  id: number;
  exercise_id: number;
  input: string;
  input_type: string;
  show_input: boolean;
  expected_output: string;
  expected_output_type: string;
  show_expected_output: boolean;
  show_user_output: boolean;
  cpu_time_limit_seconds: number;
  mem_usage_limit_bytes: number;
  stack_limit_bytes: number;
  file_size_limit_bytes: number;
  files: unknown[];
}

/** Values accepted when creating a test case (text or file for each side). */
export interface NewTestCase {
  input_type: string;
  expected_output_type: string;
  input?: string;
  expected_output?: string;
  input_file?: File;
  expected_output_file?: File;
  show_input?: boolean;
  show_expected_output?: boolean;
  show_user_output?: boolean;
  cpu_time_limit_seconds?: number;
  mem_usage_limit_bytes?: number;
  stack_limit_bytes?: number;
  file_size_limit_bytes?: number;
  files?: File[];
}

/** A compilation file attached to an exercise. */
export interface CompilationFile {
  id: number;
  filename?: string;
  name?: string;
  [key: string]: unknown;
}

/** List the file types the platform supports. */
export function getAllowedFileTypes(): Promise<AllowedFileType[]> {
  return apiGet<AllowedFileType[]>("/api/v1/allowed-file-types");
}

/** List the exercises of an offering. */
export function getOfferingExercises(offeringId: number): Promise<Exercise[]> {
  return apiGet<Exercise[]>(
    `/api/v1/offerings/${String(offeringId)}/exercises`,
  );
}

/** Create an exercise inside an offering. */
export function createExercise(
  offeringId: number,
  payload: ExercisePayload,
): Promise<Exercise> {
  return apiPost<Exercise>(
    `/api/v1/offerings/${String(offeringId)}/exercises`,
    payload,
  );
}

/** Fetch a single exercise by id. */
export function getExercise(id: number): Promise<Exercise> {
  return apiGet<Exercise>(`/api/v1/exercises/${String(id)}`);
}

/** Update an exercise. */
export function updateExercise(
  id: number,
  payload: Partial<ExercisePayload>,
): Promise<Exercise> {
  return apiPut<Exercise>(`/api/v1/exercises/${String(id)}`, payload);
}

/** Delete an exercise. */
export function deleteExercise(id: number): Promise<void> {
  return apiDelete(`/api/v1/exercises/${String(id)}`);
}

/** List an exercise's test cases. */
export function getExerciseTestCases(exerciseId: number): Promise<TestCase[]> {
  return apiGet<TestCase[]>(
    `/api/v1/exercises/${String(exerciseId)}/test-cases`,
  );
}

function appendWhenDefined(
  form: FormData,
  key: string,
  value: string | number | boolean | undefined,
): void {
  if (value !== undefined) {
    form.append(key, String(value));
  }
}

/** Create a test case. Text fields and file fields are mutually optional. */
export function createTestCase(
  exerciseId: number,
  testCase: NewTestCase,
): Promise<TestCase> {
  const form = new FormData();
  form.append("input_type", testCase.input_type);
  form.append("expected_output_type", testCase.expected_output_type);
  appendWhenDefined(form, "input", testCase.input);
  appendWhenDefined(form, "expected_output", testCase.expected_output);
  appendWhenDefined(form, "show_input", testCase.show_input);
  appendWhenDefined(
    form,
    "show_expected_output",
    testCase.show_expected_output,
  );
  appendWhenDefined(form, "show_user_output", testCase.show_user_output);
  appendWhenDefined(
    form,
    "cpu_time_limit_seconds",
    testCase.cpu_time_limit_seconds,
  );
  appendWhenDefined(
    form,
    "mem_usage_limit_bytes",
    testCase.mem_usage_limit_bytes,
  );
  appendWhenDefined(form, "stack_limit_bytes", testCase.stack_limit_bytes);
  appendWhenDefined(
    form,
    "file_size_limit_bytes",
    testCase.file_size_limit_bytes,
  );
  if (testCase.input_file) form.append("input_file", testCase.input_file);
  if (testCase.expected_output_file) {
    form.append("expected_output_file", testCase.expected_output_file);
  }
  for (const file of testCase.files ?? []) {
    form.append("files", file);
  }

  return apiRequest<TestCase>(
    `/api/v1/exercises/${String(exerciseId)}/test-cases`,
    { method: "POST", body: form },
  );
}

/** Delete a test case. */
export function deleteTestCase(
  exerciseId: number,
  caseId: number,
): Promise<void> {
  return apiDelete(
    `/api/v1/exercises/${String(exerciseId)}/test-cases/${String(caseId)}`,
  );
}

/** List an exercise's compilation files. */
export function getCompilationFiles(
  exerciseId: number,
): Promise<CompilationFile[]> {
  return apiGet<CompilationFile[]>(
    `/api/v1/exercises/${String(exerciseId)}/compilation-files`,
  );
}

/** Upload a compilation file for an exercise. */
export function createCompilationFile(
  exerciseId: number,
  file: File,
): Promise<CompilationFile> {
  const form = new FormData();
  form.append("file", file);
  return apiRequest<CompilationFile>(
    `/api/v1/exercises/${String(exerciseId)}/compilation-files`,
    { method: "POST", body: form },
  );
}

/** Delete a compilation file from an exercise. */
export function deleteCompilationFile(
  exerciseId: number,
  fileId: number,
): Promise<void> {
  return apiDelete(
    `/api/v1/exercises/${String(exerciseId)}/compilation-files/${String(fileId)}`,
  );
}
