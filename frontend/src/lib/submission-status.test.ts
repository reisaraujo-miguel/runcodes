import { describe, expect, test } from "bun:test";

import {
  caseStatus,
  commitStatus,
  finishedStatus,
  isPendingStatus,
} from "./submission-status";

describe("commitStatus", () => {
  test("maps every commit status to a label and tone", () => {
    expect(commitStatus("queued")).toEqual({ label: "Na fila", tone: "neutral" });
    expect(commitStatus("compiling")).toEqual({ label: "Compilando", tone: "info" });
    expect(commitStatus("completed")).toEqual({ label: "Concluído", tone: "success" });
    expect(commitStatus("compilation_error").tone).toBe("danger");
    expect(commitStatus("timeout").tone).toBe("danger");
  });
});

describe("caseStatus", () => {
  test("maps case verdicts", () => {
    expect(caseStatus("correct")).toEqual({ label: "Correto", tone: "success" });
    expect(caseStatus("bad_formatted_output").tone).toBe("warning");
    expect(caseStatus("killed_with_signal").tone).toBe("danger");
  });
});

describe("finishedStatus", () => {
  test("matches the commit status mapping", () => {
    expect(finishedStatus("uncompleted")).toEqual(commitStatus("uncompleted"));
    expect(finishedStatus("server_error")).toEqual(commitStatus("server_error"));
  });
});

describe("isPendingStatus", () => {
  test("is true only while the run is in progress", () => {
    expect(isPendingStatus("queued")).toBe(true);
    expect(isPendingStatus("compiling")).toBe(true);
    expect(isPendingStatus("running")).toBe(true);
    expect(isPendingStatus("completed")).toBe(false);
    expect(isPendingStatus("server_error")).toBe(false);
  });
});
