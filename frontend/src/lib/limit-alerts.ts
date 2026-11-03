import type { LimitAlertCase } from "@/lib/api/exercises";

/** Display names of the limits a case can be interrupted by. */
const LIMIT_LABELS: Record<string, string> = {
  timeout: "tempo limite",
  cpu_time: "tempo de CPU",
  memory: "memória",
  stack: "pilha",
  file_size: "tamanho de arquivo",
};

/** The display name of a limit, falling back to the stored name. */
export function limitLabel(limit: string): string {
  return LIMIT_LABELS[limit] ?? limit;
}

/** How many affected cases an alert names before it summarizes the rest. */
export const ALERT_CASE_LIMIT = 5;

/**
 * Names the test cases a limit alert's failures concentrated in, worst first,
 * the way the alert email does. Returns null when the alert has no case detail,
 * so the caller can leave that sentence out.
 */
export function formatLimitAlertCases(
  cases: readonly LimitAlertCase[],
): string | null {
  if (cases.length === 0) return null;

  const shown = cases.slice(0, ALERT_CASE_LIMIT);
  const text = shown
    .map(
      (affected) =>
        `caso ${String(affected.number)} (${String(affected.failures)} ${
          affected.failures === 1 ? "submissão" : "submissões"
        })`,
    )
    .join(", ");

  const remaining = cases.length - shown.length;
  if (remaining === 0) return text;
  return `${text} e mais ${String(remaining)} ${
    remaining === 1 ? "caso" : "casos"
  }`;
}
