import { describe, expect, test } from "bun:test";

import type { TestCase } from "@/lib/api";

import {
  draftFromTestCase,
  draftToNewTestCase,
  draftToPatch,
  emptyDraft,
  limitToInput,
  parseLimit,
  testCaseFileKey,
  testCaseFileName,
  type TestCaseDraft,
} from "./test-case-draft";

/** A stored text-in/text-out case, with an unset (0 = default) CPU limit. */
const textCase: TestCase = {
  id: 1,
  exercise_id: 7,
  input: "1 2",
  input_type: "text",
  show_input: true,
  expected_output: "3",
  expected_output_type: "text",
  show_expected_output: true,
  show_user_output: true,
  cpu_time_limit_seconds: 0,
  mem_usage_limit_bytes: 0,
  stack_limit_bytes: 0,
  file_size_limit_bytes: 0,
  files: [],
};

describe("parseLimit", () => {
  test("treats an empty value as unset", () => {
    expect(parseLimit("")).toBeUndefined();
    expect(parseLimit("   ")).toBeUndefined();
  });

  test("accepts zero and positive integers", () => {
    expect(parseLimit("0")).toBe(0);
    expect(parseLimit("15")).toBe(15);
  });

  test("rejects negatives and non-numbers", () => {
    expect(parseLimit("-1")).toBeUndefined();
    expect(parseLimit("abc")).toBeUndefined();
    expect(parseLimit("NaN")).toBeUndefined();
  });
});

describe("limitToInput", () => {
  test("renders a stored limit as its input value", () => {
    expect(limitToInput(0)).toBe("0");
    expect(limitToInput(2048)).toBe("2048");
  });

  test("renders a non-finite limit as empty", () => {
    expect(limitToInput(Number.NaN)).toBe("");
  });
});

describe("draftToNewTestCase", () => {
  test("sends the default types, flags and text of a blank draft", () => {
    expect(draftToNewTestCase(emptyDraft())).toEqual({
      input_type: "text",
      expected_output_type: "text",
      show_input: false,
      show_expected_output: false,
      show_user_output: true,
      input: "",
      expected_output: "",
    });
  });

  test("includes text, flags and limits when set", () => {
    const draft: TestCaseDraft = {
      ...emptyDraft(),
      inputText: "1 2",
      outputText: "3",
      showInput: true,
      cpuTime: "2",
      memLimit: "1024",
    };

    expect(draftToNewTestCase(draft)).toEqual({
      input_type: "text",
      expected_output_type: "text",
      show_input: true,
      show_expected_output: false,
      show_user_output: true,
      input: "1 2",
      expected_output: "3",
      cpu_time_limit_seconds: 2,
      mem_usage_limit_bytes: 1024,
    });
  });

  test("sends files for file-typed sides, and extras when present", () => {
    const inputFile = new File(["in"], "in.txt");
    const extra = new File(["extra"], "helper.txt");
    const draft: TestCaseDraft = {
      ...emptyDraft(),
      inputType: "file",
      outputType: "file",
      inputFile,
      outputFile: new File(["out"], "out.txt"),
      extraFiles: [extra],
    };

    const payload = draftToNewTestCase(draft);
    expect(payload.input_file).toBe(inputFile);
    expect(payload.expected_output_file).toBeInstanceOf(File);
    expect(payload.files).toEqual([extra]);
    // A file-typed side never sends the text field.
    expect(payload.input).toBeUndefined();
    expect(payload.expected_output).toBeUndefined();
  });
});

describe("draftToPatch", () => {
  test("produces an empty patch when nothing changed", () => {
    expect(draftToPatch(draftFromTestCase(textCase), textCase)).toEqual({});
  });

  test("sends only the fields that differ", () => {
    const draft = { ...draftFromTestCase(textCase), inputText: "9 9" };
    expect(draftToPatch(draft, textCase)).toEqual({ input: "9 9" });
  });

  test("includes a changed flag and a changed limit", () => {
    const draft = {
      ...draftFromTestCase(textCase),
      showUserOutput: false,
      cpuTime: "5",
    };
    expect(draftToPatch(draft, textCase)).toEqual({
      show_user_output: false,
      cpu_time_limit_seconds: 5,
    });
  });

  test("treats a 0 limit as unchanged (it means platform default)", () => {
    const draft = { ...draftFromTestCase(textCase), cpuTime: "0" };
    expect(draftToPatch(draft, textCase)).toEqual({});
  });

  test("replaces extra files only when some were picked", () => {
    const unchanged = draftFromTestCase(textCase);
    expect(draftToPatch(unchanged, textCase).files).toBeUndefined();

    const extra = new File(["x"], "extra.txt");
    const withExtras = { ...unchanged, extraFiles: [extra] };
    expect(draftToPatch(withExtras, textCase).files).toEqual([extra]);
  });

  test("sends a new file only for a file-typed side", () => {
    const fileCase: TestCase = {
      ...textCase,
      input_type: "file",
      expected_output_type: "file",
    };
    const newFile = new File(["in"], "in.bin");
    const draft: TestCaseDraft = {
      ...draftFromTestCase(fileCase),
      inputFile: newFile,
    };

    const patch = draftToPatch(draft, fileCase);
    expect(patch.input_file).toBe(newFile);
    expect(patch.expected_output_file).toBeUndefined();
    expect(patch.input).toBeUndefined();
  });
});

describe("testCaseFileName", () => {
  test("reads the path when present", () => {
    expect(testCaseFileName({ id: 3, path: "a.txt" }, 0)).toBe("a.txt");
  });

  test("falls back to a numbered name", () => {
    expect(testCaseFileName("x", 0)).toBe("Arquivo 1");
    expect(testCaseFileName({ path: "   " }, 1)).toBe("Arquivo 2");
  });
});

describe("testCaseFileKey", () => {
  test("prefers a stable id", () => {
    expect(testCaseFileKey({ id: 5, path: "a.txt" }, 0)).toBe("5");
    expect(testCaseFileKey({ id: "abc" }, 0)).toBe("abc");
  });

  test("falls back to the name and index", () => {
    expect(testCaseFileKey(null, 1)).toBe("Arquivo 2-1");
  });
});
