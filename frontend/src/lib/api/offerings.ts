import {
  apiDelete,
  apiDownload,
  apiGet,
  apiPost,
  apiPut,
  apiUpload,
  type DownloadedFile,
} from "./client";
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

/** Replace the enrollment code of a class owned by the caller. */
export function rotateEnrollmentCode(id: number): Promise<Offering> {
  return apiPost<Offering>(
    `/api/v1/offerings/${String(id)}/enrollment-code/rotate`,
    {},
  );
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

/** One column of the grade table: an exercise of the class. */
export interface GradebookExercise {
  id: number;
  title: string;
  deadline: string;
}

/** One student's result on one exercise; `score` is null without a submission. */
export interface GradebookCell {
  exercise_id: number;
  score: number | null;
  submission_count: number;
  last_submission_at: string | null;
}

/** One row of the grade table: a student and their offering grade. */
export interface GradebookStudent {
  user_id: number;
  name: string;
  email: string;
  scores: GradebookCell[];
  average: number;
  submission_count: number;
  last_submission_at: string | null;
}

/** Aggregated numbers for the whole class. */
export interface GradeSummary {
  student_count: number;
  students_with_submission: number;
  students_above_50: number;
  global_average: number;
  total_submissions: number;
}

/** Aggregated numbers for one exercise. */
export interface ExerciseStatistics {
  exercise_id: number;
  title: string;
  deadline: string;
  student_count: number;
  submitted_students: number;
  students_above_50: number;
  /** Mean over every student, counting missing submissions as zero. */
  average: number;
  /** Mean among the students who submitted; null when nobody did. */
  average_submitted: number | null;
  approval_rate: number;
  best_score: number | null;
  lowest_score: number | null;
}

/** The full grade table plus the statistics derived from it. */
export interface OfferingGradebook {
  offering_id: number;
  offering_name: string;
  exercises: GradebookExercise[];
  students: GradebookStudent[];
  summary: GradeSummary;
  exercise_statistics: ExerciseStatistics[];
}

/** Fetch the class grade table and its statistics (owner or co-professor). */
export function getOfferingGradebook(id: number): Promise<OfferingGradebook> {
  return apiGet<OfferingGradebook>(`/api/v1/offerings/${String(id)}/grades`);
}

/** Download the grade table as a CSV file. */
export function downloadOfferingGradesCsv(id: number): Promise<DownloadedFile> {
  return apiDownload(`/api/v1/offerings/${String(id)}/grades.csv`);
}

/** Download the whole class as a reusable zip archive. */
export function downloadOfferingArchive(id: number): Promise<DownloadedFile> {
  return apiDownload(`/api/v1/offerings/${String(id)}/export`);
}

export interface ImportOfferingOptions {
  /** Optional override for the archived class name. */
  name?: string;
  /** Optional override for the archived description. */
  description?: string;
  /** Optional end date (RFC3339). Exercises shift by the same delta. */
  end_date?: string;
}

/**
 * Create a new class from a previously exported archive. The caller owns the new
 * class; an end date moves every exercise by the same amount so the schedule is
 * preserved when a course is reused in another semester.
 */
export function importOffering(
  file: File,
  options: ImportOfferingOptions = {},
): Promise<Offering> {
  const form = new FormData();
  form.append("archive", file);
  if (options.name) form.append("name", options.name);
  if (options.description !== undefined) {
    form.append("description", options.description);
  }
  if (options.end_date) form.append("end_date", options.end_date);
  return apiUpload<Offering>("/api/v1/offerings/import", form);
}

/** Download the newest submission of every student on every exercise. */
export function downloadOfferingSubmissions(
  id: number,
): Promise<DownloadedFile> {
  return apiDownload(`/api/v1/offerings/${String(id)}/submissions/export`);
}

export interface RecompileResult {
  queued: number;
  failed: number;
}

/** Queue a fresh run of every student's newest submission on every exercise. */
export function recompileOfferingSubmissions(
  id: number,
): Promise<RecompileResult> {
  return apiPost<RecompileResult>(
    `/api/v1/offerings/${String(id)}/submissions/recompile`,
    {},
  );
}

/** Send an email to every member of the class. */
export function notifyOfferingMembers(
  id: number,
  subject: string,
  message: string,
): Promise<{ recipients: number }> {
  return apiPost<{ recipients: number }>(
    `/api/v1/offerings/${String(id)}/notify`,
    { subject, message },
  );
}
