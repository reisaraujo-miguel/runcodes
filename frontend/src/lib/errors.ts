import { ApiRequestError } from "@/lib/api/client";

/**
 * Turns a thrown value into a message safe to render.
 *
 * An `ApiRequestError` already carries the API's own message (e.g. "the deadline
 * for this exercise has passed"), which is more useful than a generic fallback,
 * so it is preferred whenever present. Anything else (a network failure, a
 * timeout with no body) falls back to the caller's copy.
 */
export function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiRequestError) return error.message;
  if (error instanceof Error && error.message) return error.message;
  return fallback;
}
