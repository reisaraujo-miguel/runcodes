/**
 * Helpers for the admin alerts panel.
 *
 * `alertDetailLine` is what turns the producer's structured `details` into the
 * one-line summary under an alert's message. The judge sets a small, known set of
 * keys; anything else falls back to the raw JSON, so a future producer is still
 * readable without a UI change.
 */

/** Keys the platform's producers set today, in the order the panel shows them. */
const KNOWN_DETAIL_KEYS = [
  "language",
  "image",
  "milestone",
  "wait_seconds",
  "filename",
] as const;

/** Renders one detail value, without ever falling back to `[object Object]`. */
function renderDetailValue(value: unknown): string {
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }

  return JSON.stringify(value);
}

/** Renders one `key=value …` line for an alert's details. */
export function alertDetailLine(
  details: Record<string, unknown> | null | undefined,
): string {
  if (!details) return "";

  const parts = KNOWN_DETAIL_KEYS.filter(
    (key) => details[key] !== undefined && details[key] !== null,
  ).map((key) => `${key}=${renderDetailValue(details[key])}`);

  if (parts.length === 0) {
    return JSON.stringify(details);
  }

  return parts.join(" · ");
}
