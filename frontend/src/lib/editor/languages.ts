import type { AllowedFileType } from "@/lib/api/exercises";
import { matchesAllowedExtension, normalizeExtension } from "@/lib/file-types";

/** A programming language offered by the in-browser editor. */
export interface EditorLanguage {
  /** Stable identity: the backend allowed-file-type id when available. */
  id: string;
  label: string;
  /** Extension without the leading dot, as stored by the backend. */
  extension: string;
  /** Suggested submission filename for a new buffer. */
  defaultFilename: string;
}

/** Suggests a submission filename; Java and C# require a conventional entry point. */
export function defaultFilenameFor(extension: string): string {
  const normalized = normalizeExtension(extension).replace(/^\./, "");
  if (normalized === "") return "main";
  if (normalized === "java") return "Main.java";
  if (normalized === "cs") return "Program.cs";
  return `main.${normalized}`;
}

/** Builds an editor language from an exercise's allowed file type. */
export function editorLanguageFrom(fileType: AllowedFileType): EditorLanguage {
  const extension = normalizeExtension(fileType.extension).replace(/^\./, "");
  return {
    id: String(fileType.id),
    label: fileType.name,
    extension,
    defaultFilename: defaultFilenameFor(extension),
  };
}

/** Allowed file types that are archives/documents rather than source code. */
const NON_CODE_EXTENSIONS = new Set([".zip", ".pdf"]);

/** The editable languages among an exercise's allowed file types. */
export function editorLanguagesFor(
  fileTypes: readonly AllowedFileType[],
): EditorLanguage[] {
  return fileTypes
    .filter(
      (type) =>
        type.is_compilable &&
        type.is_available &&
        !NON_CODE_EXTENSIONS.has(normalizeExtension(type.extension)),
    )
    .map(editorLanguageFrom);
}

/**
 * Finds the editor language matching a submitted filename. Used when restoring
 * a past solution: the stored filename is the only language signal a commit
 * carries. Returns undefined when the file is not one the editor can edit
 * (e.g. a ZIP submission, or a language no longer allowed by the exercise).
 *
 * The longest matching extension wins, so "solver.omp.c" restores as OpenMP C
 * rather than as plain C when both are offered.
 */
export function editorLanguageForFilename(
  languages: readonly EditorLanguage[],
  filename: string,
): EditorLanguage | undefined {
  let match: EditorLanguage | undefined;
  let matchLength = 0;

  for (const language of languages) {
    if (!matchesAllowedExtension(filename, [language.extension])) continue;
    const length = normalizeExtension(language.extension).length;
    if (length > matchLength) {
      match = language;
      matchLength = length;
    }
  }

  return match;
}
