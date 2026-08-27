import { apiGet, apiPost } from "./client";

export type UserRole = "student" | "professor" | "admin";

/** The authenticated user as returned by GET /api/v1/auth. */
export interface AuthUser {
  id: number;
  name: string;
  email: string;
  role: UserRole;
  /** When the session token expires (unix timestamp in seconds). */
  expires_at: number;
}

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

/** Fetch the current session's user info. */
export function checkAuth(): Promise<AuthUser> {
  return apiGet<AuthUser>("/api/v1/auth");
}

/** Renew the current session before it expires (sliding expiration). */
export function refreshSession(): Promise<AuthUser> {
  return apiPost<AuthUser>("/api/v1/auth/refresh");
}

/** Send login credentials and obtain a session cookie. */
export function login(payload: LoginPayload): Promise<LoginResponse> {
  return apiPost<LoginResponse>("/api/v1/user/login", payload);
}

/** Register a new user account. */
export function signUp(payload: SignUpPayload): Promise<void> {
  return apiPost<undefined>("/api/v1/user/signup", payload);
}
