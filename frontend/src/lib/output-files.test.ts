import { describe, expect, test } from "bun:test";

import {
  filesForTestCase,
  outputFileCaseId,
  outputFileCaseNumber,
} from "./output-files";

describe("outputFileCaseNumber", () => {
  test("reads the exercise-local case number from the storage prefix", () => {
    expect(outputFileCaseNumber("test_1/result.txt")).toBe(1);
    expect(outputFileCaseNumber("test_42/nested/dir/out.csv")).toBe(42);
  });

  test("returns null for files that belong to no case", () => {
    expect(outputFileCaseNumber("outputfiles/explicit.txt")).toBeNull();
    expect(outputFileCaseNumber("produced.txt")).toBeNull();
  });

  test("returns null for a prefix that is not a case number", () => {
    expect(outputFileCaseNumber("test_/result.txt")).toBeNull();
    expect(outputFileCaseNumber("test_abc/result.txt")).toBeNull();
    expect(outputFileCaseNumber("test_0/result.txt")).toBeNull();
  });
});

describe("outputFileCaseId", () => {
  test("keeps the old name for the same exercise-local number", () => {
    expect(outputFileCaseId("test_2/result.txt")).toBe(2);
  });
});

describe("filesForTestCase", () => {
  const files = [
    { path: "test_1/a.txt", size: 1 },
    { path: "test_2/b.txt", size: 2 },
    { path: "outputfiles/explicit.txt", size: 3 },
    { path: "test_1/nested/c.txt", size: 4 },
  ];

  test("keeps only the selected case's files", () => {
    expect(filesForTestCase(files, 1).map((file) => file.path)).toEqual([
      "test_1/a.txt",
      "test_1/nested/c.txt",
    ]);
    expect(filesForTestCase(files, 2).map((file) => file.path)).toEqual([
      "test_2/b.txt",
    ]);
  });

  test("returns an empty list for a case that produced nothing", () => {
    expect(filesForTestCase(files, 9)).toEqual([]);
  });
});
