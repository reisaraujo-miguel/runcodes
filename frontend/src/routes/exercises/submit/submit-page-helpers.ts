import type { AllowedFileType, CommitStatus, Exercise } from "@/lib/api";
import { ApiRequestError } from "@/lib/api/client";
import { normalizeExtension } from "@/lib/file-types";

export const INITIAL_STATUS: CommitStatus = "queued";
export const FORM_ID = "submission-form";

// CodeMirror and its grammars are large, so they load only when the editor is

export function submitErrorMessage(error: unknown): string {
  if (error instanceof ApiRequestError) {
    switch (error.status) {
      case 400:
        return "Tipo de arquivo não permitido para este exercício.";
      case 403:
        return "Você não está matriculado nesta turma.";
      case 422:
        return "O prazo deste exercício já passou.";
      case 503:
        return "O corretor está indisponível no momento. Tente novamente em alguns instantes.";
    }
  }
  return error instanceof Error
    ? error.message
    : "Erro ao enviar a submissão. Tente novamente.";
}

function commitStorageKey(exerciseId: number): string {
  return `runcodes:submission:${String(exerciseId)}`;
}

export function readStoredCommit(exerciseId: number): number | null {
  try {
    const stored = sessionStorage.getItem(commitStorageKey(exerciseId));
    if (!stored) return null;
    const parsed = Number(stored);
    return Number.isInteger(parsed) && parsed > 0 ? parsed : null;
  } catch {
    return null;
  }
}

export function storeCommit(exerciseId: number, commitId: number | null): void {
  try {
    if (commitId === null) {
      sessionStorage.removeItem(commitStorageKey(exerciseId));
    } else {
      sessionStorage.setItem(commitStorageKey(exerciseId), String(commitId));
    }
  } catch {
    // Storage unavailable (private mode) — reload will simply not resume.
  }
}

/**
 * Resolves the types an exercise accepts on the server: its own allowed types
 * when configured, otherwise every available platform type.
 */
export function resolveAllowedFileTypes(
  exercise: Exercise | null,
  allTypes: AllowedFileType[],
): AllowedFileType[] {
  if (!exercise) return [];

  const own = exercise.allowed_file_types;
  if (own && own.length > 0) {
    const available = own.filter((type) => type.is_available);
    if (available.length > 0) return available;
  }

  const ids = exercise.allowed_file_type_ids;
  if (ids && ids.length > 0) {
    return allTypes.filter(
      (type) => ids.includes(type.id) && type.is_available,
    );
  }

  // An explicitly empty exercise list means "no restriction": offer everything
  // the platform can compile so the editor still has languages to choose from.
  return allTypes.filter((type) => type.is_available);
}

/**
 * Resolves the extensions the exercise accepts for client-side validation.
 * Returns `null` when nothing is restricted (skip validation).
 */
export function resolveAllowedExtensions(
  exercise: Exercise | null,
  allTypes: AllowedFileType[],
): string[] | null {
  if (!exercise) return null;

  const own = exercise.allowed_file_types;
  if (own) {
    // An empty list means no restriction; a list whose entries are all
    // unavailable is treated the same way by the backend's validation.
    const available = own.filter((type) => type.is_available);
    if (own.length === 0 || available.length === 0) return null;
    const extensions = [
      ...new Set(available.map((type) => normalizeExtension(type.extension))),
    ].filter((extension) => extension !== "");
    return extensions.length > 0 ? extensions : null;
  }

  const ids = exercise.allowed_file_type_ids;
  const source =
    ids && ids.length > 0
      ? allTypes.filter((type) => ids.includes(type.id))
      : allTypes.filter((type) => type.is_available);
  const extensions = [
    ...new Set(source.map((type) => normalizeExtension(type.extension))),
  ].filter((extension) => extension !== "");
  return extensions.length > 0 ? extensions : null;
}
