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

/** Formats an RFC3339 timestamp as a date only (no time). */
export function formatDate(value: string | null | undefined): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString(undefined, { dateStyle: "long" });
}

function pad2(value: number): string {
  return String(value).padStart(2, "0");
}

/**
 * Converts an RFC3339 timestamp to the `YYYY-MM-DD` value a native date input
 * expects, in the user's local timezone. Returns an empty string for a missing
 * or unparseable value.
 */
export function formatDateInput(value: string | null | undefined): string {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return `${String(date.getFullYear())}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())}`;
}

/**
 * Converts an RFC3339 timestamp to the `HH:mm` value a native time input
 * expects, in the user's local timezone. Returns an empty string for a missing
 * or unparseable value.
 */
export function formatTimeInput(value: string | null | undefined): string {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return `${pad2(date.getHours())}:${pad2(date.getMinutes())}`;
}

/**
 * Reports whether a timestamp is the last minute of the day in the user's
 * timezone. It drives the "Fim do dia" shortcut, so seconds are ignored.
 */
export function isEndOfDay(value: string | null | undefined): boolean {
  if (!value) return false;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return false;
  return date.getHours() === 23 && date.getMinutes() === 59;
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
 * Formats a per-case limit, which the API stores as 0 when the exercise left the
 * field empty: the judge reads that as "use the platform default", not as a
 * limit of zero, so it is shown as such.
 */
export function formatLimit(
  value: number,
  render: (value: number) => string,
): string {
  return value > 0 ? render(value) : "padrão";
}

/** Parses the date part of a native date input, rejecting normalized dates. */
function parseDateInput(
  dateOnly: string,
): { year: number; month: number; day: number } | null {
  const [yearPart, monthPart, dayPart] = dateOnly.split("-");
  if (!yearPart || !monthPart || !dayPart) return null;

  const year = Number(yearPart);
  const month = Number(monthPart);
  const day = Number(dayPart);
  if (
    !Number.isInteger(year) ||
    !Number.isInteger(month) ||
    !Number.isInteger(day) ||
    month < 1 ||
    month > 12 ||
    day < 1 ||
    day > 31
  ) {
    return null;
  }

  const date = new Date(year, month - 1, day);
  if (
    Number.isNaN(date.getTime()) ||
    date.getFullYear() !== year ||
    date.getMonth() !== month - 1 ||
    date.getDate() !== day
  ) {
    return null;
  }

  return { year, month, day };
}

/** Parses the time part of a native time input (`HH:mm`). */
function parseTimeInput(
  time: string,
): { hours: number; minutes: number } | null {
  const [hoursPart, minutesPart] = time.split(":");
  if (!hoursPart || !minutesPart) return null;

  const hours = Number(hoursPart);
  const minutes = Number(minutesPart);
  if (
    !Number.isInteger(hours) ||
    !Number.isInteger(minutes) ||
    hours < 0 ||
    hours > 23 ||
    minutes < 0 ||
    minutes > 59
  ) {
    return null;
  }

  return { hours, minutes };
}

/**
 * Converts date and time input values into an ISO 8601 timestamp in the user's
 * local timezone. With `endOfDay`, the last second of the chosen day is used
 * (the "Fim do dia" shortcut); otherwise the selected time is used with zero
 * seconds. Returns null for malformed or impossible dates/times.
 */
export function dateTimeInputToTimestamp(
  dateOnly: string,
  time: string,
  endOfDay = false,
): string | null {
  const dateParts = parseDateInput(dateOnly);
  if (!dateParts) return null;

  let hours: number;
  let minutes: number;
  let seconds: number;

  if (endOfDay) {
    hours = 23;
    minutes = 59;
    seconds = 59;
  } else {
    const timeParts = parseTimeInput(time);
    if (!timeParts) return null;
    hours = timeParts.hours;
    minutes = timeParts.minutes;
    seconds = 0;
  }

  const date = new Date(
    dateParts.year,
    dateParts.month - 1,
    dateParts.day,
    hours,
    minutes,
    seconds,
  );
  if (Number.isNaN(date.getTime())) return null;
  return date.toISOString();
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
  return dateTimeInputToTimestamp(
    dateOnly,
    boundary === "end" ? "23:59" : "00:00",
    boundary === "end",
  );
}
