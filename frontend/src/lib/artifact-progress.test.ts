import { describe, expect, test } from "bun:test";

import { applyArtifactProgress } from "./artifact-progress";

describe("applyArtifactProgress", () => {
  test("adds a case when its upload starts", () => {
    expect(applyArtifactProgress([], { test_case_id: 3, state: "pending" })).toEqual([3]);
    expect(
      applyArtifactProgress([3], { test_case_id: 3, state: "pending" }),
    ).toEqual([3]);
  });

  test("removes a case when its upload is ready or failed", () => {
    expect(
      applyArtifactProgress([3, 4], { test_case_id: 3, state: "ready" }),
    ).toEqual([4]);
    expect(
      applyArtifactProgress([3, 4], { test_case_id: 4, state: "failed" }),
    ).toEqual([3]);
  });

  test("ignores run-level and malformed events", () => {
    const pending = [3];
    expect(applyArtifactProgress(pending, { state: "ready" })).toEqual(pending);
    expect(applyArtifactProgress(pending, { test_case_id: 3 })).toEqual(pending);
    expect(
      applyArtifactProgress(pending, { test_case_id: 3, state: "other" }),
    ).toEqual(pending);
  });
});
