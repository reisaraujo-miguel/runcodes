import { apiDelete, apiGet, apiPut } from "./client";
import type { PlatformSettings } from "./settings";

/** A user as listed and edited from the admin panel. */
export interface AdminUser {
  id: number;
  name: string;
  email: string;
  org_id: string;
  role: string;
  confirmed: boolean;
  created_at: string;
}

/** Fields an admin may change on any user. */
export interface UpdateUserPayload {
  name?: string;
  email?: string;
  org_id?: string;
  role?: string;
  confirmed?: boolean;
}

/** A class as listed from the admin panel. */
export interface AdminOffering {
  id: number;
  name: string;
  description: string;
  end_date: string;
  enrollment_code: string;
  visible_to_enroll: boolean;
  owner_id: number | null;
  owner_name: string;
  owner_email: string;
  member_count: number;
  exercise_count: number;
  created_at: string;
}

/** Fields an admin may change on any class, including its owner. */
export interface UpdateAdminOfferingPayload {
  name?: string;
  description?: string;
  end_date?: string;
  visible_to_enroll?: boolean;
  owner_id?: number;
}

/** Roles a user account can hold. */
export const USER_ROLES = ["student", "professor", "admin", "dev"] as const;

/**
 * How many rows a listing asks for. The backend caps a page at 200, so asking
 * for the cap keeps the panel to a single request; a page that comes back full
 * tells the UI to ask for a narrower search.
 */
export const ADMIN_PAGE_LIMIT = 200;

function listQuery(query: string): string {
  const params = new URLSearchParams({ limit: String(ADMIN_PAGE_LIMIT) });
  const trimmed = query.trim();
  if (trimmed !== "") params.set("query", trimmed);
  return `?${params.toString()}`;
}

/** List the platform's users, optionally filtered by a search term. */
export function adminListUsers(query = ""): Promise<AdminUser[]> {
  return apiGet<AdminUser[]>(`/api/v1/admin/users${listQuery(query)}`);
}

/** Update any user of the platform. */
export function adminUpdateUser(
  id: number,
  payload: UpdateUserPayload,
): Promise<AdminUser> {
  return apiPut<AdminUser>(`/api/v1/admin/users/${String(id)}`, payload);
}

/** Delete a user, with the classes they own and their submissions. */
export function adminDeleteUser(id: number): Promise<void> {
  return apiDelete(`/api/v1/admin/users/${String(id)}`);
}

/** List every class on the platform, optionally filtered by a search term. */
export function adminListOfferings(query = ""): Promise<AdminOffering[]> {
  return apiGet<AdminOffering[]>(`/api/v1/admin/offerings${listQuery(query)}`);
}

/** Update any class on the platform, including transferring its ownership. */
export function adminUpdateOffering(
  id: number,
  payload: UpdateAdminOfferingPayload,
): Promise<AdminOffering> {
  return apiPut<AdminOffering>(
    `/api/v1/admin/offerings/${String(id)}`,
    payload,
  );
}

/** Delete any class on the platform. */
export function adminDeleteOffering(id: number): Promise<void> {
  return apiDelete(`/api/v1/admin/offerings/${String(id)}`);
}

/** Read the platform settings the admin panel edits. */
export function adminGetSettings(): Promise<PlatformSettings> {
  return apiGet<PlatformSettings>("/api/v1/admin/settings");
}

/** Store the contact email and the disclaimer shown on the login page. */
export function adminUpdateSettings(
  payload: PlatformSettings,
): Promise<PlatformSettings> {
  return apiPut<PlatformSettings>("/api/v1/admin/settings", payload);
}
