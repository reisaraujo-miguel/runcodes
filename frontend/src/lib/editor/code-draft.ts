/**
 * Local persistence for the code editor.
 *
 * Drafts are kept in `localStorage` rather than on the server: they must survive
 * a lost connection or a closed tab, and the API has no draft endpoint. The key
 * is scoped by user and exercise so a shared browser never shows one student's
 * code to another.
 *
 * This module is intentionally free of React: the storage and serialization
 * rules are pure so they can be tested without a DOM.
 */

/** Maximum number of code units stored for one draft. */
export const MAX_CODE_DRAFT_LENGTH = 1_000_000;

/** Identifies the payload shape; a future format can bump it safely. */
const CODE_DRAFT_VERSION = 1;

const STORAGE_PREFIX = "runcodes:code-draft:v1";

/**
 * The part of a draft that changes as the student edits. Kept separate from the
 * timestamp so the autosave hook can compare it without allocating a JSON string
 * on every keystroke.
 */
export interface CodeDraftContent {
  code: string;
  /** The editor language chosen for the buffer, or null for the default. */
  languageId: string | null;
  /** A custom submission filename, or null to use the language default. */
  filename: string | null;
}

/** A stored draft: the editor content plus when it was last written. */
export interface CodeDraft extends CodeDraftContent {
  /** Epoch milliseconds of the last successful save. */
  updatedAt: number;
}

/** The subset of `Storage` this module needs, injectable for tests. */
export interface DraftStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

export type CodeDraftWriteResult = "saved" | "too-large" | "unavailable";

function isPositiveId(value: number | null): value is number {
  return value !== null && Number.isInteger(value) && value > 0;
}

function readOptionalString(value: unknown): string | null {
  return typeof value === "string" && value !== "" ? value : null;
}

/** The storage key for one user's draft of one exercise. */
export function codeDraftStorageKey(
  userId: number,
  exerciseId: number,
): string {
  return `${STORAGE_PREFIX}:${String(userId)}:${String(exerciseId)}`;
}

/**
 * The browser's local storage, or null when it is unavailable. Accessing the
 * property itself can throw in privacy modes, so even that is guarded.
 */
export function getCodeDraftStorage(): DraftStorage | null {
  if (typeof window === "undefined") return null;

  try {
    return window.localStorage;
  } catch {
    return null;
  }
}

/**
 * Validates a value read from storage. Unknown or corrupt fields reject the
 * draft rather than restoring something the editor cannot display.
 */
export function normalizeCodeDraft(value: unknown): CodeDraft | null {
  if (typeof value !== "object" || value === null) return null;

  const record = value as Record<string, unknown>;
  if (record.version !== CODE_DRAFT_VERSION) return null;
  if (
    typeof record.code !== "string" ||
    record.code.length > MAX_CODE_DRAFT_LENGTH
  ) {
    return null;
  }

  const updatedAt =
    typeof record.updatedAt === "number" &&
    Number.isFinite(record.updatedAt) &&
    record.updatedAt > 0
      ? record.updatedAt
      : 0;

  return {
    code: record.code,
    languageId: readOptionalString(record.languageId),
    filename: readOptionalString(record.filename),
    updatedAt,
  };
}

/** Reads the stored draft for a user and exercise, if there is a valid one. */
export function readCodeDraft(
  storage: DraftStorage | null,
  userId: number | null,
  exerciseId: number | null,
): CodeDraft | null {
  if (!storage || !isPositiveId(userId) || !isPositiveId(exerciseId)) {
    return null;
  }

  try {
    const raw = storage.getItem(codeDraftStorageKey(userId, exerciseId));
    if (raw === null) return null;
    return normalizeCodeDraft(JSON.parse(raw) as unknown);
  } catch {
    return null;
  }
}

/**
 * Writes one draft. Storage failures (private mode, quota exceeded) are reported
 * instead of thrown so autosave can surface a warning without breaking editing.
 */
export function writeCodeDraft(
  storage: DraftStorage | null,
  userId: number | null,
  exerciseId: number | null,
  content: CodeDraftContent,
  updatedAt: number = Date.now(),
): CodeDraftWriteResult {
  if (!storage || !isPositiveId(userId) || !isPositiveId(exerciseId)) {
    return "unavailable";
  }

  if (content.code.length > MAX_CODE_DRAFT_LENGTH) {
    return "too-large";
  }

  try {
    const draft: CodeDraft & { version: number } = {
      version: CODE_DRAFT_VERSION,
      code: content.code,
      languageId: content.languageId,
      filename: content.filename,
      updatedAt,
    };
    storage.setItem(
      codeDraftStorageKey(userId, exerciseId),
      JSON.stringify(draft),
    );
    return "saved";
  } catch {
    return "unavailable";
  }
}

/** Removes the stored draft for a user and exercise. */
export function removeCodeDraft(
  storage: DraftStorage | null,
  userId: number | null,
  exerciseId: number | null,
): void {
  if (!storage || !isPositiveId(userId) || !isPositiveId(exerciseId)) return;

  try {
    storage.removeItem(codeDraftStorageKey(userId, exerciseId));
  } catch {
    // Storage unavailable: there is nothing more this call can do.
  }
}
