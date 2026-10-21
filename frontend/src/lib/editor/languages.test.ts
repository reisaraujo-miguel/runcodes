import { describe, expect, test } from "bun:test";

import type { AllowedFileType } from "@/lib/api/exercises";
import {
  defaultFilenameFor,
  editorLanguageForFilename,
  editorLanguagesFor,
  type EditorLanguage,
} from "@/lib/editor/languages";

function fileType(
  id: number,
  name: string,
  extension: string,
  overrides: Partial<AllowedFileType> = {},
): AllowedFileType {
  return {
    id,
    name,
    extension,
    is_compilable: true,
    is_available: true,
    ...overrides,
  };
}

describe("defaultFilenameFor", () => {
  test("uses conventional entry points for Java and C#", () => {
    expect(defaultFilenameFor("java")).toBe("Main.java");
    expect(defaultFilenameFor("cs")).toBe("Program.cs");
  });

  test("keeps compound extensions", () => {
    expect(defaultFilenameFor("omp.c")).toBe("main.omp.c");
  });

  test("falls back to a plain name without an extension", () => {
    expect(defaultFilenameFor("")).toBe("main");
  });
});

describe("editorLanguagesFor", () => {
  test("offers only available, compilable programming languages", () => {
    const languages = editorLanguagesFor([
      fileType(1, "Python 3", "py"),
      fileType(2, "PDF", "pdf", { is_compilable: false }),
      fileType(3, "Makefile", "zip"),
      fileType(4, "C++", "cpp", { is_available: false }),
    ]);

    expect(languages.map((language) => language.label)).toEqual(["Python 3"]);
  });

  test("keeps the backend's label and extension", () => {
    const [language] = editorLanguagesFor([
      fileType(10, "C (OpenMP)", "omp.c"),
    ]);

    expect(language).toMatchObject({
      id: "10",
      label: "C (OpenMP)",
      extension: "omp.c",
      defaultFilename: "main.omp.c",
    });
  });
});

describe("editorLanguageForFilename", () => {
  const languages: EditorLanguage[] = editorLanguagesFor([
    fileType(1, "Python 3", "py"),
    fileType(4, "C", "c"),
    fileType(2, "C (OpenMP)", "omp.c"),
    fileType(3, "Java", "java"),
  ]);

  test("matches a restored submission to its language", () => {
    expect(editorLanguageForFilename(languages, "main.py")?.id).toBe("1");
    expect(editorLanguageForFilename(languages, "Main.java")?.id).toBe("3");
    expect(editorLanguageForFilename(languages, "main.c")?.id).toBe("4");
  });

  test("prefers the most specific compound extension", () => {
    expect(editorLanguageForFilename(languages, "solver.omp.c")?.id).toBe("2");
  });

  test("returns undefined for files the editor cannot edit", () => {
    expect(editorLanguageForFilename(languages, "project.zip")).toBeUndefined();
    expect(editorLanguageForFilename(languages, "README")).toBeUndefined();
  });
});
