import type { UserOfferingExercise } from "@/lib/api";
import { isPendingStatus } from "@/lib/submission-status";

/** Which exercises a student wants to see on a class page. */
export type ClassExerciseFilter = "all" | "open" | "unsubmitted" | "graded";

/** How the visible exercises are ordered. */
export type ClassExerciseSort =
  "deadline" | "score-desc" | "score-asc" | "recent";

/** Aggregate progress for one class, shown next to the exercise list. */
export interface ClassExerciseSummary {
  total: number;
  open: number;
  submitted: number;
  graded: number;
  /** Mean of the best scores; null until at least one exercise is graded. */
  averageScore: number | null;
}

/** True while the exercise's deadline is still ahead. */
export function isExerciseOpen(
  exercise: UserOfferingExercise,
  now = Date.now(),
): boolean {
  const deadline = Date.parse(exercise.deadline);
  return Number.isFinite(deadline) && deadline > now;
}

/** True while the caller's latest submission is still being judged. */
export function isSubmissionPending(exercise: UserOfferingExercise): boolean {
  return exercise.last_status !== null && isPendingStatus(exercise.last_status);
}

function matchesFilter(
  exercise: UserOfferingExercise,
  filter: ClassExerciseFilter,
  now: number,
): boolean {
  switch (filter) {
    case "open":
      return isExerciseOpen(exercise, now);
    case "unsubmitted":
      return exercise.submission_count === 0;
    case "graded":
      return exercise.best_score !== null;
    case "all":
      return true;
  }
}

function compareScores(
  a: UserOfferingExercise,
  b: UserOfferingExercise,
  direction: "asc" | "desc",
): number {
  // Exercises without a grade stay at the end in both directions: they are not
  // "the lowest score", they simply have no score yet.
  if (a.best_score === null && b.best_score === null) return 0;
  if (a.best_score === null) return 1;
  if (b.best_score === null) return -1;
  return direction === "desc"
    ? b.best_score - a.best_score
    : a.best_score - b.best_score;
}

function compareRecent(
  a: UserOfferingExercise,
  b: UserOfferingExercise,
): number {
  const aTime = a.last_submission_at
    ? Date.parse(a.last_submission_at)
    : Number.NaN;
  const bTime = b.last_submission_at
    ? Date.parse(b.last_submission_at)
    : Number.NaN;

  if (Number.isNaN(aTime) && Number.isNaN(bTime)) return 0;
  if (Number.isNaN(aTime)) return 1;
  if (Number.isNaN(bTime)) return -1;
  return bTime - aTime;
}

/**
 * Orders exercises by urgency: the ones still open come first, soonest deadline
 * first, then the closed ones, most recently closed first.
 */
function compareDeadlines(
  a: UserOfferingExercise,
  b: UserOfferingExercise,
  now: number,
): number {
  const aOpen = isExerciseOpen(a, now);
  const bOpen = isExerciseOpen(b, now);
  if (aOpen !== bOpen) return aOpen ? -1 : 1;

  const aDeadline = Date.parse(a.deadline);
  const bDeadline = Date.parse(b.deadline);
  if (!Number.isFinite(aDeadline)) return 1;
  if (!Number.isFinite(bDeadline)) return -1;

  return aOpen ? aDeadline - bDeadline : bDeadline - aDeadline;
}

/** Filters and sorts a class's exercises for the student view. */
export function arrangeExercises(
  exercises: readonly UserOfferingExercise[],
  filter: ClassExerciseFilter,
  sort: ClassExerciseSort,
  now = Date.now(),
): UserOfferingExercise[] {
  const filtered = exercises.filter((exercise) =>
    matchesFilter(exercise, filter, now),
  );
  const byDeadline = (a: UserOfferingExercise, b: UserOfferingExercise) =>
    compareDeadlines(a, b, now);

  return filtered.sort((a, b) => {
    switch (sort) {
      case "score-desc":
        return compareScores(a, b, "desc") || byDeadline(a, b);
      case "score-asc":
        return compareScores(a, b, "asc") || byDeadline(a, b);
      case "recent":
        return compareRecent(a, b) || byDeadline(a, b);
      case "deadline":
        return byDeadline(a, b);
    }
  });
}

/** Counts what the student has done in the class so far. */
export function summarizeExercises(
  exercises: readonly UserOfferingExercise[],
  now = Date.now(),
): ClassExerciseSummary {
  const graded = exercises.filter((exercise) => exercise.best_score !== null);
  const scoreSum = graded.reduce(
    (sum, exercise) => sum + (exercise.best_score ?? 0),
    0,
  );

  return {
    total: exercises.length,
    open: exercises.filter((exercise) => isExerciseOpen(exercise, now)).length,
    submitted: exercises.filter((exercise) => exercise.submission_count > 0)
      .length,
    graded: graded.length,
    averageScore: graded.length > 0 ? scoreSum / graded.length : null,
  };
}
