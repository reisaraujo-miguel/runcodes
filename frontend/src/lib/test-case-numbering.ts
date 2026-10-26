/**
 * Test cases have a platform-wide database id, but they are always presented
 * inside one exercise. Numbering them by that global id makes the first case of
 * a second exercise start at 2; the UI instead numbers each exercise's cases
 * from 1 in the order the API returns them (ascending by id).
 */

/**
 * Maps each test case id to its 1-based position in the exercise.
 *
 * The order of `testCases` is the display order, so a case removed from the
 * middle of the list renumbers the ones that follow it.
 */
export function testCaseNumbers(
  testCases: readonly { id: number }[],
): Map<number, number> {
  return new Map(testCases.map((testCase, index) => [testCase.id, index + 1]));
}
