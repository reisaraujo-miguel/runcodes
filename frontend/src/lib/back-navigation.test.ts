import { describe, expect, test } from "bun:test";

import { backLabelForPath, originFromState } from "./back-navigation";

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
