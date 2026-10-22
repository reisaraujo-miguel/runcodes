import { apiGet, apiPut } from "./client";

/** The caller's own account, as returned by the profile endpoints. */
export interface Profile {
  id: number;
  name: string;
  email: string;
  org_id: string;
  role: string;
  confirmed: boolean;
  code_editor_enabled: boolean;
  created_at: string;
  /** A requested email change waiting for the link sent to the new address. */
  pending_email: string;
}

/** Fields the caller may change on their own account. */
export interface ProfilePayload {
  name?: string;
  email?: string;
  org_id?: string;
  code_editor_enabled?: boolean;
  /** Required only when email changes. */
  current_password?: string;
}

export interface PasswordPayload {
  current_password: string;
  new_password: string;
  new_password_confirmation: string;
}

/** Fetch the caller's own account. */
export function getProfile(): Promise<Profile> {
  return apiGet<Profile>("/api/v1/user/profile");
}

/** Update the caller's own name, email and organization id. */
export function updateProfile(payload: ProfilePayload): Promise<Profile> {
  return apiPut<Profile>("/api/v1/user/profile", payload);
}

/** Replace the caller's password, checking the current one. */
export function changePassword(payload: PasswordPayload): Promise<void> {
  return apiPut<undefined>("/api/v1/user/password", payload);
}
