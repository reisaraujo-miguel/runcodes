import { apiGet, apiPost } from "./client";

export interface CreateOfferingPayload {
  name: string;
  end_date?: string;
  description?: string;
}

/** A class offering as returned by the API. */
export interface Offering {
  id: number;
  name: string;
  end_date: string;
  description: string;
  enrollment_code: string;
}

/** Create a new class / offering for a professor. */
export function createOffering(
  payload: CreateOfferingPayload,
): Promise<Offering> {
  return apiPost<Offering>("/api/v1/offerings/create", payload);
}

/** Fetch a class offering by id. */
export function getOffering(id: number): Promise<Offering> {
  return apiGet<Offering>(`/api/v1/offerings/${String(id)}`);
}
