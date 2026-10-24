import { apiGet, apiPost } from "./client";

export type UserRole = "student" | "professor" | "admin";

/** A reduced role an administrator may preview on their own account. */
export type PreviewRole = "student" | "professor";

/** The authenticated user as returned by GET /api/v1/auth. */
export interface AuthUser {
  id: number;
  name: string;
  email: string;
  role: UserRole;
  /** When the session token expires (unix timestamp in seconds). */
  expires_at: number;
  /**
   * Present only while an administrator previews the platform with a reduced
   * role. The account and its data are still the admin's own.
   */
  previewing_role?: PreviewRole | null;
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
 * Ask for a password reset link. The API always answers the same way, so the UI
 * must not promise that an account exists for the address.
 */
export function forgotPassword(email: string): Promise<void> {
  return apiPost<undefined>("/api/v1/user/forgot-password", { email });
}

export interface ResetPasswordPayload {
  token: string;
  password: string;
  password_confirmation: string;
}

/**
 * Replace the password using the token from the recovery email. The token is
 * single use and short lived; an invalid one is reported as a 400.
 */
export function resetPassword(payload: ResetPasswordPayload): Promise<void> {
  return apiPost<undefined>("/api/v1/user/reset-password", payload);
}

/**
 * End the session. The backend clears the HttpOnly session cookie, which is the
 * only way to drop it since JavaScript cannot read or delete it.
 */
export function logout(): Promise<void> {
  return apiPost<undefined>("/api/v1/user/logout");
}

/**
 * Start previewing the platform with a student or professor role. The session
 * stays on the administrator's own account; only the role claim changes, so the
 * caller must refresh the auth state afterwards.
 */
export function adminPreview(role: PreviewRole): Promise<AuthUser> {
  return apiPost<AuthUser>("/api/v1/admin/preview", { role });
}

/** End the role preview and restore the administrator's real role. */
export function stopPreview(): Promise<AuthUser> {
  return apiPost<AuthUser>("/api/v1/auth/stop-preview");
}
