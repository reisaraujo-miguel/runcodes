import { apiGet, apiPost } from "./client";

export interface LoginPayload {
  email: string;
  password: string;
}

export interface LoginResponse {
  // Empty on success — the backend sets the session cookie.
  [key: string]: unknown;
}

export interface SignUpPayload {
  name: string;
  email: string;
  password: string;
  password_confirmation: string;
}

/** Check whether the current session is authenticated. */
export function checkAuth(): Promise<LoginResponse> {
  return apiGet<LoginResponse>("/api/v1/auth");
}

/** Send login credentials and obtain a session cookie. */
export function login(payload: LoginPayload): Promise<LoginResponse> {
  return apiPost<LoginResponse>("/api/v1/user/login", payload);
}

/** Register a new user account. */
export function signUp(payload: SignUpPayload): Promise<void> {
  return apiPost<void>("/api/v1/user/signup", payload);
}

