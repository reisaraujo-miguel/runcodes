import { describe, expect, test } from "bun:test";

import {
  dateInputToTimestamp,
  formatBytes,
  formatCpuTime,
  formatDateTime,
} from "./format";

describe("formatBytes", () => {
  test("renders bytes below 1 KiB verbatim", () => {
    expect(formatBytes(0)).toBe("0 B");
    expect(formatBytes(1023)).toBe("1023 B");
  });

  test("scales to larger units", () => {
    expect(formatBytes(1024)).toBe("1.0 KB");
    expect(formatBytes(1536)).toBe("1.5 KB");
    expect(formatBytes(1024 * 1024)).toBe("1.0 MB");
  });

  test("renders unavailable values as a dash", () => {
    expect(formatBytes(-1)).toBe("—");
    expect(formatBytes(Number.NaN)).toBe("—");
    expect(formatBytes(Number.POSITIVE_INFINITY)).toBe("—");
  });
});

describe("formatCpuTime", () => {
  test("formats seconds with millisecond precision", () => {
    expect(formatCpuTime(0)).toBe("0.000 s");
    expect(formatCpuTime(0.0019)).toBe("0.002 s");
  });

  test("renders unavailable values as a dash", () => {
    expect(formatCpuTime(-1)).toBe("—");
    expect(formatCpuTime(Number.NaN)).toBe("—");
  });
});

describe("formatDateTime", () => {
  test("renders missing values as a dash", () => {
    expect(formatDateTime("")).toBe("—");
    expect(formatDateTime(null)).toBe("—");
    expect(formatDateTime(undefined)).toBe("—");
  });

  test("falls back to the raw value when unparseable", () => {
    expect(formatDateTime("not-a-date")).toBe("not-a-date");
  });

  test("renders a valid timestamp", () => {
    const formatted = formatDateTime("2026-01-02T03:04:05Z");
    expect(formatted).not.toBe("—");
    expect(formatted).not.toBe("2026-01-02T03:04:05Z");
  });
});

describe("dateInputToTimestamp", () => {
  test("maps to the start and end of the local day", () => {
    const start = dateInputToTimestamp("2026-03-04", "start");
    const end = dateInputToTimestamp("2026-03-04", "end");
    if (start === null || end === null) {
      throw new Error("expected both timestamps to be defined");
    }

    const startDate = new Date(start);
    const endDate = new Date(end);
    expect(startDate.getHours()).toBe(0);
    expect(startDate.getMinutes()).toBe(0);
    expect(endDate.getHours()).toBe(23);
    expect(endDate.getMinutes()).toBe(59);
    expect(endDate.getDate()).toBe(startDate.getDate());
  });

  test("returns null for malformed input", () => {
    expect(dateInputToTimestamp("", "start")).toBeNull();
    expect(dateInputToTimestamp("2026-03", "start")).toBeNull();
  });
});
