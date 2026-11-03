import { describe, expect, test } from "bun:test";

import { formatLimitAlertCases, limitLabel } from "@/lib/limit-alerts";

describe("formatLimitAlertCases", () => {
  test("omits the sentence when the alert has no case detail", () => {
    expect(formatLimitAlertCases([])).toBeNull();
  });

  test("uses the singular for one failure", () => {
    expect(formatLimitAlertCases([{ number: 3, failures: 1 }])).toBe(
      "caso 3 (1 submissão)",
    );
  });

  test("lists the cases worst first, as the API orders them", () => {
    expect(
      formatLimitAlertCases([
        { number: 18, failures: 12 },
        { number: 19, failures: 7 },
        { number: 20, failures: 5 },
      ]),
    ).toBe(
      "caso 18 (12 submissões), caso 19 (7 submissões), caso 20 (5 submissões)",
    );
  });

  test("names the worst five and counts the rest", () => {
    const cases = Array.from({ length: 8 }, (_, index) => ({
      number: index + 1,
      failures: 10 - (index + 1),
    }));

    expect(formatLimitAlertCases(cases)).toBe(
      "caso 1 (9 submissões), caso 2 (8 submissões), caso 3 (7 submissões), " +
        "caso 4 (6 submissões), caso 5 (5 submissões) e mais 3 casos",
    );
  });

  test("summarizes a single remaining case in the singular", () => {
    const cases = Array.from({ length: 6 }, (_, index) => ({
      number: index + 1,
      failures: 2,
    }));

    expect(formatLimitAlertCases(cases)).toEndWith("e mais 1 caso");
  });
});

describe("limitLabel", () => {
  test("translates the stored limit names", () => {
    expect(limitLabel("cpu_time")).toBe("tempo de CPU");
    expect(limitLabel("file_size")).toBe("tamanho de arquivo");
  });

  test("falls back to the stored name", () => {
    expect(limitLabel("unknown")).toBe("unknown");
  });
});
