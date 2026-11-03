import {
  apiDelete,
  apiDownload,
  apiGet,
  apiPost,
  apiPut,
  apiRequest,
  apiUpload,
  type DownloadedFile,
} from "./client";

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
/** The layer a limit was resolved from. */
export type LimitSource = "platform" | "exercise" | "case";

/** The four limits that apply to one test case under one language. */
export interface LimitValues {
  cpu_time_limit_seconds: number;
  mem_usage_limit_bytes: number;
  stack_limit_bytes: number;
  file_size_limit_bytes: number;
}

/** Which layer supplied each of those limits. */
export interface LimitSources {
  cpu_time_limit_seconds: LimitSource;
  mem_usage_limit_bytes: LimitSource;
  stack_limit_bytes: LimitSource;
  file_size_limit_bytes: LimitSource;
}

/** One layer's values; `null` inherits the layer below. */
export interface LimitOverrides {
  cpu_time_limit_seconds: number | null;
  mem_usage_limit_bytes: number | null;
  stack_limit_bytes: number | null;
  file_size_limit_bytes: number | null;
}

/** The platform-wide defaults, the last layer of the chain. */
export type PlatformLimits = LimitValues;

/** One language of an exercise, with its override and the values that apply. */
export interface LanguageLimits {
  allowed_file_type_id: number;
  name: string;
  extension: string;
  overrides: LimitOverrides;
  limits: LimitValues;
  sources: LimitSources;
}

/** The per-language overrides sent when saving a test case. */
export interface LanguageLimitsPayload {
  allowed_file_type_id: number;
  cpu_time_limit_seconds?: number | null;
  mem_usage_limit_bytes?: number | null;
  stack_limit_bytes?: number | null;
  file_size_limit_bytes?: number | null;
}

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
  /** Resolved per allowed language, with the source of each value. */
  limits: LanguageLimits[];
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
  /** The whole set of per-language overrides; absent fields inherit. */
  language_limits?: LanguageLimitsPayload[];
  files?: File[];
}

/**
 * Values accepted when updating a test case. Every field is optional: the ones
 * left out keep their stored value, which is what the backend's partial update
 * does with an absent multipart field.
 */
export type TestCasePatch = Partial<NewTestCase>;

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

/**
 * Builds the multipart body of a test case. The create path sends the required
 * type fields; the update path sends only what changed.
 */
function buildTestCaseForm(
  testCase: TestCasePatch,
  partial: boolean,
): FormData {
  const form = new FormData();
  if (!partial) {
    form.append("input_type", testCase.input_type ?? "text");
    form.append(
      "expected_output_type",
      testCase.expected_output_type ?? "text",
    );
  } else {
    appendWhenDefined(form, "input_type", testCase.input_type);
    appendWhenDefined(
      form,
      "expected_output_type",
      testCase.expected_output_type,
    );
  }
  appendWhenDefined(form, "input", testCase.input);
  appendWhenDefined(form, "expected_output", testCase.expected_output);
  appendWhenDefined(form, "show_input", testCase.show_input);
  appendWhenDefined(
    form,
    "show_expected_output",
    testCase.show_expected_output,
  );
  appendWhenDefined(form, "show_user_output", testCase.show_user_output);
  if (testCase.language_limits) {
    form.append("language_limits", JSON.stringify(testCase.language_limits));
  }
  if (testCase.input_file) form.append("input_file", testCase.input_file);
  if (testCase.expected_output_file) {
    form.append("expected_output_file", testCase.expected_output_file);
  }
  for (const file of testCase.files ?? []) {
    form.append("files", file);
  }

  return form;
}

/** Create a test case. Text fields and file fields are mutually optional. */
export function createTestCase(
  exerciseId: number,
  testCase: NewTestCase,
): Promise<TestCase> {
  return apiRequest<TestCase>(
    `/api/v1/exercises/${String(exerciseId)}/test-cases`,
    { method: "POST", body: buildTestCaseForm(testCase, false) },
  );
}

/**
 * Update a test case. Only the fields present in the patch are sent, so the rest
 * keep their stored value (including their uploaded files).
 */
