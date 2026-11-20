/**
 * Generated files live under the test case that produced them. The current
 * judge stores them as `test_v2_<case number>/<relative path>`, where the number
 * is the case's frozen 1-based position in the exercise; an older judge stored
 * `test_<case id>/<relative path>`. A file written straight into the runner's
 * output directory has no case (`outputfiles/<relative path>`). The results
 * panel shows one case at a time, so it needs to split the listing along that
 * path prefix.
 */

const currentCasePrefix = /^test_v2_(\d+)\//;
const legacyCasePrefix = /^test_(\d+)\//;

function positiveInteger(match: RegExpExecArray | null): number | null {
  if (!match) return null;
  const value = Number(match[1]);
  return Number.isSafeInteger(value) && value > 0 ? value : null;
}

/**
 * Returns the case number a generated file belongs to, for a path in the
 * current `test_v2_<number>/` namespace; null for a legacy path or a file that
 * belongs to the submission as a whole.
 */
export function outputFileCaseNumber(filePath: string): number | null {
  return positiveInteger(currentCasePrefix.exec(filePath));
}

/**
 * Returns the platform-wide case id a generated file belongs to, for a path in
 * the legacy `test_<case id>/` namespace; null for the current namespace or a
 * file that belongs to the submission as a whole.
 */
export function outputFileLegacyCaseId(filePath: string): number | null {
  return positiveInteger(legacyCasePrefix.exec(filePath));
}

/**
 * Keeps only the files one test case produced, in their original order. The
 * current namespace matches the case's frozen number; a legacy file matches its
 * database id, which is what its directory names.
 */
export function filesForTestCase<T extends { path: string }>(
  files: readonly T[],
  testCase: { id: number; number: number | null },
): T[] {
  return files.filter((file) => {
    const number = outputFileCaseNumber(file.path);
    if (number !== null) return number === testCase.number;
    return outputFileLegacyCaseId(file.path) === testCase.id;
  });
}
