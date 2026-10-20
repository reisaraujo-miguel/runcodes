import { StreamLanguage } from "@codemirror/language";
import type { Extension } from "@codemirror/state";

const supportCache = new Map<string, Promise<Extension>>();

/**
 * Loads the CodeMirror support for an extension on demand. Grammars are large,
 * so each family lives in its own chunk and is only fetched once the student
 * actually edits a solution in that language.
 */
export function loadLanguageSupport(extension: string): Promise<Extension> {
  let support = supportCache.get(extension);
  if (!support) {
    support = importSupport(extension);
    supportCache.set(extension, support);
  }
  return support;
}

async function importSupport(extension: string): Promise<Extension> {
  switch (extension) {
    case "c":
    case "cpp":
    case "omp.c":
    case "omp.cpp":
    case "mpi.c":
    case "mpi.cpp":
      return (await import("@codemirror/lang-cpp")).cpp();
    case "py":
      return (await import("@codemirror/lang-python")).python();
    case "java":
      return (await import("@codemirror/lang-java")).java();
    case "rs":
      return (await import("@codemirror/lang-rust")).rust();
    case "go":
      return (await import("@codemirror/lang-go")).go();
    case "cs":
      return stream(
        (await import("@codemirror/legacy-modes/mode/clike")).csharp,
      );
    case "hs":
      return stream(
        (await import("@codemirror/legacy-modes/mode/haskell")).haskell,
      );
    case "f":
      return stream(
        (await import("@codemirror/legacy-modes/mode/fortran")).fortran,
      );
    case "pas":
      return stream(
        (await import("@codemirror/legacy-modes/mode/pascal")).pascal,
      );
    case "lua":
      return stream((await import("@codemirror/legacy-modes/mode/lua")).lua);
    case "r":
      return stream((await import("@codemirror/legacy-modes/mode/r")).r);
    case "m":
      return stream(
        (await import("@codemirror/legacy-modes/mode/octave")).octave,
      );
    case "v":
      return stream(
        (await import("@codemirror/legacy-modes/mode/verilog")).verilog,
      );
    default:
      return [];
  }
}

function stream(mode: Parameters<typeof StreamLanguage.define>[0]): Extension {
  return StreamLanguage.define(mode);
}
