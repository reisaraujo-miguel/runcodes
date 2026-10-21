import { useCallback, useEffect, useRef, useState } from "react";

import {
  getCodeDraftStorage,
  removeCodeDraft,
  writeCodeDraft,
  type CodeDraft,
  type CodeDraftContent,
  type CodeDraftWriteResult,
} from "@/lib/editor/code-draft";

/**
 * How long after the first unsaved change the draft is written. The timer is
 * not restarted by later keystrokes: a student who types without pausing still
 * gets a save at this interval instead of waiting for a pause that may never
 * come.
 */
const SAVE_DELAY_MS = 800;

const TOO_LARGE_MESSAGE =
  "O código é grande demais para ser salvo automaticamente.";
const UNAVAILABLE_MESSAGE =
  "Não foi possível salvar o rascunho neste navegador.";

export type CodeDraftSaveStatus = "idle" | "saving" | "saved" | "error";

export interface CodeDraftController {
  /** Save state derived from the current content and the last write. */
  status: CodeDraftSaveStatus;
  /** When the draft was last written, or null if none exists. */
  savedAt: number | null;
  /** Why the last write failed, if it did. */
  error: string | null;
  /** True while the editor still shows an unmodified restored draft. */
  restored: boolean;
  /** Whether a stored draft exists that can be discarded. */
  hasDraft: boolean;
  /** Writes pending changes immediately; use before a submission. */
  flush: () => void;
  /**
   * Overwrites the saved draft with new content immediately. Used when the
   * student restores a previous submission, which intentionally discards the
   * draft currently in the editor.
   */
  replace: (content: CodeDraftContent) => void;
  /** Removes the stored draft without touching the editor content. */
  discard: () => void;
}

export interface UseCodeDraftOptions {
  userId: number | null;
  exerciseId: number | null;
  code: string;
  languageId: string | null;
  filename: string | null;
  /**
   * The draft read before these states were initialized. It seeds the saved
   * baseline so mounting does not immediately rewrite the restored draft.
   */
  initialDraft: CodeDraft | null;
}

function writeMessage(result: Exclude<CodeDraftWriteResult, "saved">): string {
  return result === "too-large" ? TOO_LARGE_MESSAGE : UNAVAILABLE_MESSAGE;
}

/**
 * Persists the editor content as the student types.
 *
 * The hook never writes on mount: `savedValues` starts at the values the page
 * restored, so only an actual edit makes the draft dirty. A save is attempted at
 * most once per `SAVE_DELAY_MS` while edits continue, and pending changes are
 * flushed when the page is hidden, the component unmounts, or the caller submits.
 */
export function useCodeDraft({
  userId,
  exerciseId,
  code,
  languageId,
  filename,
  initialDraft,
}: UseCodeDraftOptions): CodeDraftController {
  // The last content known to be in storage. Comparing against it derives the
  // "saving" state during render, without setState in an effect.
  const [savedValues, setSavedValues] = useState<CodeDraftContent>(() => ({
    code,
    languageId,
    filename,
  }));
  const [savedAt, setSavedAt] = useState<number | null>(
    initialDraft?.updatedAt ?? null,
  );
  const [hasDraft, setHasDraft] = useState(initialDraft !== null);
  const [restored, setRestored] = useState(initialDraft !== null);
  const [error, setError] = useState<string | null>(null);

  const dirty =
    code !== savedValues.code ||
    languageId !== savedValues.languageId ||
    filename !== savedValues.filename;

  const timerRef = useRef<number | null>(null);
  const latestRef = useRef<CodeDraftContent>({ code, languageId, filename });
  const dirtyRef = useRef(dirty);

  const persist = useCallback(
    (content: CodeDraftContent, notify = true) => {
      if (userId === null || exerciseId === null) return;

      const timestamp = Date.now();
      const result = writeCodeDraft(
        getCodeDraftStorage(),
        userId,
        exerciseId,
        content,
        timestamp,
      );

      // An unmount cleanup runs after the component is gone, so it must not
      // schedule state updates.
      if (!notify) return;

      if (result === "saved") {
        setSavedValues(content);
        setSavedAt(timestamp);
        setHasDraft(true);
        setRestored(false);
        setError(null);
      } else {
        setError(writeMessage(result));
      }
    },
    [userId, exerciseId],
  );

  // Read through a ref so the throttled timer always uses the latest content
  // and the latest ids without being recreated on every keystroke.
  const persistRef = useRef(persist);
  useEffect(() => {
    persistRef.current = persist;
  }, [persist]);

  useEffect(() => {
    latestRef.current = { code, languageId, filename };
    dirtyRef.current = dirty;

    if (!dirty) {
      if (timerRef.current !== null) {
        window.clearTimeout(timerRef.current);
        timerRef.current = null;
      }
      return;
    }

    // Keep an already scheduled write: it captures the latest content through
    // the ref, and not resetting it guarantees progress while typing continues.
    timerRef.current ??= window.setTimeout(() => {
      timerRef.current = null;
      if (dirtyRef.current) persistRef.current(latestRef.current);
    }, SAVE_DELAY_MS);
  }, [code, languageId, filename, dirty]);

  const flush = useCallback(() => {
    if (!dirty) return;

    if (timerRef.current !== null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    persist({ code, languageId, filename });
  }, [dirty, persist, code, languageId, filename]);

  // The debounce timer cannot run once the page is frozen or closed. Write the
  // latest content synchronously while storage is still available.
  useEffect(() => {
    const handlePageHide = () => {
      flush();
    };
    const handleVisibilityChange = () => {
      if (document.visibilityState === "hidden") flush();
    };

    window.addEventListener("pagehide", handlePageHide);
    document.addEventListener("visibilitychange", handleVisibilityChange);
    return () => {
      window.removeEventListener("pagehide", handlePageHide);
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, [flush]);

  // A client-side navigation does not fire pagehide. Flush on unmount too, but
  // without state updates because the component is already gone.
  useEffect(() => {
    return () => {
      if (timerRef.current !== null) {
        window.clearTimeout(timerRef.current);
        timerRef.current = null;
      }
      if (dirtyRef.current) persistRef.current(latestRef.current, false);
    };
  }, []);

  const replace = useCallback(
    (content: CodeDraftContent) => {
      if (timerRef.current !== null) {
        window.clearTimeout(timerRef.current);
        timerRef.current = null;
      }
      persist(content);
    },
    [persist],
  );

  const discard = useCallback(() => {
    if (userId === null || exerciseId === null) return;

    if (timerRef.current !== null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }

    removeCodeDraft(getCodeDraftStorage(), userId, exerciseId);

    // Treat the current editor content as the new baseline: deleting the draft
    // must not recreate it from the same content on the next render.
    setSavedValues({ code, languageId, filename });
    setSavedAt(null);
    setHasDraft(false);
    setRestored(false);
    setError(null);
  }, [userId, exerciseId, code, languageId, filename]);

  const status: CodeDraftSaveStatus =
    dirty && error !== null
      ? "error"
      : dirty
        ? "saving"
        : hasDraft
          ? "saved"
          : "idle";

  return {
    status,
    savedAt,
    error,
    restored,
    hasDraft,
    flush,
    replace,
    discard,
  };
}
