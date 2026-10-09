import { describe, expect, test } from "bun:test";

import type { AllowedFileType } from "@/lib/api";

import {
  fileTypeOptions,
  orphanTypeIds,
  selectionFromCombobox,
} from "./allowed-file-types";

function type(
  id: number,
  name: string,
  extension: string,
  isAvailable = true,
): AllowedFileType {
  return {
    id,
    name,
    extension,
    is_compilable: false,
    is_available: isAvailable,
  };
}

const c = type(1, "C", "c");
const python = type(2, "Python", "py");
const cobol = type(3, "COBOL", "cob", false);

describe("fileTypeOptions", () => {
  test("lists available types and labels them with their extension", () => {
    expect(fileTypeOptions([c, python], [])).toEqual([
      { value: 1, label: "C (c)" },
      { value: 2, label: "Python (py)" },
    ]);
  });

  test("keeps a selected type that is no longer available", () => {
    expect(fileTypeOptions([c, cobol], [3])).toEqual([
      { value: 1, label: "C (c)" },
      { value: 3, label: "COBOL (cob)" },
    ]);
  });

  test("hides an unavailable type that is not selected", () => {
    expect(fileTypeOptions([c, cobol], [])).toEqual([
      { value: 1, label: "C (c)" },
    ]);
  });
});

describe("orphanTypeIds", () => {
  test("returns selected ids the API no longer lists", () => {
    const options = fileTypeOptions([c, python], [2, 9]);
    expect(orphanTypeIds(options, [2, 9])).toEqual([9]);
  });

  test("is empty when every selected id has an option", () => {
    const options = fileTypeOptions([c, python], []);
    expect(orphanTypeIds(options, [1, 2])).toEqual([]);
  });
});

describe("selectionFromCombobox", () => {
  test("keeps orphaned ids alongside the chosen ones", () => {
    expect(selectionFromCombobox([{ value: 1, label: "C (c)" }], [9])).toEqual([
      1, 9,
    ]);
  });
});