export function updateTestCase(
  exerciseId: number,
  caseId: number,
  patch: TestCasePatch,
): Promise<TestCase> {
  return apiRequest<TestCase>(
    `/api/v1/exercises/${String(exerciseId)}/test-cases/${String(caseId)}`,
    { method: "PUT", body: buildTestCaseForm(patch, true) },
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

/** One test case a limit alert's failures were concentrated in. */
export interface LimitAlertCase {
  number: number;
  failures: number;
}

/** A warning that a language has been failing submissions because of a limit. */
export interface ExerciseLimitAlert {
  id: number;
  exercise_id: number;
  allowed_file_type_id: number | null;
  language_name: string | null;
  limit_name: string;
  failures: number;
  /** The affected cases, worst first; empty when none could be attributed. */
  cases: LimitAlertCase[];
  window_started_at: string;
  window_ended_at: string;
  created_at: string;
  acknowledged_at?: string | null;
}

/** The platform-wide default limits. */
export function getPlatformLimits(): Promise<PlatformLimits> {
  return apiGet<PlatformLimits>("/api/v1/limits");
}

/**
 * The exercise's per-language defaults, with the values that apply when a test
 * case has no override of its own.
 */
export function getExerciseLanguageLimits(
  exerciseId: number,
): Promise<LanguageLimits[]> {
  return apiGet<LanguageLimits[]>(
    `/api/v1/exercises/${String(exerciseId)}/language-limits`,
  );
}

/** Save the exercise's per-language defaults (the whole set). */
export function replaceExerciseLanguageLimits(
  exerciseId: number,
  limits: LanguageLimitsPayload[],
): Promise<LanguageLimits[]> {
  return apiPut<LanguageLimits[]>(
    `/api/v1/exercises/${String(exerciseId)}/language-limits`,
    limits,
  );
}

/** The unacknowledged limit alerts of an exercise. */
export function getExerciseLimitAlerts(
  exerciseId: number,
): Promise<ExerciseLimitAlert[]> {
  return apiGet<ExerciseLimitAlert[]>(
    `/api/v1/exercises/${String(exerciseId)}/limit-alerts`,
  );
}

/** Mark one limit alert as handled. */
export function acknowledgeExerciseLimitAlert(
  exerciseId: number,
  alertId: number,
): Promise<void> {
  return apiPost<undefined>(
    `/api/v1/exercises/${String(exerciseId)}/limit-alerts/${String(alertId)}/acknowledge`,
    undefined,
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

/** Configuration shared by every case of a batch import. */
export interface BatchTestCaseConfig {
  input_type: string;
  expected_output_type: string;
  show_input?: boolean;
  show_expected_output?: boolean;
  show_user_output?: boolean;
  /** Applied as an override for every language the exercise allows. */
  cpu_time_limit_seconds?: number;
  mem_usage_limit_bytes?: number;
  stack_limit_bytes?: number;
  file_size_limit_bytes?: number;
}

/** One folder of the archive the batch importer could not parse. */
export interface BatchTestCaseIssue {
  folder: string;
  reason: string;
}

/** The error body of a rejected batch import. */
export interface BatchTestCaseErrorBody {
  error_msg: string;
  invalid_folders?: BatchTestCaseIssue[];
}

export interface BatchTestCaseImportResult {
  created: number;
  /**
   * Files at the archive root that were skipped because they are neither `.in`
   * nor `.out`. The import still succeeds; the professor is told what was left
   * out.
   */
  ignored?: string[];
  test_cases: TestCase[];
}

/**
 * Upload a zip with one test case per `.in`/`.out` pair. The pair may sit at the
 * archive root, named after the case, or inside a folder of that name when the
 * case carries extra files. The configuration applies to every case.
 */
export function importTestCasesBatch(
  exerciseId: number,
  archive: File,
  config: BatchTestCaseConfig,
): Promise<BatchTestCaseImportResult> {
  const form = new FormData();
  form.append("archive", archive);
  form.append("input_type", config.input_type);
  form.append("expected_output_type", config.expected_output_type);
  appendWhenDefined(form, "show_input", config.show_input);
  appendWhenDefined(form, "show_expected_output", config.show_expected_output);
  appendWhenDefined(form, "show_user_output", config.show_user_output);
  appendWhenDefined(
    form,
    "cpu_time_limit_seconds",
    config.cpu_time_limit_seconds,
  );
  appendWhenDefined(
    form,
    "mem_usage_limit_bytes",
    config.mem_usage_limit_bytes,
  );
  appendWhenDefined(form, "stack_limit_bytes", config.stack_limit_bytes);
  appendWhenDefined(
    form,
    "file_size_limit_bytes",
    config.file_size_limit_bytes,
  );

  return apiUpload<BatchTestCaseImportResult>(
    `/api/v1/exercises/${String(exerciseId)}/test-cases/batch`,
    form,
  );
}

/** Download every test case in the same folder layout the batch importer reads. */
export function downloadExerciseTestCases(
  exerciseId: number,
): Promise<DownloadedFile> {
  return apiDownload(
    `/api/v1/exercises/${String(exerciseId)}/test-cases/export`,
  );
}
