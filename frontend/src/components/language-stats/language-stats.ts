import type { LanguageStat } from "@/lib/api";

const countFormatter = new Intl.NumberFormat("pt-BR");
const percentFormatter = new Intl.NumberFormat("pt-BR", {
  minimumFractionDigits: 1,
  maximumFractionDigits: 1,
});
const ratioFormatter = new Intl.NumberFormat("pt-BR", {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
  useGrouping: false,
});
const millisecondFormatter = new Intl.NumberFormat("pt-BR", {
  maximumFractionDigits: 1,
});
const secondFormatter = new Intl.NumberFormat("pt-BR", {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

/** Whole counts use the Brazilian thousands separator. */
export function formatCount(value: number): string {
  return countFormatter.format(value);
}

/** The API sends percentages already scaled to 0–100. */
export function formatPercent(value: number): string {
  return `${percentFormatter.format(value)}%`;
}

/** A CPU ratio where 1.00 is the typical language on the same test case. */
export function formatRatio(value: number): string {
  return `${ratioFormatter.format(value)}×`;
}

/** A compact, human-readable duration in seconds. */
export function formatDuration(seconds: number | null): string {
  if (seconds === null || !Number.isFinite(seconds) || seconds < 0) return "—";
  if (seconds < 0.001) return "< 1 ms";
  if (seconds < 1) return `${millisecondFormatter.format(seconds * 1000)} ms`;
  return `${secondFormatter.format(seconds)} s`;
}

/** The CPU table's name for the same duration formatter. */
export const formatMedianCpu = formatDuration;

/** The p25–p75 spread of the relative CPU ratio. */
export function formatRatioRange(
  low: number | null,
  high: number | null,
): string {
  if (low === null && high === null) return "—";
  if (low === null) return formatRatio(high ?? 0);
  if (high === null) return formatRatio(low);
  return `${ratioFormatter.format(low)}–${formatRatio(high)}`;
}

/** The number of test cases behind a relative CPU figure. */
export function formatCaseCount(value: number): string {
  return `${formatCount(value)} ${value === 1 ? "caso" : "casos"}`;
}

/** Below this many comparable cases the median is shown with a caution mark. */
export const SMALL_SAMPLE = 5;

export function isSmallSample(cases: number): boolean {
  return cases > 0 && cases < SMALL_SAMPLE;
}

export type RelativeTone = "faster" | "typical" | "slower";

/** Classifies a CPU ratio without hiding the exact number. */
export function relativeTone(ratio: number): RelativeTone {
  if (ratio < 0.95) return "faster";
  if (ratio > 1.05) return "slower";
  return "typical";
}

/**
 * The diverging CPU bar uses 2× as the full width and puts 1× in the middle.
 * The exact ratio is always printed next to it, so the bar never carries the
 * only copy of the value.
 */
export function relativeBarWidth(ratio: number): number {
  return Math.min(Math.max(ratio / 2, 0), 1) * 100;
}

/** Languages ordered by classified student submissions. */
export function choiceLanguages(
  languages: readonly LanguageStat[],
): LanguageStat[] {
  return languages
    .filter((language) => language.submissions > 0)
    .toSorted(
      (a, b) =>
        b.submissions - a.submissions || a.name.localeCompare(b.name, "pt-BR"),
    );
}

/** Every catalog entry, ordered by how many exercises allow it. */
export function allowanceLanguages(
  languages: readonly LanguageStat[],
): LanguageStat[] {
  return languages.toSorted(
    (a, b) =>
      b.exercises_offered - a.exercises_offered ||
      a.name.localeCompare(b.name, "pt-BR"),
  );
}

/**
 * Performance rows keep every language: a dash for a type with no comparable
 * test case is information too. Reliable samples come first, then small
 * samples, then unmeasured types; inside a group the cheapest ratio wins. The
 * ordering keeps a one-case outlier from leading the table and reading like a
 * result.
 */
export function performanceLanguages(
  languages: readonly LanguageStat[],
): LanguageStat[] {
  return languages.toSorted((a, b) => {
    const rank = sampleRank(a) - sampleRank(b);
    if (rank !== 0) return rank;

    if (a.relative_cpu === null && b.relative_cpu === null) {
      return a.name.localeCompare(b.name, "pt-BR");
    }
    if (a.relative_cpu === null) return 1;
    if (b.relative_cpu === null) return -1;
    return (
      a.relative_cpu - b.relative_cpu || a.name.localeCompare(b.name, "pt-BR")
    );
  });
}

/** 0 = enough comparable cases, 1 = small sample, 2 = no comparison. */
function sampleRank(language: LanguageStat): number {
  if (language.compared_test_cases >= SMALL_SAMPLE) return 0;
  if (language.compared_test_cases > 0) return 1;
  return 2;
}
