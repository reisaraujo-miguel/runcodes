import { apiDelete, apiGet, apiPost } from "./client";

/** A user's role inside a class, as stored in the `enrollment_role_t` enum. */
export type EnrollmentRole = "student" | "professor" | "monitor";

/** A class the caller belongs to (or owns), as listed on the home page. */
export interface UserOffering {
  offering_id: number;
  name: string;
  description: string;
  end_date: string;
  role: EnrollmentRole;
  owner_name: string;
  is_owner: boolean;
}

/** An exercise of one of the caller's classes that is open right now. */
export interface OpenExercise {
  id: number;
  offering_id: number;
  offering_name: string;
  title: string;
  description: string;
  deadline: string;
  open_date: string;
}

/** Join a class with the enrollment code its professor shared. */
export function enroll(enrollmentCode: string): Promise<UserOffering> {
  return apiPost<UserOffering>("/api/v1/offerings/enroll", {
    enrollment_code: enrollmentCode,
  });
}

/** Leave a class the caller joined. Owners cannot leave their own class. */
export function unenroll(offeringId: number): Promise<void> {
  return apiDelete(`/api/v1/offerings/${String(offeringId)}/enrollment`);
}

/** List the classes the caller belongs to or owns. */
export function getMyOfferings(): Promise<UserOffering[]> {
  return apiGet<UserOffering[]>("/api/v1/user/offerings");
}

/** List the exercises that are open right now across the caller's classes. */
export function getMyOpenExercises(): Promise<OpenExercise[]> {
  return apiGet<OpenExercise[]>("/api/v1/user/exercises");
}
