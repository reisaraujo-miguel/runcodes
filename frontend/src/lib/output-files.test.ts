import { describe, expect, test } from "bun:test";

import { filesForTestCase, outputFileCaseId } from "./output-files";

describe("outputFileCaseId", () => {
  test("reads the case from the storage prefix", () => {
    expect(outputFileCaseId("test_1/result.txt")).toBe(1);
    expect(outputFileCaseId("test_42/nested/dir/out.csv")).toBe(42);
  });

  test("returns null for files that belong to no case", () => {
    expect(outputFileCaseId("outputfiles/explicit.txt")).toBeNull();
    expect(outputFileCaseId("produced.txt")).toBeNull();
  });

  test("returns null for a prefix that is not a case id", () => {
    expect(outputFileCaseId("test_/result.txt")).toBeNull();
    expect(outputFileCaseId("test_abc/result.txt")).toBeNull();
    expect(outputFileCaseId("test_0/result.txt")).toBeNull();
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
