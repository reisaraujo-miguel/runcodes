import { describe, expect, test } from "bun:test";

import { initialCommitId } from "./submission-hydration";

describe("initialCommitId", () => {
  test("opens on the latest server submission when the tab stored none", () => {
    // The bug: a new tab has no sessionStorage entry, but the student has
    // submissions on the server and expects to see the newest one.
    expect(initialCommitId(null, 23, null, false)).toBe(23);
  });

  test("does not let a stale history list replace a run just submitted here", () => {
    // The list was requested before the new submission existed.
    expect(initialCommitId(24, 23, null, false)).toBe(24);
  });

  test("prefers the server's newest submission over another tab's older run", () => {
    expect(initialCommitId(null, 23, 22, false)).toBe(23);
  });

  test("ignores a stored run the server no longer lists", () => {
    expect(initialCommitId(null, null, 22, false)).toBeNull();
  });

  test("falls back to the stored run when the history cannot be loaded", () => {
    expect(initialCommitId(null, null, 22, true)).toBe(22);
  });

  test("stays empty when nothing is known", () => {
    expect(initialCommitId(null, null, null, false)).toBeNull();
    expect(initialCommitId(null, null, null, true)).toBeNull();
  });
});
