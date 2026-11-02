import { describe, expect, test } from "bun:test";

import type { LanguageStat, MemoryStat } from "@/lib/api";

import {
  allowanceLanguages,
  choiceLanguages,
  formatCaseCount,
  formatCount,
  formatMedianCpu,
  formatPercent,
  formatRatio,
  formatRatioRange,
  isSmallSample,
  memoryLanguages,
  performanceLanguages,
  relativeBarWidth,
  relativeTone,
} from "./language-stats";

function memoryStat(overrides: Partial<MemoryStat> = {}): MemoryStat {
  return {
    relative: 1,
    p25: 0.9,
    p75: 1.1,
    median_bytes: 1024,
    compared_test_cases: 10,
    compared_exercises: 2,
    known_results: 10,
    total_results: 10,
    coverage: 100,
    ...overrides,
  };
}

function language(overrides: Partial<LanguageStat> = {}): LanguageStat {
  return {
    file_type_id: 1,
    name: "C",
    extension: "c",
    is_compilable: true,
    is_available: true,
    submissions: 0,
    submission_share: 0,
    students: 0,
    exercises_chosen: 0,
    pick_rate: null,
    pick_rate_samples: 0,
    exercises_offered: 0,
    exercise_offer_rate: 0,
    offerings_offered: 0,
    professors_offering: 0,
    professor_offer_rate: 0,
    results: 0,
    test_cases: 0,
    compared_test_cases: 0,
    compared_exercises: 0,
    cpu_time_median_seconds: null,
    relative_cpu: null,
    relative_cpu_p25: null,
    relative_cpu_p75: null,
    timings: {},
    ...overrides,
  };
}

describe("formatting", () => {
  test("formats counts and percentages in pt-BR", () => {
    expect(formatCount(12345)).toBe("12.345");
    expect(formatPercent(12.34)).toBe("12,3%");
    expect(formatCaseCount(1)).toBe("1 caso");
    expect(formatCaseCount(5)).toBe("5 casos");
  });

  test("formats CPU ratios with two decimals", () => {
    expect(formatRatio(1)).toBe("1,00×");
    expect(formatRatio(0.876)).toBe("0,88×");
    expect(formatRatioRange(0.9, 1.5)).toBe("0,90–1,50×");
    expect(formatRatioRange(null, null)).toBe("—");
  });

  test("formats median CPU time compactly", () => {
    expect(formatMedianCpu(null)).toBe("—");
    expect(formatMedianCpu(0.0004)).toBe("< 1 ms");
    expect(formatMedianCpu(0.042)).toBe("42 ms");
    expect(formatMedianCpu(1.234)).toBe("1,23 s");
  });

  test("flags small samples but not empty ones", () => {
    expect(isSmallSample(0)).toBe(false);
    expect(isSmallSample(4)).toBe(true);
    expect(isSmallSample(5)).toBe(false);
  });
});

describe("relative CPU presentation", () => {
  test("classifies ratios around the 1.00 baseline", () => {
    expect(relativeTone(0.9)).toBe("faster");
    expect(relativeTone(1)).toBe("typical");
    expect(relativeTone(1.1)).toBe("slower");
  });

  test("clamps the diverging bar to the 0–2× track", () => {
    expect(relativeBarWidth(0.5)).toBe(25);
    expect(relativeBarWidth(1)).toBe(50);
    expect(relativeBarWidth(3)).toBe(100);
    expect(relativeBarWidth(-1)).toBe(0);
  });
});

describe("ordering the three views", () => {
  test("choice keeps only used languages, largest first", () => {
    const rows = choiceLanguages([
      language({ file_type_id: 1, name: "C", submissions: 2 }),
      language({ file_type_id: 2, name: "Python", submissions: 10 }),
      language({ file_type_id: 3, name: "Rust", submissions: 0 }),
    ]);
    expect(rows.map((row) => row.name)).toEqual(["Python", "C"]);
  });

  test("allowance keeps every catalog entry, most offered first", () => {
    const rows = allowanceLanguages([
      language({ file_type_id: 1, name: "C", exercises_offered: 1 }),
      language({ file_type_id: 2, name: "Python", exercises_offered: 9 }),
      language({ file_type_id: 3, name: "Rust", exercises_offered: 0 }),
    ]);
    expect(rows.map((row) => row.name)).toEqual(["Python", "C", "Rust"]);
  });

  test("performance sorts measured languages first and nulls last", () => {
    const rows = performanceLanguages([
      language({
        file_type_id: 1,
        name: "Rust",
        relative_cpu: 1.4,
        compared_test_cases: 8,
      }),
      language({
        file_type_id: 2,
        name: "C",
        relative_cpu: 0.8,
        compared_test_cases: 8,
      }),
      language({ file_type_id: 3, name: "R", relative_cpu: null }),
    ]);
    expect(rows.map((row) => row.name)).toEqual(["C", "Rust", "R"]);
  });

  test("memory keeps measured languages, reliable samples first", () => {
    const rows = memoryLanguages([
      language({
        file_type_id: 1,
        name: "Outlier",
        memory: memoryStat({ relative: 0.5, compared_test_cases: 1 }),
      }),
      language({
        file_type_id: 2,
        name: "Solid",
        memory: memoryStat({ relative: 1.2, compared_test_cases: 10 }),
      }),
      language({ file_type_id: 3, name: "None", memory: null }),
    ]);
    expect(rows.map((row) => row.name)).toEqual(["Solid", "Outlier"]);
  });

  test("performance keeps small samples below reliable ones", () => {
    const rows = performanceLanguages([
      language({
        file_type_id: 1,
        name: "Outlier",
        relative_cpu: 0.5,
        compared_test_cases: 1,
      }),
      language({
        file_type_id: 2,
        name: "Solid",
        relative_cpu: 1.2,
        compared_test_cases: 10,
      }),
      language({ file_type_id: 3, name: "Unmeasured", relative_cpu: null }),
    ]);
    expect(rows.map((row) => row.name)).toEqual([
      "Solid",
      "Outlier",
      "Unmeasured",
    ]);
  });
});
