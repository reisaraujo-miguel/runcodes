import { apiPost } from "./client";

export interface CreateOfferingPayload {
  name: string;
  end_date?: string;
  description?: string;
}

export interface CreateOfferingResponse {
  [key: string]: unknown;
}

/** Create a new class / offering for a professor. */
export function createOffering(
  payload: CreateOfferingPayload,
): Promise<CreateOfferingResponse> {
  return apiPost<CreateOfferingResponse>("/api/v1/offerings/create", payload);
}
