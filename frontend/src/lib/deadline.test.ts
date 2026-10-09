import { describe, expect, test } from "bun:test";

import { deadlineInfo } from "./deadline";

const HOUR = 60 * 60 * 1000;
const DAY = 24 * HOUR;
const NOW = new Date("2026-06-01T12:00:00.000Z").getTime();

function at(offsetMs: number): string {
  return new Date(NOW + offsetMs).toISOString();
}

describe("deadlineInfo", () => {
  test("marks a past deadline as encerrado", () => {
    expect(deadlineInfo(at(-HOUR), NOW)).toEqual({
      label: "Encerrado",
      tone: "destructive",
      expired: true,
    });
    // The boundary itself is expired, not in the future.
    expect(deadlineInfo(at(0), NOW).expired).toBe(true);
  });

  test("counts down the remaining hours within the last day", () => {
    expect(deadlineInfo(at(2 * HOUR), NOW)).toEqual({
      label: "Encerra em 2h",
      tone: "destructive",
      expired: false,
    });
    // Under an hour still rounds up to one, never zero.
    expect(deadlineInfo(at(10 * 60 * 1000), NOW).label).toBe("Encerra em 1h");
  });

  test("warns while the deadline is within three days", () => {
    expect(deadlineInfo(at(DAY), NOW)).toEqual({
      label: "Encerra em 1 dia",
      tone: "warning",
      expired: false,
    });
    expect(deadlineInfo(at(2 * DAY), NOW)).toEqual({
      label: "Encerra em 2 dias",
      tone: "warning",
      expired: false,
    });
    expect(deadlineInfo(at(3 * DAY), NOW).tone).toBe("warning");
  });

  test("is neutral once the deadline is further out", () => {
    const info = deadlineInfo(at(10 * DAY), NOW);
    expect(info.tone).toBe("neutral");
    expect(info.expired).toBe(false);
    expect(info.label).not.toContain("Encerra");
  });

  test("keeps an unparseable value instead of inventing a countdown", () => {
    expect(deadlineInfo("not-a-date", NOW)).toEqual({
      label: "not-a-date",
      tone: "neutral",
      expired: false,
    });
  });
});
