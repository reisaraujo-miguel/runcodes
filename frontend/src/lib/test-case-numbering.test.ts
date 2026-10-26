import { describe, expect, test } from "bun:test";

import { testCaseNumbers } from "./test-case-numbering";

describe("testCaseNumbers", () => {
  test("numbers an exercise's cases from 1 regardless of their ids", () => {
    const numbers = testCaseNumbers([{ id: 2 }, { id: 7 }]);

    expect(numbers.get(2)).toBe(1);
    expect(numbers.get(7)).toBe(2);
  });

  test("renumbers the remaining cases after one is removed", () => {
    const numbers = testCaseNumbers([{ id: 4 }, { id: 9 }]);

    expect(numbers.get(4)).toBe(1);
    expect(numbers.get(9)).toBe(2);
  });

  test("returns an empty map for an exercise without cases", () => {
    expect(testCaseNumbers([]).size).toBe(0);
  });
});
