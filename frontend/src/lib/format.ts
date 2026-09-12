/** Formats an RFC3339 timestamp for display, falling back to the raw value. */
export function formatDateTime(value: string | null | undefined): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString(undefined, {
    dateStyle: "long",
    timeStyle: "short",
  });
}

/** Formats a signed byte count; negative values (unavailable) render as "—". */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "—";
  if (bytes < 1024) return `${String(bytes)} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = bytes / 1024;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  return `${value.toFixed(1)} ${units[unitIndex] ?? "TB"}`;
}

/** Formats a CPU time measurement in seconds. */
export function formatCpuTime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return "—";
  return `${seconds.toFixed(3)} s`;
}

/**
 * Converts a date-only value (YYYY-MM-DD) from a date input into an ISO 8601
 * timestamp for the start or end of that day in the user's local timezone.
 * Returns null when the value is not a valid date.
 */
export function dateInputToTimestamp(
  dateOnly: string,
  boundary: "start" | "end",
): string | null {
  const [year, month, day] = dateOnly.split("-");
  if (!year || !month || !day) return null;
  const date =
    boundary === "start"
      ? new Date(Number(year), Number(month) - 1, Number(day), 0, 0, 0)
      : new Date(Number(year), Number(month) - 1, Number(day), 23, 59, 59);
  if (Number.isNaN(date.getTime())) return null;
  return date.toISOString();
}
