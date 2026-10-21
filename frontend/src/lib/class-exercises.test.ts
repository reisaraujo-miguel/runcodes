import { describe, expect, test } from "bun:test";

import type { UserOfferingExercise } from "@/lib/api";

import {
  arrangeExercises,
  isExerciseOpen,
  isSubmissionPending,
  summarizeExercises,
} from "./class-exercises";

const NOW = Date.parse("2026-10-21T12:00:00Z");
const HOUR = 60 * 60 * 1000;
const DAY = 24 * HOUR;

function exercise(
  id: number,
  overrides: Partial<UserOfferingExercise> = {},
): UserOfferingExercise {
  return {
    id,
    offering_id: 1,
    title: `Exercício ${String(id)}`,
    description: "",
    deadline: new Date(NOW + DAY).toISOString(),
    open_date: new Date(NOW - DAY).toISOString(),
    submission_count: 0,
    last_submission_at: null,
    last_status: null,
    best_score: null,
    ...overrides,
  };
}

describe("isExerciseOpen", () => {
  test("is true only before the deadline", () => {
    expect(isExerciseOpen(exercise(1), NOW)).toBe(true);
    expect(
      isExerciseOpen(
        exercise(1, { deadline: new Date(NOW - 1).toISOString() }),
        NOW,
      ),
    ).toBe(false);
  });

  test("treats an unparseable deadline as closed", () => {
    expect(isExerciseOpen(exercise(1, { deadline: "not-a-date" }), NOW)).toBe(
      false,
    );
  });
});

describe("isSubmissionPending", () => {
  test("is true while the latest submission is in progress", () => {
    expect(isSubmissionPending(exercise(1, { last_status: "queued" }))).toBe(
      true,
    );
    expect(isSubmissionPending(exercise(1, { last_status: "running" }))).toBe(
      true,
    );
  });

  test("is false without a submission or after it settles", () => {
    expect(isSubmissionPending(exercise(1))).toBe(false);
    expect(isSubmissionPending(exercise(1, { last_status: "completed" }))).toBe(
      false,
    );
  });
});

describe("arrangeExercises", () => {
  test("filters to exercises that can still receive a submission", () => {
    const list = [
      exercise(1),
      exercise(2, { deadline: new Date(NOW - HOUR).toISOString() }),
    ];

    expect(
      arrangeExercises(list, "open", "deadline", NOW).map((e) => e.id),
    ).toEqual([1]);
  });

  test("filters to exercises without any submission", () => {
    const list = [
      exercise(1),
      exercise(2, { submission_count: 1, last_status: "queued" }),
    ];

    expect(
      arrangeExercises(list, "unsubmitted", "deadline", NOW).map((e) => e.id),
    ).toEqual([1]);
  });

  test("filters to exercises that already have a grade", () => {
    const list = [
      exercise(1),
      exercise(2, { submission_count: 1, best_score: 0 }),
    ];

    expect(
      arrangeExercises(list, "graded", "deadline", NOW).map((e) => e.id),
    ).toEqual([2]);
  });

  test("orders open exercises first and closed ones most recently closed first", () => {
    const list = [
      exercise(1, { deadline: new Date(NOW - DAY).toISOString() }),
      exercise(2, { deadline: new Date(NOW + 2 * DAY).toISOString() }),
      exercise(3, { deadline: new Date(NOW - 2 * DAY).toISOString() }),
      exercise(4, { deadline: new Date(NOW + DAY).toISOString() }),
    ];

    expect(
      arrangeExercises(list, "all", "deadline", NOW).map((e) => e.id),
    ).toEqual([4, 2, 1, 3]);
  });

  test("orders by score in both directions, leaving ungraded last", () => {
    const list = [
      exercise(1, { submission_count: 1, best_score: 70 }),
      exercise(2),
      exercise(3, { submission_count: 1, best_score: 95 }),
      exercise(4, { submission_count: 1, best_score: 40 }),
    ];

    expect(
      arrangeExercises(list, "all", "score-desc", NOW).map((e) => e.id),
    ).toEqual([3, 1, 4, 2]);
    expect(
      arrangeExercises(list, "all", "score-asc", NOW).map((e) => e.id),
    ).toEqual([4, 1, 3, 2]);
  });

  test("orders by the latest submission, leaving never-submitted last", () => {
    const list = [
      exercise(1, {
        submission_count: 1,
        last_submission_at: new Date(NOW - 3 * HOUR).toISOString(),
      }),
      exercise(2),
      exercise(3, {
        submission_count: 1,
        last_submission_at: new Date(NOW - HOUR).toISOString(),
      }),
    ];

    expect(
      arrangeExercises(list, "all", "recent", NOW).map((e) => e.id),
    ).toEqual([3, 1, 2]);
  });
});

describe("summarizeExercises", () => {
  test("counts open, submitted and graded exercises and averages best scores", () => {
    const summary = summarizeExercises(
      [
        exercise(1, { submission_count: 1, best_score: 80 }),
        exercise(2, {
          submission_count: 2,
          best_score: 60,
          deadline: new Date(NOW - HOUR).toISOString(),
        }),
        exercise(3),
      ],
      NOW,
    );

    expect(summary).toEqual({
      total: 3,
      open: 2,
      submitted: 2,
      graded: 2,
      averageScore: 70,
    });
  });

  test("reports no average until an exercise is graded", () => {
    expect(summarizeExercises([exercise(1)], NOW).averageScore).toBeNull();
  });
});
