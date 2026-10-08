import type { NewTestCase, TestCase, TestCasePatch } from "@/lib/api";

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
  cpuTime: string;
  memLimit: string;
  stackLimit: string;
  fileSizeLimit: string;
  extraFiles: File[];
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
    cpuTime: "",
    memLimit: "",
    stackLimit: "",
    fileSizeLimit: "",
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
    cpuTime: limitToInput(testCase.cpu_time_limit_seconds),
    memLimit: limitToInput(testCase.mem_usage_limit_bytes),
    stackLimit: limitToInput(testCase.stack_limit_bytes),
    fileSizeLimit: limitToInput(testCase.file_size_limit_bytes),
    extraFiles: [],
  };
}

/** Parses a limit field; an empty value means "let the API decide". */
export function parseLimit(value: string): number | undefined {
  if (value.trim() === "") return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : undefined;
}

/** Renders a stored limit as the value of a number input (0 = platform default). */
export function limitToInput(value: number): string {
  return Number.isFinite(value) ? String(value) : "";
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

  applyLimits(payload, draft);

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

  if (draft.showInput !== current.show_input) patch.show_input = draft.showInput;
  if (draft.showExpectedOutput !== current.show_expected_output) {
    patch.show_expected_output = draft.showExpectedOutput;
  }
  if (draft.showUserOutput !== current.show_user_output) {
    patch.show_user_output = draft.showUserOutput;
  }

  const cpu = parseLimit(draft.cpuTime);
  if (cpu !== undefined && cpu !== current.cpu_time_limit_seconds) {
    patch.cpu_time_limit_seconds = cpu;
  }
  const mem = parseLimit(draft.memLimit);
  if (mem !== undefined && mem !== current.mem_usage_limit_bytes) {
    patch.mem_usage_limit_bytes = mem;
  }
  const stack = parseLimit(draft.stackLimit);
  if (stack !== undefined && stack !== current.stack_limit_bytes) {
    patch.stack_limit_bytes = stack;
  }
  const fileSize = parseLimit(draft.fileSizeLimit);
  if (fileSize !== undefined && fileSize !== current.file_size_limit_bytes) {
    patch.file_size_limit_bytes = fileSize;
  }

  if (draft.extraFiles.length > 0) patch.files = draft.extraFiles;

  return patch;
}

function applyLimits(payload: NewTestCase, draft: TestCaseDraft): void {
  const cpu = parseLimit(draft.cpuTime);
  if (cpu !== undefined) payload.cpu_time_limit_seconds = cpu;
  const mem = parseLimit(draft.memLimit);
  if (mem !== undefined) payload.mem_usage_limit_bytes = mem;
  const stack = parseLimit(draft.stackLimit);
  if (stack !== undefined) payload.stack_limit_bytes = stack;
  const fileSize = parseLimit(draft.fileSizeLimit);
  if (fileSize !== undefined) payload.file_size_limit_bytes = fileSize;
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
