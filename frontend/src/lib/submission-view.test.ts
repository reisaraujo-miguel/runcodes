import { describe, expect, test } from "bun:test";

import {
  upsertCaseResult,
  viewFromEvent,
  viewFromSnapshot,
  type CaseResultView,
} from "./submission-view";

const snapshotResult = {
  exercise_test_case_id: 7,
  cpu_time: 0.12,
  mem_usage: 2048,
  user_output: "hello\n",
  user_output_type: "text",
  status: "correct",
  status_message: "",
  error_message: "",
} as const;

describe("viewFromSnapshot", () => {
  test("maps snapshot columns to the view", () => {
    const view = viewFromSnapshot(snapshotResult);
    expect(view).toEqual({
      testCaseId: 7,
      cpuTime: 0.12,
      memUsage: 2048,
      status: "correct",
      statusMessage: "",
      errorMessage: "",
      userOutput: "hello\n",
      userOutputType: "text",
    });
  });
});

describe("viewFromEvent", () => {
  test("maps event fields to the view", () => {
    const view = viewFromEvent({
      type: "case_result",
      commit_id: 1,
      seq: 3,
      test_case_id: 7,
      cpu_time: 0.12,
      mem_usage: 2048,
      status: "bad_formatted_output",
      status_message: "wrong spacing",
      user_output: "hello",
      user_output_type: "text",
      error_message: "",
    });
    expect(view.status).toBe("bad_formatted_output");
    expect(view.testCaseId).toBe(7);
    expect(view.statusMessage).toBe("wrong spacing");
  });
});

describe("upsertCaseResult", () => {
  const make = (testCaseId: number, status: CaseResultView["status"]) =>
    ({
      testCaseId,
      cpuTime: 0,
      memUsage: -1,
      status,
      statusMessage: "",
      errorMessage: "",
      userOutput: "",
      userOutputType: "text",
    }) satisfies CaseResultView;

  test("appends a new case preserving order", () => {
    const first = make(1, "correct");
    const second = make(2, "correct");
    expect(upsertCaseResult([first], second)).toEqual([first, second]);
  });

  test("replaces an existing case in place", () => {
    const first = make(1, "killed_with_signal");
    const updated = make(1, "correct");
    const result = upsertCaseResult([first, make(2, "correct")], updated);
    expect(result).toEqual([updated, make(2, "correct")]);
    expect(result).toHaveLength(2);
  });

  test("does not mutate the input slice", () => {
    const first = make(1, "correct");
    const input = [first];
    upsertCaseResult(input, make(1, "bad_formatted_output"));
    expect(input[0]).toEqual(first);
  });
});
