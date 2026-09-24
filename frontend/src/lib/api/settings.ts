import { apiGet } from "./client";

/**
 * The platform settings the frontend renders: the contact address and the
 * disclaimer shown on the login page. Both are editable by an admin.
 */
export interface PlatformSettings {
  contact_email: string;
  contact_disclaimer_html: string;
}

/**
 * Build-time values, used before the API answers and when it is unreachable, so
 * the login page never renders an empty contact disclaimer.
 */
export const FALLBACK_SETTINGS: PlatformSettings = {
  contact_email: import.meta.env.VITE_CONTACT_EMAIL ?? "contact@example.com",
  contact_disclaimer_html: import.meta.env.VITE_CONTACT_DISCLAIMER_HTML ?? "",
};

/** Fetch the platform settings shown on the public pages. */
export function getPublicSettings(): Promise<PlatformSettings> {
  return apiGet<PlatformSettings>("/api/v1/settings/public");
}
