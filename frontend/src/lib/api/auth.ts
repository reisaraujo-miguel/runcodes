import { apiGet, apiPost } from "./client";

export interface LoginPayload {
  email: string;
  password: string;
}

export interface LoginResponse {
  // Empty on success — the backend sets the session cookie.
  [key: string]: unknown;
}

/** Check whether the current session is authenticated. */
export function checkAuth(): Promise<LoginResponse> {
  return apiGet<LoginResponse>("/api/v1/auth");
}

/** Send login credentials and obtain a session cookie. */
export function login(payload: LoginPayload): Promise<LoginResponse> {
  return apiPost<LoginResponse>("/api/v1/user/login", payload);
}

