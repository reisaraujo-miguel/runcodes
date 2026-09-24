import { apiDelete, apiGet, apiPost, apiPut } from "./client";
import type { EnrollmentRole } from "./enrollments";

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
  visible_to_enroll: boolean;
}

/** A class the caller manages: one they own, or one they teach. */
export interface ManagedOffering {
  id: number;
  name: string;
  description: string;
  end_date: string;
  enrollment_code: string;
  visible_to_enroll: boolean;
  owner_id: number;
  is_owner: boolean;
  member_count: number;
  exercise_count: number;
  enrollment_open: boolean;
}

/** Fields a professor may change on a class they own. */
export interface UpdateOfferingPayload {
  name?: string;
  end_date?: string;
  description?: string;
  visible_to_enroll?: boolean;
}

export type MemberRole = "professor" | "monitor";

/** A person enrolled in a class: a student, a monitor or a co-professor. */
export interface OfferingMember {
  user_id: number;
  name: string;
  email: string;
  role: EnrollmentRole;
  banned: boolean;
  created_at: string;
}

/** Create a new class / offering for a professor. */
export function createOffering(
  payload: CreateOfferingPayload,
): Promise<Offering> {
  return apiPost<Offering>("/api/v1/offerings/create", payload);
}

/** Fetch a class offering by id (owner, or a professor assigned to it). */
export function getOffering(id: number): Promise<Offering> {
  return apiGet<Offering>(`/api/v1/offerings/${String(id)}`);
}

/** List the classes the caller owns or teaches. */
export function listOfferings(): Promise<ManagedOffering[]> {
  return apiGet<ManagedOffering[]>("/api/v1/offerings");
}

/** Update a class owned by the caller. */
export function updateOffering(
  id: number,
  payload: UpdateOfferingPayload,
): Promise<Offering> {
  return apiPut<Offering>(`/api/v1/offerings/${String(id)}`, payload);
}

/** Delete a class owned by the caller, with its exercises and enrollments. */
export function deleteOffering(id: number): Promise<void> {
  return apiDelete(`/api/v1/offerings/${String(id)}`);
}

/** List the members of a class owned by the caller. */
export function listOfferingMembers(id: number): Promise<OfferingMember[]> {
  return apiGet<OfferingMember[]>(`/api/v1/offerings/${String(id)}/members`);
}

/** Assign a monitor or a co-professor to a class owned by the caller. */
export function addOfferingMember(
  id: number,
  email: string,
  role: MemberRole,
): Promise<OfferingMember> {
  return apiPost<OfferingMember>(`/api/v1/offerings/${String(id)}/members`, {
    email,
    role,
  });
}

/** Change a member's role, or ban and unban them. */
export function updateOfferingMember(
  id: number,
  userId: number,
  payload: { role?: MemberRole; banned?: boolean },
): Promise<OfferingMember> {
  return apiPut<OfferingMember>(
    `/api/v1/offerings/${String(id)}/members/${String(userId)}`,
    payload,
  );
}

/** Remove a member from a class owned by the caller. */
export function removeOfferingMember(
  id: number,
  userId: number,
): Promise<void> {
  return apiDelete(`/api/v1/offerings/${String(id)}/members/${String(userId)}`);
}
