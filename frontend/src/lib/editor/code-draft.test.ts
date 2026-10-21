import { describe, expect, test } from "bun:test";

import {
  MAX_CODE_DRAFT_LENGTH,
  codeDraftStorageKey,
  normalizeCodeDraft,
  readCodeDraft,
  removeCodeDraft,
  writeCodeDraft,
  type DraftStorage,
} from "./code-draft";

class MemoryStorage implements DraftStorage {
  private readonly values = new Map<string, string>();

  getItem(key: string): string | null {
    return this.values.get(key) ?? null;
  }

  setItem(key: string, value: string): void {
    this.values.set(key, value);
  }

  removeItem(key: string): void {
    this.values.delete(key);
  }

  raw(key: string): string | undefined {
    return this.values.get(key);
  }
}

class FailingStorage implements DraftStorage {
  getItem(): string | null {
    throw new Error("storage unavailable");
  }

  setItem(): void {
    throw new Error("storage unavailable");
  }

  removeItem(): void {
    throw new Error("storage unavailable");
  }
}

describe("codeDraftStorageKey", () => {
  test("scopes the key by user and exercise", () => {
    expect(codeDraftStorageKey(7, 42)).toBe("runcodes:code-draft:v1:7:42");
    expect(codeDraftStorageKey(7, 42)).not.toBe(codeDraftStorageKey(8, 42));
    expect(codeDraftStorageKey(7, 42)).not.toBe(codeDraftStorageKey(7, 43));
  });
});

describe("normalizeCodeDraft", () => {
  test("keeps a complete draft", () => {
    expect(
      normalizeCodeDraft({
        version: 1,
        code: "print('oi')\n",
        languageId: "3",
        filename: "main.py",
        updatedAt: 1_700_000_000_000,
      }),
    ).toEqual({
      code: "print('oi')\n",
      languageId: "3",
      filename: "main.py",
      updatedAt: 1_700_000_000_000,
    });
  });

  test("defaults missing optional fields and an invalid timestamp", () => {
    expect(
      normalizeCodeDraft({ version: 1, code: "x", updatedAt: "yesterday" }),
    ).toEqual({ code: "x", languageId: null, filename: null, updatedAt: 0 });

    expect(
      normalizeCodeDraft({ version: 1, code: "x", languageId: "" }),
    ).toEqual({ code: "x", languageId: null, filename: null, updatedAt: 0 });
  });

  test("rejects corrupt payloads and unknown versions", () => {
    expect(normalizeCodeDraft(null)).toBeNull();
    expect(normalizeCodeDraft("not a draft")).toBeNull();
    expect(normalizeCodeDraft({ version: 1 })).toBeNull();
    expect(normalizeCodeDraft({ version: 1, code: 42 })).toBeNull();
    expect(normalizeCodeDraft({ version: 2, code: "x" })).toBeNull();
    expect(
      normalizeCodeDraft({
        version: 1,
        code: "x".repeat(MAX_CODE_DRAFT_LENGTH + 1),
      }),
    ).toBeNull();
  });
});

describe("readCodeDraft / writeCodeDraft", () => {
  test("round-trips a draft", () => {
    const storage = new MemoryStorage();
    const code = 'fn main() {\n    println!("oi");\n}\n';

    expect(
      writeCodeDraft(
        storage,
        12,
        5,
        { code, languageId: "9", filename: "main.rs" },
        1_700_000_000_000,
      ),
    ).toBe("saved");

    expect(readCodeDraft(storage, 12, 5)).toEqual({
      code,
      languageId: "9",
      filename: "main.rs",
      updatedAt: 1_700_000_000_000,
    });
  });

  test("does not leak a draft to another user or exercise", () => {
    const storage = new MemoryStorage();
    writeCodeDraft(storage, 1, 10, {
      code: "student one",
      languageId: null,
      filename: null,
    });

    expect(readCodeDraft(storage, 1, 10)?.code).toBe("student one");
    expect(readCodeDraft(storage, 2, 10)).toBeNull();
    expect(readCodeDraft(storage, 1, 11)).toBeNull();
  });

  test("ignores missing, corrupt, and foreign payloads", () => {
    const storage = new MemoryStorage();
    const key = codeDraftStorageKey(1, 1);

    expect(readCodeDraft(storage, 1, 1)).toBeNull();

    storage.setItem(key, "{not json");
    expect(readCodeDraft(storage, 1, 1)).toBeNull();

    storage.setItem(key, JSON.stringify({ version: 99, code: "x" }));
    expect(readCodeDraft(storage, 1, 1)).toBeNull();
  });

  test("returns null for invalid ids without touching storage", () => {
    const storage = new MemoryStorage();
    writeCodeDraft(storage, 1, 1, {
      code: "x",
      languageId: null,
      filename: null,
    });

    expect(readCodeDraft(storage, null, 1)).toBeNull();
    expect(readCodeDraft(storage, 1, null)).toBeNull();
    expect(readCodeDraft(storage, 0, 1)).toBeNull();
    expect(readCodeDraft(storage, 1, -1)).toBeNull();
    expect(
      writeCodeDraft(storage, null, 1, {
        code: "x",
        languageId: null,
        filename: null,
      }),
    ).toBe("unavailable");
  });

  test("reports drafts over the storage budget without writing them", () => {
    const storage = new MemoryStorage();

    expect(
      writeCodeDraft(storage, 1, 1, {
        code: "x".repeat(MAX_CODE_DRAFT_LENGTH + 1),
        languageId: null,
        filename: null,
      }),
    ).toBe("too-large");
    expect(readCodeDraft(storage, 1, 1)).toBeNull();
  });

  test("survives storage that throws", () => {
    const storage = new FailingStorage();

    expect(readCodeDraft(storage, 1, 1)).toBeNull();
    expect(
      writeCodeDraft(storage, 1, 1, {
        code: "x",
        languageId: null,
        filename: null,
      }),
    ).toBe("unavailable");
    expect(() => {
      removeCodeDraft(storage, 1, 1);
    }).not.toThrow();
  });

  test("removeCodeDraft deletes only the requested draft", () => {
    const storage = new MemoryStorage();
    writeCodeDraft(storage, 1, 1, {
      code: "first",
      languageId: null,
      filename: null,
    });
    writeCodeDraft(storage, 1, 2, {
      code: "second",
      languageId: null,
      filename: null,
    });

    removeCodeDraft(storage, 1, 1);

    expect(readCodeDraft(storage, 1, 1)).toBeNull();
    expect(readCodeDraft(storage, 1, 2)?.code).toBe("second");
    expect(storage.raw(codeDraftStorageKey(1, 1))).toBeUndefined();
  });
});
