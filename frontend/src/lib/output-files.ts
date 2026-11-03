/**
 * Generated files are stored under the test case that produced them
 * (`test_<case id>/<relative path>`); files written straight into the runner's
 * output directory have no case (`outputfiles/<relative path>`). The results
 * panel shows one case at a time, so it needs to split the listing along that
 * path prefix.
 */

const casePrefix = /^test_(\d+)\//;

/**
 * Returns the test case a generated file belongs to, or null when it belongs to
 * the submission as a whole.
 */
export function outputFileCaseId(filePath: string): number | null {
  const match = casePrefix.exec(filePath);
  if (!match) return null;

  const caseId = Number(match[1]);
  return Number.isSafeInteger(caseId) && caseId > 0 ? caseId : null;
}

/** Keeps only the files one test case produced, in their original order. */
export function filesForTestCase<T extends { path: string }>(
  files: readonly T[],
  testCaseId: number,
): T[] {
  return files.filter((file) => outputFileCaseId(file.path) === testCaseId);
}
