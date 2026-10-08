import { describe, expect, test } from "bun:test";

import {
  enrollmentRoleLabel,
  enrollmentRoleTone,
  platformRoleLabel,
  platformRoleTone,
} from "./roles";

describe("platformRoleLabel", () => {
  test("translates every platform role", () => {
    expect(platformRoleLabel("student")).toBe("Estudante");
    expect(platformRoleLabel("professor")).toBe("Professor");
    expect(platformRoleLabel("admin")).toBe("Administrador");
    expect(platformRoleLabel("dev")).toBe("Desenvolvedor");
  });

  test("passes through an unrecognised role", () => {
    expect(platformRoleLabel("wizard")).toBe("wizard");
  });
});

describe("platformRoleTone", () => {
  test("maps roles to badge tones", () => {
    expect(platformRoleTone("student")).toBe("secondary");
    expect(platformRoleTone("professor")).toBe("info");
    expect(platformRoleTone("admin")).toBe("default");
    expect(platformRoleTone("dev")).toBe("warning");
  });

  test("falls back to the outline tone", () => {
    expect(platformRoleTone("wizard")).toBe("outline");
  });
});

describe("enrollmentRoleLabel", () => {
  test("translates each role inside a class", () => {
    expect(enrollmentRoleLabel("student")).toBe("Estudante");
    expect(enrollmentRoleLabel("monitor")).toBe("Monitor");
    expect(enrollmentRoleLabel("professor")).toBe("Professor");
  });
});

describe("enrollmentRoleTone", () => {
  test("maps roles inside a class to badge tones", () => {
    expect(enrollmentRoleTone("student")).toBe("secondary");
    expect(enrollmentRoleTone("monitor")).toBe("info");
    expect(enrollmentRoleTone("professor")).toBe("default");
  });
});
