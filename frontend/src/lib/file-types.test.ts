import { describe, expect, test } from "bun:test";

import {
  extensionOf,
  matchesAllowedExtension,
  normalizeExtension,
  validateFileType,
} from "@/lib/file-types";

describe("normalizeExtension", () => {
  test("lowercases and adds the leading dot", () => {
    expect(normalizeExtension("PY")).toBe(".py");
    expect(normalizeExtension(".Py")).toBe(".py");
    expect(normalizeExtension("  cpp ")).toBe(".cpp");
  });

  test("keeps an empty extension empty", () => {
    expect(normalizeExtension("   ")).toBe("");
  });
});

describe("extensionOf", () => {
  test("returns the last extension lowercased", () => {
    expect(extensionOf("Main.JAVA")).toBe(".java");
    expect(extensionOf("archive.tar.gz")).toBe(".gz");
  });

  test("returns an empty string when there is no extension", () => {
    expect(extensionOf("Makefile")).toBe("");
  });
});

describe("matchesAllowedExtension", () => {
  test("matches compound extensions by suffix", () => {
    expect(matchesAllowedExtension("main.omp.c", ["omp.c"])).toBe(true);
    expect(matchesAllowedExtension("main.mpi.cpp", ["mpi.cpp"])).toBe(true);
  });

  test("accepts a compound file for its short extension, like the backend", () => {
    expect(matchesAllowedExtension("main.omp.c", ["c"])).toBe(true);
    expect(matchesAllowedExtension("main.c", ["omp.c"])).toBe(false);
  });

  test("is case-insensitive", () => {
    expect(matchesAllowedExtension("MAIN.C", [".c"])).toBe(true);
  });
});

describe("validateFileType", () => {
  const file = (name: string) => new File(["x"], name, { type: "text/plain" });

  test("skips validation when there is no restriction", () => {
    expect(validateFileType(file("anything.xyz"), null)).toBeNull();
  });

  test("accepts a matching extension", () => {
    expect(validateFileType(file("main.py"), [".py", ".c"])).toBeNull();
  });

  test("reports the accepted types on a mismatch", () => {
    expect(validateFileType(file("main.java"), [".py", ".c"])).toBe(
      "Extensão não permitida. Tipos aceitos: .py, .c.",
    );
  });
});
