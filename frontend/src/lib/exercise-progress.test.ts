import { describe, expect, test } from "bun:test";

import type { UserExercise } from "@/lib/api";

import {
  completedExercises,
  exerciseProgress,
  formatScore,
  hasPassed,
  pendingExercises,
} from "./exercise-progress";

function exercise(overrides: Partial<UserExercise> = {}): UserExercise {
  return {
    id: 1,
    offering_id: 1,
    offering_name: "CS101",
    title: "Hello, world",
    description: "",
    deadline: "2026-11-10T23:59:00Z",
    open_date: "2026-11-01T00:00:00Z",
    is_open: true,
    submission_count: 0,
    last_submission_at: null,
    last_status: null,
    best_score: null,
    ...overrides,
  };
}

describe("hasPassed", () => {
  test("passes only when the best score is a full 100", () => {
    expect(hasPassed(exercise({ best_score: 100 }))).toBe(true);
    expect(hasPassed(exercise({ best_score: 99.99 }))).toBe(false);
    expect(hasPassed(exercise({ best_score: 0 }))).toBe(false);
    expect(hasPassed(exercise({ best_score: null }))).toBe(false);
  });
});

describe("formatScore", () => {
  test("drops trailing zeros without losing a real fraction", () => {
    expect(formatScore(100)).toBe("100");
    expect(formatScore(87.5)).toBe("87.5");
    expect(formatScore(66.66666666666667)).toBe("66.67");
    expect(formatScore(0)).toBe("0");
  });
});

describe("exerciseProgress", () => {
  test("marks a full score as passed", () => {
    const progress = exerciseProgress(
      exercise({ best_score: 100, submission_count: 1 }),
    );
    expect(progress).toMatchObject({
      state: "passed",
      label: "100/100",
      tone: "success",
      passed: true,
    });
  });

  test("shows the best score out of 100 for a partial result", () => {
    const progress = exerciseProgress(
      exercise({ best_score: 87.5, submission_count: 2 }),
    );
    expect(progress).toMatchObject({
      state: "scored",
      label: "87.5/100",
      tone: "warning",
      passed: false,
    });
  });

  test("says a first run is still being judged", () => {
    const progress = exerciseProgress(
      exercise({ submission_count: 1, last_status: "running" }),
    );
    expect(progress).toMatchObject({
      state: "pending",
      label: "Em correção",
      tone: "info",
      passed: false,
    });
  });

  test("does not claim an unscored terminal submission is missing", () => {
    const progress = exerciseProgress(
      exercise({ submission_count: 1, last_status: "compilation_error" }),
    );
    expect(progress).toMatchObject({
      state: "submitted",
      label: "Sem nota",
      passed: false,
    });
  });

  test("says explicitly when there is no submission", () => {
    expect(exerciseProgress(exercise())).toMatchObject({
      state: "unsubmitted",
      label: "Sem submissão",
      tone: "neutral",
      passed: false,
    });
  });
});

describe("exercise lists", () => {
  test("pending holds open exercises that are not fully passed", () => {
    const items = [
      exercise({ id: 1, is_open: true, best_score: null }),
      exercise({ id: 2, is_open: true, best_score: 70 }),
      exercise({ id: 3, is_open: true, best_score: 100 }),
      exercise({ id: 4, is_open: false, best_score: 40 }),
    ];

    expect(pendingExercises(items).map((item) => item.id)).toEqual([1, 2]);
  });

  test("completed holds passed exercises even after the deadline", () => {
    const items = [
      exercise({ id: 1, is_open: true, best_score: 100 }),
      exercise({ id: 2, is_open: false, best_score: 100 }),
      exercise({ id: 3, is_open: false, best_score: 80 }),
    ];

    expect(completedExercises(items).map((item) => item.id)).toEqual([1, 2]);
  });
});
