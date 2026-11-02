import { describe, expect, test } from "bun:test";

import { alertDetailLine } from "./alerts";

describe("alertDetailLine", () => {
  test("renders the keys the judge sets today", () => {
    expect(
      alertDetailLine({
        language: "Python",
        image: "ghcr.io/runcodes-icmc/runcodes-runner-python:latest",
        milestone: "run.start",
        wait_seconds: 30,
        filename: "main.py",
      }),
    ).toBe(
      "language=Python · image=ghcr.io/runcodes-icmc/runcodes-runner-python:latest" +
        " · milestone=run.start · wait_seconds=30 · filename=main.py",
    );
  });

  test("keeps a known key only when it has a value", () => {
    expect(alertDetailLine({ language: "Python", image: null })).toBe(
      "language=Python",
    );
  });

  test("falls back to the raw JSON for an unknown shape", () => {
    expect(alertDetailLine({ queue_depth: 12 })).toBe('{"queue_depth":12}');
  });

  test("renders nothing without details", () => {
    expect(alertDetailLine(undefined)).toBe("");
    expect(alertDetailLine(null)).toBe("");
  });
});
