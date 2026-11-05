/**
 * Generated files are stored under the test case that produced them
 * (`test_<case number>/<relative path>`); files written straight into the
 * runner's output directory have no case (`outputfiles/<relative path>`). The
 * number is the case's 1-based position in the exercise, matching the results
 * panel. The results panel shows one case at a time, so it needs to split the
 * listing along that path prefix.
 */

const casePrefix = /^test_(\d+)\//;

/**
 * Returns the case number a generated file belongs to, or null when it belongs
 * to the submission as a whole.
 */
export function outputFileCaseNumber(filePath: string): number | null {
  const match = casePrefix.exec(filePath);
  if (!match) return null;

  const caseNumber = Number(match[1]);
  return Number.isSafeInteger(caseNumber) && caseNumber > 0 ? caseNumber : null;
}

/**
 * Backwards-compatible name for `outputFileCaseNumber`. The value is the
 * exercise-local number, not the database id.
 */
export function outputFileCaseId(filePath: string): number | null {
  return outputFileCaseNumber(filePath);
}

/** Keeps only the files one test case produced, in their original order. */
export function filesForTestCase<T extends { path: string }>(
  files: readonly T[],
  testCaseNumber: number,
): T[] {
  return files.filter(
    (file) => outputFileCaseNumber(file.path) === testCaseNumber,
  );
}
