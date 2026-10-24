// Empty by default: Caddy serves the API same-origin, so the app uses the
// relative /api path. An absolute URL is only needed for split deployments.
export const API_BASE_URL = import.meta.env.VITE_API_ENDPOINT ?? "";

/**
 * How long a request may take before it is aborted. Without a bound, a request
 * that is accepted but never answered (a wedged proxy, a dropped connection)
 * leaves the UI waiting forever with no way to retry.
 */
export const REQUEST_TIMEOUT_MS = 15_000;

/**
 * Dispatched on the global object whenever the API answers 401. The session
 * cookie may have expired or been revoked by a password/email change; the auth
 * provider listens and clears its user so the route guard redirects instead of
 * leaving the UI looking signed in until the next reload.
 */
export const UNAUTHORIZED_EVENT = "runcodes:unauthorized";

/** A file upload is slower than a JSON round trip, so it gets its own budget. */
export const UPLOAD_TIMEOUT_MS = 120_000;

/**
 * A download may be a whole class archive, so it gets a much larger budget than
 * an ordinary request. The browser can still cancel it by navigating away.
 */
export const DOWNLOAD_TIMEOUT_MS = 10 * 60_000;

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

/**
 * A failed API request. It carries the HTTP status alongside the message the
 * API sent, so a caller can react to the reason (a 403 is not a 404) without
 * matching the message text.
 */
export class ApiRequestError extends Error {
  readonly status: number;
  /** The parsed error body, for endpoints that attach structured details. */
  readonly body: unknown;

  constructor(message: string, status: number, body?: unknown) {
    super(message);
    this.name = "ApiRequestError";
    this.status = status;
    this.body = body;
  }
}

/** True for non-null objects, used to narrow parsed JSON safely. */
export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/**
 * Reads the API's `error_msg` and the raw body of a failed response. Some
 * endpoints attach more than a message (a batch test-case import lists the
 * invalid folders), so the body is kept for callers that need it.
 */
export async function readApiFailure(
  response: Response,
): Promise<{ message: string; body: unknown }> {
  let message = "Erro desconhecido";
  let body: unknown;
  try {
    body = await response.json();
    if (isRecord(body) && typeof body.error_msg === "string") {
      message = body.error_msg;
    }
  } catch {
    // Non-JSON response — keep default message
  }
  return { message, body };
}

/** Reads the API's `error_msg` from a failed response. */
export async function readApiError(response: Response): Promise<string> {
  return (await readApiFailure(response)).message;
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

  if (response.status === 401) {
    globalThis.dispatchEvent(new Event(UNAUTHORIZED_EVENT));
  }

  if (!response.ok) {
    const failure = await readApiFailure(response);
    throw new ApiRequestError(failure.message, response.status, failure.body);
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

/**
 * Uploads a multipart body with the upload timeout. `apiRequest` keeps the
 * ordinary request budget, which is too short for a large archive.
 */
export async function apiUpload<T>(path: string, body: FormData): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${path}`, {
      method: "POST",
      body,
      credentials: "include",
      signal: requestSignal(undefined, UPLOAD_TIMEOUT_MS),
    });
  } catch (error) {
    if (isTimeoutAbort(error)) {
      throw new Error("Tempo esgotado ao enviar o arquivo.", { cause: error });
    }
    throw error;
  }

  if (response.status === 401) {
    globalThis.dispatchEvent(new Event(UNAUTHORIZED_EVENT));
  }

  if (!response.ok) {
    const failure = await readApiFailure(response);
    throw new ApiRequestError(failure.message, response.status, failure.body);
  }

  return readApiBody<T>(response);
}

/** A downloaded file and the server-suggested name. */
export interface DownloadedFile {
  blob: Blob;
  filename: string;
} /**
 * Downloads a file endpoint as a blob. The caller gets the server's suggested
 * file name from Content-Disposition and can hand both to `downloadBlob`.
 */
export async function apiDownload(path: string): Promise<DownloadedFile> {
  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${path}`, {
      credentials: "include",
      signal: requestSignal(undefined, DOWNLOAD_TIMEOUT_MS),
    });
  } catch (error) {
    if (isTimeoutAbort(error)) {
      throw new Error("Tempo esgotado ao baixar o arquivo.", { cause: error });
    }
    throw error;
  }

  if (response.status === 401) {
    globalThis.dispatchEvent(new Event(UNAUTHORIZED_EVENT));
  }

  if (!response.ok) {
    const failure = await readApiFailure(response);
    throw new ApiRequestError(failure.message, response.status, failure.body);
  }

  return {
    blob: await response.blob(),
    filename: filenameFromDisposition(
      response.headers.get("Content-Disposition"),
    ),
  };
}

/** Reads the quoted filename of a Content-Disposition header. */
export function filenameFromDisposition(header: string | null): string {
  if (!header) return "download";
  const match =
    /filename="([^"]+)"/i.exec(header) ?? /filename=([^;]+)/i.exec(header);
  const value = match?.[1]?.trim();
  return value ?? "download";
}
