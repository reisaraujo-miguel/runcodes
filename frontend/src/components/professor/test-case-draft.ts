import type {
  LanguageLimitsPayload,
  NewTestCase,
  TestCase,
  TestCasePatch,
} from "@/lib/api";

/** The two ways a test-case side may be provided. */
export const TEXT_INPUT = "text";
export const FILE_INPUT = "file";

/**
 * The editable state shared by the create and edit forms. Text and file values
 * live side by side; which one is sent is decided by the matching type field.
 */
export interface TestCaseDraft {
  inputType: string;
  outputType: string;
  inputText: string;
  outputText: string;
  inputFile: File | null;
  outputFile: File | null;
  showInput: boolean;
  showExpectedOutput: boolean;
  showUserOutput: boolean;
  /**
   * The per-language overrides being edited, keyed by allowed file type id. An
   * empty input inherits the exercise's default for that language; a language
   * with no entry in this map has no override of its own.
   */
  languageLimits: Record<number, LanguageLimitDraft>;
  extraFiles: File[];
}

/** One language's four limit inputs, as text ("" = inherit). */
export interface LanguageLimitDraft {
  cpu: string;
  mem: string;
  stack: string;
  fileSize: string;
}

/** A blank draft, used to create a new case and to reset after a creation. */
export function emptyDraft(): TestCaseDraft {
  return {
    inputType: TEXT_INPUT,
    outputType: TEXT_INPUT,
    inputText: "",
    outputText: "",
    inputFile: null,
    outputFile: null,
    showInput: false,
    showExpectedOutput: false,
    showUserOutput: true,
    languageLimits: {},
    extraFiles: [],
  };
}

/** Seeds the draft from a stored case, for the edit form. */
export function draftFromTestCase(testCase: TestCase): TestCaseDraft {
  return {
    inputType: testCase.input_type,
    outputType: testCase.expected_output_type,
    inputText: testCase.input,
    outputText: testCase.expected_output,
    inputFile: null,
    outputFile: null,
    showInput: testCase.show_input,
    showExpectedOutput: testCase.show_expected_output,
    showUserOutput: testCase.show_user_output,
    languageLimits: overridesFromCase(testCase),
    extraFiles: [],
  };
}

/** Seeds one editor row per language the case already overrides. */
function overridesFromCase(
  testCase: TestCase,
): Record<number, LanguageLimitDraft> {
  const rows: Record<number, LanguageLimitDraft> = {};
  for (const language of testCase.limits) {
    const overrides = language.overrides;
    if (
      overrides.cpu_time_limit_seconds === null &&
      overrides.mem_usage_limit_bytes === null &&
      overrides.stack_limit_bytes === null &&
      overrides.file_size_limit_bytes === null
    ) {
      continue;
    }
    rows[language.allowed_file_type_id] = {
      cpu: limitToInput(overrides.cpu_time_limit_seconds),
      mem: bytesToMiBInput(overrides.mem_usage_limit_bytes),
      stack: bytesToMiBInput(overrides.stack_limit_bytes),
      fileSize: bytesToMiBInput(overrides.file_size_limit_bytes),
    };
  }
  return rows;
}

/** Parses a limit field; an empty value means "inherit". */
export function parseLimit(value: string): number | undefined {
  if (value.trim() === "") return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : undefined;
}

/** Renders a stored second limit as the value of a number input. */
export function limitToInput(value: number | null): string {
  if (value === null || !Number.isFinite(value)) return "";
  return String(value);
}

const MIB = 1024 * 1024;

/** Renders a stored byte limit in MiB, with at most two decimals. */
export function bytesToMiBInput(value: number | null): string {
  if (value === null || !Number.isFinite(value)) return "";
  return String(Math.round((value / MIB) * 100) / 100);
}

/**
 * Parses a MiB input into bytes. An empty value inherits; a zero is meaningful
 * and means "no limit".
 */
export function parseMiBInput(value: string): number | undefined {
  if (value.trim() === "") return undefined;
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed < 0) return undefined;
  return Math.round(parsed * MIB);
}

