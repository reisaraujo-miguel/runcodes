const API_BASE_URL = import.meta.env.VITE_API_ENDPOINT as string;

/** Standard error shape returned by the API. */
export interface ApiError {
  error_msg: string;
}

/** A typed wrapper around fetch that always sends credentials and handles JSON parsing. */
export async function apiRequest<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
    ...options,
  });

  if (!response.ok) {
    let message = "Erro desconhecido";
    try {
      const body: unknown = await response.json();
      if (
        typeof body === "object" &&
        body !== null &&
        "error_msg" in body &&
        typeof (body as ApiError).error_msg === "string"
      ) {
        message = (body as ApiError).error_msg;
      }
    } catch {
      // Non-JSON response — keep default message
    }
    throw new Error(message);
  }

  return response.json() as Promise<T>;
}

/** GET helper. */
export function apiGet<T>(path: string): Promise<T> {
  return apiRequest<T>(path, { method: "GET" });
}

/** POST helper. */
export function apiPost<T>(
  path: string,
  body: unknown,
): Promise<T> {
  return apiRequest<T>(path, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

