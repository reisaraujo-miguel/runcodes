import { describe, expect, test } from "bun:test";

import {
  filesForTestCase,
  outputFileCaseNumber,
  outputFileLegacyCaseId,
} from "./output-files";

describe("outputFileCaseNumber", () => {
  test("reads the frozen exercise number from the current prefix", () => {
    expect(outputFileCaseNumber("test_v2_1/result.txt")).toBe(1);
    expect(outputFileCaseNumber("test_v2_42/nested/dir/out.csv")).toBe(42);
  });

  test("returns null for files that belong to no case", () => {
    expect(outputFileCaseNumber("outputfiles/explicit.txt")).toBeNull();
    expect(outputFileCaseNumber("produced.txt")).toBeNull();
  });

  test("returns null for a legacy id-only prefix", () => {
    expect(outputFileCaseNumber("test_7/result.txt")).toBeNull();
  });

  test("returns null for a prefix that is not a case number", () => {
    expect(outputFileCaseNumber("test_v2_/result.txt")).toBeNull();
    expect(outputFileCaseNumber("test_v2_abc/result.txt")).toBeNull();
    expect(outputFileCaseNumber("test_v2_0/result.txt")).toBeNull();
  });
});

describe("outputFileLegacyCaseId", () => {
  test("reads the database id from a legacy prefix", () => {
    expect(outputFileLegacyCaseId("test_101/result.txt")).toBe(101);
    expect(outputFileLegacyCaseId("test_7/nested/out.csv")).toBe(7);
  });

  test("does not read the current namespace as an id", () => {
    expect(outputFileLegacyCaseId("test_v2_3/result.txt")).toBeNull();
  });

  test("returns null for files that belong to no case", () => {
    expect(outputFileLegacyCaseId("outputfiles/explicit.txt")).toBeNull();
    expect(outputFileLegacyCaseId("test_/result.txt")).toBeNull();
    expect(outputFileLegacyCaseId("test_0/result.txt")).toBeNull();
  });
});

describe("filesForTestCase", () => {
  const testCase = { id: 101, number: 1 };
  const files = [
    { path: "test_v2_1/a.txt", size: 1 },
    { path: "test_v2_2/b.txt", size: 2 },
    { path: "test_101/legacy.txt", size: 3 },
    { path: "test_2/foreign.txt", size: 4 },
    { path: "outputfiles/explicit.txt", size: 5 },
    { path: "test_v2_1/nested/c.txt", size: 6 },
  ];

  test("keeps the current and legacy files of the selected case", () => {
    expect(filesForTestCase(files, testCase).map((file) => file.path)).toEqual([
      "test_v2_1/a.txt",
      "test_101/legacy.txt",
      "test_v2_1/nested/c.txt",
    ]);
    expect(
      filesForTestCase(files, { id: 102, number: 2 }).map((file) => file.path),
    ).toEqual(["test_v2_2/b.txt"]);
  });

  test("matches only the id when the case has no frozen number", () => {
    expect(
      filesForTestCase(files, { id: 101, number: null }).map(
        (file) => file.path,
      ),
    ).toEqual(["test_101/legacy.txt"]);
  });

  test("returns an empty list for a case that produced nothing", () => {
    expect(filesForTestCase(files, { id: 999, number: 9 })).toEqual([]);
  });
});
