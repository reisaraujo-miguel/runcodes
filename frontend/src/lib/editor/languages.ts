import { cpp } from "@codemirror/lang-cpp";
import { go } from "@codemirror/lang-go";
import { java } from "@codemirror/lang-java";
import { python } from "@codemirror/lang-python";
import { rust } from "@codemirror/lang-rust";
import { StreamLanguage } from "@codemirror/language";
import { csharp } from "@codemirror/legacy-modes/mode/clike";
import { fortran } from "@codemirror/legacy-modes/mode/fortran";
import { haskell } from "@codemirror/legacy-modes/mode/haskell";
import { lua } from "@codemirror/legacy-modes/mode/lua";
import { octave } from "@codemirror/legacy-modes/mode/octave";
import { pascal } from "@codemirror/legacy-modes/mode/pascal";
import { r } from "@codemirror/legacy-modes/mode/r";
import { verilog } from "@codemirror/legacy-modes/mode/verilog";
import type { Extension } from "@codemirror/state";

import type { AllowedFileType } from "@/lib/api/exercises";
import { normalizeExtension } from "@/lib/file-types";

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

/** A language together with its CodeMirror support. */
export interface ResolvedEditorLanguage extends EditorLanguage {
  support: Extension;
}

function streamLanguage(parser: Parameters<typeof StreamLanguage.define>[0]) {
  return StreamLanguage.define(parser);
}

/**
 * CodeMirror support keyed by extension. C-family variants (including the
 * OpenMP/MPI ones) share the C++ grammar, and modes without a dedicated
 * grammar fall back to plain text.
 */
function supportForExtension(extension: string): Extension {
  switch (extension) {
    case "c":
    case "cpp":
    case "omp.c":
    case "omp.cpp":
    case "mpi.c":
    case "mpi.cpp":
      return cpp();
    case "py":
      return python();
    case "java":
      return java();
    case "rs":
      return rust();
    case "go":
      return go();
    case "cs":
      return streamLanguage(csharp);
    case "hs":
      return streamLanguage(haskell);
    case "f":
      return streamLanguage(fortran);
    case "pas":
      return streamLanguage(pascal);
    case "lua":
      return streamLanguage(lua);
    case "r":
      return streamLanguage(r);
    case "m":
      return streamLanguage(octave);
    case "v":
      return streamLanguage(verilog);
    default:
      return [];
  }
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
export function editorLanguageFrom(
  fileType: AllowedFileType,
): ResolvedEditorLanguage {
  const extension = normalizeExtension(fileType.extension).replace(/^\./, "");
  return {
    id: String(fileType.id),
    label: fileType.name,
    extension,
    defaultFilename: defaultFilenameFor(extension),
    support: supportForExtension(extension),
  };
}

/** Allowed file types that are archives/documents rather than source code. */
const NON_CODE_EXTENSIONS = new Set([".zip", ".pdf"]);

/** The editable languages among an exercise's allowed file types. */
export function editorLanguagesFor(
  fileTypes: readonly AllowedFileType[],
): ResolvedEditorLanguage[] {
  return fileTypes
    .filter(
      (type) =>
        type.is_compilable &&
        type.is_available &&
        !NON_CODE_EXTENSIONS.has(normalizeExtension(type.extension)),
    )
    .map(editorLanguageFrom);
}
