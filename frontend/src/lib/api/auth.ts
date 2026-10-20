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

/** Register a new user account. The account starts unverified; the API emails a link. */
export function signUp(payload: SignUpPayload): Promise<void> {
  return apiPost<undefined>("/api/v1/user/signup", payload);
}

/**
 * Confirm an account with the token from the verification email. The endpoint
 * is public: the link is often opened in a browser with no session.
 */
export function verifyEmail(token: string): Promise<void> {
  return apiPost<undefined>("/api/v1/user/verify-email", { token });
}

/**
 * Ask for a new verification link. The API always answers the same way, so the
 * UI must not promise that an account exists for the address.
 */
export function resendVerification(email: string): Promise<void> {
  return apiPost<undefined>("/api/v1/user/resend-verification", { email });
}

/**
 * End the session. The backend clears the HttpOnly session cookie, which is the
 * only way to drop it since JavaScript cannot read or delete it.
 */
export function logout(): Promise<void> {
  return apiPost<undefined>("/api/v1/user/logout");
}
