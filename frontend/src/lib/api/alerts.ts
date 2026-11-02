import { apiGet, apiPost } from "./client";

/** How serious an operational alert is. Mirrors the backend's severities. */
export type AlertSeverity = "warning" | "critical";

/**
 * One operational alert, as the admin panel reads it.
 *
 * The judge raises them for runs it could not complete (a container that never
 * printed its first milestone, for example); the backend is the durable record
 * and also emails every administrator when one is created.
 */
export interface SystemAlert {
  id: number;
  created_at: string;
  severity: AlertSeverity;
  /** Stable machine-readable identifier, e.g. `startup_timeout`. */
  code: string;
  source: "judge" | "backend";
  /** The run the alert is about, when it is about one. */
  commit_id: number | null;
  message: string;
  /** Structured context from the producer (language, image, wait seconds, …). */
  details?: Record<string, unknown> | null;
  acknowledged_at: string | null;
  acknowledged_by: number | null;
  acknowledged_by_name?: string;
}

/** One page of alerts plus the number still open (the panel's badge). */
export interface AlertPage {
  open_count: number;
  alerts: SystemAlert[];
}

/** Page size the panel asks for; the backend caps it at 200. */
export const ALERT_PAGE_LIMIT = 100;

/**
 * Lists the platform's operational alerts, newest first.
 */
export function adminListAlerts(
  options: { onlyOpen?: boolean; limit?: number; offset?: number } = {},
): Promise<AlertPage> {
  const params = new URLSearchParams({
    limit: String(options.limit ?? ALERT_PAGE_LIMIT),
    offset: String(Math.max(0, options.offset ?? 0)),
  });
  if (options.onlyOpen) params.set("open", "1");

  return apiGet<AlertPage>(`/api/v1/admin/alerts?${params.toString()}`);
}

/**
 * Marks one alert as seen by the calling administrator. Idempotent: a second
 * call returns the alert with its original acknowledgement.
 */
export function adminAcknowledgeAlert(id: number): Promise<SystemAlert> {
  return apiPost<SystemAlert>(`/api/v1/admin/alerts/${String(id)}/acknowledge`);
}
