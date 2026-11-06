import type { UserExercise } from "@/lib/api";
import { isPendingStatus } from "@/lib/submission-status";

/** The judge scores an exercise as the percentage of correct test cases. */
export const PASSING_SCORE = 100;

/** What the caller has done on an exercise, from their own point of view. */
export type ExerciseProgressState =
  | "passed"
  | "scored"
  | "pending"
  | "submitted"
  | "unsubmitted";

export interface ExerciseProgress {
  state: ExerciseProgressState;
  /** Short text for a badge, e.g. "Sem submissão" or "87.5/100". */
  label: string;
  tone: "neutral" | "info" | "success" | "warning";
  passed: boolean;
}

/**
 * True once the student has passed every test case. The score is a percentage,
 * so 100 is the only passing value; comparing with `>=` keeps a future rounding
 * change from turning a full score into a failure.
 */
export function hasPassed(exercise: Pick<UserExercise, "best_score">): boolean {
  return exercise.best_score !== null && exercise.best_score >= PASSING_SCORE;
}

/**
 * Formats a score without trailing zeros so badges read "87.5/100" instead of
 * "87.50/100", while a fractional score such as 66.666… still shows 66.67.
 */
export function formatScore(score: number): string {
  if (!Number.isFinite(score)) return "—";
  return score.toFixed(2).replace(/\.?0+$/, "");
}

/**
 * Describes the caller's progress on an exercise for a card badge. A submission
 * still being judged has no score yet on its first run, so it says so instead of
 * claiming there was no submission.
 */
export function exerciseProgress(
  exercise: Pick<
    UserExercise,
    "best_score" | "last_status" | "submission_count"
  >,
): ExerciseProgress {
  if (hasPassed(exercise)) {
    return {
      state: "passed",
      label: `${String(PASSING_SCORE)}/100`,
      tone: "success",
      passed: true,
    };
  }

  if (exercise.best_score !== null) {
    return {
      state: "scored",
      label: `${formatScore(exercise.best_score)}/100`,
      tone: "warning",
      passed: false,
    };
  }

  if (exercise.submission_count > 0) {
    const pending =
      exercise.last_status !== null && isPendingStatus(exercise.last_status);
    return pending
      ? {
          state: "pending",
          label: "Em correção",
          tone: "info",
          passed: false,
        }
      : {
          state: "submitted",
          label: "Sem nota",
          tone: "neutral",
          passed: false,
        };
  }

  return {
    state: "unsubmitted",
    label: "Sem submissão",
    tone: "neutral",
    passed: false,
  };
}

/**
 * The exercises still worth solving: open now and not fully passed. This is
 * what the home page lists and what the "Em aberto" tab shows.
 */
export function pendingExercises(
  exercises: readonly UserExercise[],
): UserExercise[] {
  return exercises.filter(
    (exercise) => exercise.is_open && !hasPassed(exercise),
  );
}

/**
 * The exercises the student has fully passed, whether the deadline has passed
 * or not. They move to the "Concluídos" tab and leave the home page.
 */
export function completedExercises(
  exercises: readonly UserExercise[],
): UserExercise[] {
  return exercises.filter(hasPassed);
}
