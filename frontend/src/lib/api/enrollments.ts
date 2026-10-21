import { apiDelete, apiGet, apiPost } from "./client";
import type { CommitStatus } from "./submissions";

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

/** One visible exercise with the caller's own submission summary. */
export interface UserOfferingExercise {
  id: number;
  offering_id: number;
  title: string;
  description: string;
  deadline: string;
  open_date: string;
  /** How many submissions the caller made for this exercise. */
  submission_count: number;
  last_submission_at: string | null;
  last_status: CommitStatus | null;
  /** Best score among graded submissions; null until one is graded. */
  best_score: number | null;
}

/** A class the caller belongs to, with their progress on each exercise. */
export interface UserOfferingDetail {
  id: number;
  name: string;
  description: string;
  end_date: string;
  role: EnrollmentRole;
  owner_name: string;
  is_owner: boolean;
  member_count: number;
  exercises: UserOfferingExercise[];
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

/**
 * Fetch one class the caller belongs to, with their own exercise progress:
 * submission count, latest status and best score per exercise.
 */
export function getMyOffering(id: number): Promise<UserOfferingDetail> {
  return apiGet<UserOfferingDetail>(`/api/v1/user/offerings/${String(id)}`);
}

/** List the exercises that are open right now across the caller's classes. */
export function getMyOpenExercises(): Promise<OpenExercise[]> {
  return apiGet<OpenExercise[]>("/api/v1/user/exercises");
}
