/** Formats one test-case limit for the professor, in the unit each is stored in. */

const MIB = 1024 * 1024;

/** Renders a byte limit for an inherited value, in MiB. */
export function formatLimitMiB(value: number): string {
  if (value === 0) return "sem limite";
  return `${String(Math.round((value / MIB) * 100) / 100)} MiB`;
}

/** Renders a CPU time limit for an inherited value. */
export function formatLimitSeconds(value: number): string {
  return `${String(value)} s`;
}

/** The layer an inherited value comes from, as the professor reads it. */
export function sourceLabel(source: string): string {
  switch (source) {
    case "exercise":
      return "exercício";
    case "case":
      return "caso";
    default:
      return "plataforma";
  }
}
