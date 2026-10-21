import { describe, expect, test } from "bun:test";

import {
  dateInputToTimestamp,
  dateTimeInputToTimestamp,
  formatBytes,
  formatCpuTime,
  formatDateTime,
  formatLimit,
  formatTimeInput,
  isEndOfDay,
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

describe("formatLimit", () => {
  test("renders an unset limit as the platform default", () => {
    expect(formatLimit(0, formatBytes)).toBe("padrão");
    expect(formatLimit(0, formatCpuTime)).toBe("padrão");
  });

  test("renders a set limit with the given formatter", () => {
    expect(formatLimit(1024, formatBytes)).toBe("1.0 KB");
    expect(formatLimit(5, formatCpuTime)).toBe("5.000 s");
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

describe("formatTimeInput", () => {
  test("renders the local HH:mm of a timestamp", () => {
    const local = new Date(2026, 2, 4, 14, 30).toISOString();
    expect(formatTimeInput(local)).toBe("14:30");
  });

  test("renders missing and invalid values as empty", () => {
    expect(formatTimeInput(null)).toBe("");
    expect(formatTimeInput("not-a-date")).toBe("");
  });
});

describe("isEndOfDay", () => {
  test("recognizes 23:59 regardless of seconds", () => {
    const endOfDay = new Date(2026, 2, 4, 23, 59, 0).toISOString();
    const lastSecond = new Date(2026, 2, 4, 23, 59, 59).toISOString();
    expect(isEndOfDay(endOfDay)).toBe(true);
    expect(isEndOfDay(lastSecond)).toBe(true);
  });

  test("rejects other times and invalid values", () => {
    const earlier = new Date(2026, 2, 4, 23, 58).toISOString();
    expect(isEndOfDay(earlier)).toBe(false);
    expect(isEndOfDay(null)).toBe(false);
    expect(isEndOfDay("not-a-date")).toBe(false);
  });
});

describe("dateTimeInputToTimestamp", () => {
  test("combines a date and time in the local timezone", () => {
    const iso = dateTimeInputToTimestamp("2026-03-04", "14:30");
    if (iso === null) throw new Error("expected a timestamp");

    const date = new Date(iso);
    expect(date.getFullYear()).toBe(2026);
    expect(date.getMonth()).toBe(2);
    expect(date.getDate()).toBe(4);
    expect(date.getHours()).toBe(14);
    expect(date.getMinutes()).toBe(30);
    expect(date.getSeconds()).toBe(0);
  });

  test("uses the last second of the day for the end-of-day shortcut", () => {
    const iso = dateTimeInputToTimestamp("2026-03-04", "08:00", true);
    if (iso === null) throw new Error("expected a timestamp");

    const date = new Date(iso);
    expect(date.getHours()).toBe(23);
    expect(date.getMinutes()).toBe(59);
    expect(date.getSeconds()).toBe(59);
  });

  test("rejects invalid times and impossible dates", () => {
    expect(dateTimeInputToTimestamp("2026-03-04", "24:00")).toBeNull();
    expect(dateTimeInputToTimestamp("2026-03-04", "12:60")).toBeNull();
    expect(dateTimeInputToTimestamp("2026-02-31", "12:00")).toBeNull();
  });
});