/** Builds the multipart payload of a new case from the draft. */
export function draftToNewTestCase(draft: TestCaseDraft): NewTestCase {
  const payload: NewTestCase = {
    input_type: draft.inputType,
    expected_output_type: draft.outputType,
    show_input: draft.showInput,
    show_expected_output: draft.showExpectedOutput,
    show_user_output: draft.showUserOutput,
  };

  if (draft.inputType === FILE_INPUT) {
    if (draft.inputFile) payload.input_file = draft.inputFile;
  } else {
    payload.input = draft.inputText;
  }

  if (draft.outputType === FILE_INPUT) {
    if (draft.outputFile) payload.expected_output_file = draft.outputFile;
  } else {
    payload.expected_output = draft.outputText;
  }

  const languageLimits = languageLimitsPayload(draft);
  if (languageLimits.length > 0) payload.language_limits = languageLimits;

  if (draft.extraFiles.length > 0) payload.files = draft.extraFiles;

  return payload;
}

/**
 * Builds the partial update of an existing case. Only the fields the professor
 * changed are present, so the untouched ones (including an uploaded file) keep
 * their stored value. Selecting extra files replaces the whole extra-file set;
 * the API offers no way to clear them, so the UI does not either.
 */
export function draftToPatch(
  draft: TestCaseDraft,
  current: TestCase,
): TestCasePatch {
  const patch: TestCasePatch = {};

  if (current.input_type === FILE_INPUT) {
    if (draft.inputFile) patch.input_file = draft.inputFile;
  } else if (draft.inputText !== current.input) {
    patch.input = draft.inputText;
  }

  if (current.expected_output_type === FILE_INPUT) {
    if (draft.outputFile) patch.expected_output_file = draft.outputFile;
  } else if (draft.outputText !== current.expected_output) {
    patch.expected_output = draft.outputText;
  }

  if (draft.showInput !== current.show_input)
    patch.show_input = draft.showInput;
  if (draft.showExpectedOutput !== current.show_expected_output) {
    patch.show_expected_output = draft.showExpectedOutput;
  }
  if (draft.showUserOutput !== current.show_user_output) {
    patch.show_user_output = draft.showUserOutput;
  }

  // The editor owns the whole override set, so the patch always carries it: a
  // cleared input must remove the stored override, which a "changed fields only"
  // patch could not express.
  patch.language_limits = languageLimitsPayload(draft);

  if (draft.extraFiles.length > 0) patch.files = draft.extraFiles;

  return patch;
}

/** Builds the per-language overrides from the draft's rows. */
function languageLimitsPayload(draft: TestCaseDraft): LanguageLimitsPayload[] {
  return Object.entries(draft.languageLimits)
    .map(([id, row]) => ({
      allowed_file_type_id: Number(id),
      cpu_time_limit_seconds: parseLimit(row.cpu) ?? null,
      mem_usage_limit_bytes: parseMiBInput(row.mem) ?? null,
      stack_limit_bytes: parseMiBInput(row.stack) ?? null,
      file_size_limit_bytes: parseMiBInput(row.fileSize) ?? null,
    }))
    .filter((row) => Number.isFinite(row.allowed_file_type_id));
}

/**
 * Reads the display name of an extra file of a test case. The API types `files`
 * loosely, so the `path` is read defensively and an index-based fallback is used.
 */
export function testCaseFileName(file: unknown, index: number): string {
  if (file && typeof file === "object" && "path" in file) {
    const path = (file as { path?: unknown }).path;
    if (typeof path === "string" && path.trim() !== "") return path;
  }
  return `Arquivo ${String(index + 1)}`;
}

/** A stable React key for one of a test case's extra files. */
export function testCaseFileKey(file: unknown, index: number): string {
  if (file && typeof file === "object" && "id" in file) {
    const id = (file as { id?: unknown }).id;
    if (typeof id === "number" || typeof id === "string") return String(id);
  }
  return `${testCaseFileName(file, index)}-${String(index)}`;
}
