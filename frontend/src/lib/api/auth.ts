import { apiGet, apiPost } from "./client";

export interface LoginPayload {
  email: string;
  password: string;
}

/** Empty on success — the backend sets the session cookie. */
export type LoginResponse = Record<string, unknown>;

export interface SignUpPayload {
  name: string;
  email: string;
  password: string;
  password_confirmation: string;
}

/** Check whether the current session is authenticated. */
export function checkAuth(): Promise<void> {
  return apiGet<undefined>("/api/v1/auth").then(() => undefined);
}

/** Send login credentials and obtain a session cookie. */
export function login(payload: LoginPayload): Promise<LoginResponse> {
  return apiPost<LoginResponse>("/api/v1/user/login", payload);
}

/** Register a new user account. */
export function signUp(payload: SignUpPayload): Promise<void> {
  return apiPost<undefined>("/api/v1/user/signup", payload);
}
