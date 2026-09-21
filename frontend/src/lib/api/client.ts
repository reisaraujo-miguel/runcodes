// Empty by default: Caddy serves the API same-origin, so the app uses the
// relative /api path. An absolute URL is only needed for split deployments.
export const API_BASE_URL = import.meta.env.VITE_API_ENDPOINT ?? "";

/**
 * How long a request may take before it is aborted. Without a bound, a request
 * that is accepted but never answered (a wedged proxy, a dropped connection)
 * leaves the UI waiting forever with no way to retry.
 */
export const REQUEST_TIMEOUT_MS = 15_000;

/** A file upload is slower than a JSON round trip, so it gets its own budget. */
export const UPLOAD_TIMEOUT_MS = 120_000;

/**
 * Bounds a request with a timeout while still honouring a caller's own signal,
 * so a component can cancel on unmount as well.
 */
export function requestSignal(
  signal: AbortSignal | null | undefined,
  timeoutMs: number = REQUEST_TIMEOUT_MS,
): AbortSignal {
  const timeout = AbortSignal.timeout(timeoutMs);
  return signal ? AbortSignal.any([signal, timeout]) : timeout;
}

/** True when a thrown value is the abort caused by our own timeout. */
function isTimeoutAbort(error: unknown): boolean {
  return error instanceof DOMException && error.name === "TimeoutError";
}

/** Standard error shape returned by the API. */
export interface ApiError {
  error_msg: string;
}

/** True for non-null objects, used to narrow parsed JSON safely. */
export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/** Reads the API's `error_msg` from a failed response. */
export async function readApiError(response: Response): Promise<string> {
  let message = "Erro desconhecido";
  try {
    const body: unknown = await response.json();
    if (isRecord(body) && typeof body.error_msg === "string") {
      message = body.error_msg;
    }
  } catch {
    // Non-JSON response — keep default message
  }
  return message;
}

/**
 * Parses a JSON response body, or `undefined` for an empty body (some
 * endpoints return a success status with no content, e.g. DELETE).
 */
export async function readApiBody<T>(response: Response): Promise<T> {
  const text = await response.text();
  return (text ? JSON.parse(text) : undefined) as T;
}

/** A typed wrapper around fetch that always sends credentials and handles JSON parsing. */
export async function apiRequest<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers);
  // FormData bodies must let the browser set the multipart boundary itself.
  if (!(options.body instanceof FormData)) {
    headers.set("Content-Type", "application/json");
  }

  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${path}`, {
      // Spread first: a caller-supplied header set must be merged into `headers`
      // rather than replacing the Content-Type set above.
      ...options,
      credentials: "include",
      headers,
      signal: requestSignal(options.signal),
    });
  } catch (error) {
    if (isTimeoutAbort(error)) {
      throw new Error("Tempo esgotado ao contatar o servidor.", {
        cause: error,
      });
    }
    throw error;
  }

  if (!response.ok) {
    throw new Error(await readApiError(response));
  }

  return readApiBody<T>(response);
}

/** GET helper. */
export function apiGet<T>(path: string): Promise<T> {
  return apiRequest<T>(path, { method: "GET" });
}

/** POST helper. */
export function apiPost<T>(path: string, body?: unknown): Promise<T> {
  return apiRequest<T>(path, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

/** PUT helper. */
export function apiPut<T>(path: string, body?: unknown): Promise<T> {
  return apiRequest<T>(path, {
    method: "PUT",
    body: JSON.stringify(body),
  });
}

/** DELETE helper. */
export function apiDelete<T = void>(path: string): Promise<T> {
  return apiRequest<T>(path, { method: "DELETE" });
}
