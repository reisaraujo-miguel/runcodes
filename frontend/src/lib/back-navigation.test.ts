import { describe, expect, test } from "bun:test";

import {
  backLabelForPath,
  originFromState,
  resolveBackNavigation,
} from "./back-navigation";

describe("originFromState", () => {
  test("accepts an in-app path", () => {
    expect(originFromState({ from: "/classes/12?tab=exercises" })).toBe(
      "/classes/12?tab=exercises",
    );
  });

  test("rejects external and relative origins", () => {
    expect(originFromState(null)).toBeNull();
    expect(originFromState("state")).toBeNull();
    expect(originFromState({})).toBeNull();
    expect(originFromState({ from: 7 })).toBeNull();
    expect(originFromState({ from: "https://example.com" })).toBeNull();
    expect(originFromState({ from: "//example.com" })).toBeNull();
    expect(originFromState({ from: "javascript:alert(1)" })).toBeNull();
    expect(originFromState({ from: "classes" })).toBeNull();
  });
});

describe("backLabelForPath", () => {
  test("uses the page the user came from", () => {
    expect(backLabelForPath("/")).toBe("Início");
    expect(backLabelForPath("/classes")).toBe("Minhas turmas");
    expect(backLabelForPath("/classes/12?tab=exercises")).toBe("Turma");
    expect(backLabelForPath("/exercises")).toBe("Exercícios");
    expect(backLabelForPath("/exercises/9/submit")).toBe("Exercício");
    expect(backLabelForPath("/professor")).toBe("Gerenciar turmas");
    expect(backLabelForPath("/professor/class/4")).toBe("Turma");
    expect(backLabelForPath("/admin/file-types")).toBe("Tipos de arquivo");
  });

  test("falls back for an unknown path", () => {
    expect(backLabelForPath("/anything")).toBe("Voltar");
  });
});

describe("resolveBackNavigation", () => {
  test("pops history even when an origin is remembered", () => {
    // Regression: following the remembered origin with a push duplicated the
    // origin in history, so the next back returned to the detail page and the
    // two pages bounced forever.
    expect(
      resolveBackNavigation({
        origin: "/professor/class/4?tab=exercises",
        currentPath: "/professor/exercise/9",
        canGoBack: true,
        fallbackPath: "/professor",
      }),
    ).toEqual({ kind: "history", label: "Turma" });
  });

  test("labels history with the remembered origin", () => {
    expect(
      resolveBackNavigation({
        origin: "/classes",
        currentPath: "/classes/12",
        canGoBack: true,
        fallbackPath: "/classes",
      }),
    ).toEqual({ kind: "history", label: "Minhas turmas" });
  });

  test("uses a generic label when only history is available", () => {
    expect(
      resolveBackNavigation({
        origin: null,
        currentPath: "/professor/class/4",
        canGoBack: true,
        fallbackPath: "/professor",
      }),
    ).toEqual({ kind: "history", label: "Voltar" });
  });

  test("returns to the remembered origin on a direct visit", () => {
    expect(
      resolveBackNavigation({
        origin: "/professor/class/4",
        currentPath: "/professor/exercise/9",
        canGoBack: false,
        fallbackPath: "/professor",
      }),
    ).toEqual({
      kind: "navigate",
      path: "/professor/class/4",
      label: "Turma",
    });
  });

  test("links to the natural parent without history or an origin", () => {
    expect(
      resolveBackNavigation({
        origin: null,
        currentPath: "/professor/exercise/9",
        canGoBack: false,
        fallbackPath: "/professor",
      }),
    ).toEqual({ kind: "link", path: "/professor", label: "Gerenciar turmas" });
  });

  test("ignores an origin that points at the current page", () => {
    expect(
      resolveBackNavigation({
        origin: "/professor/class/4",
        currentPath: "/professor/class/4",
        canGoBack: false,
        fallbackPath: "/professor",
      }),
    ).toEqual({ kind: "link", path: "/professor", label: "Gerenciar turmas" });
  });
});
