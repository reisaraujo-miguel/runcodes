// Empty by default: Caddy serves the API same-origin, so the app uses the
// relative /api path. An absolute URL is only needed for split deployments.
export const API_BASE_URL = import.meta.env.VITE_API_ENDPOINT ?? "";

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

  const response = await fetch(`${API_BASE_URL}${path}`, {
    credentials: "include",
    headers,
    ...options,
  });

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
