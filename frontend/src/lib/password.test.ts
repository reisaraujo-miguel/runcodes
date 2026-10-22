import { describe, expect, test } from "bun:test";

import { passwordSchema } from "./password";

describe("passwordSchema", () => {
  test("accepts a password that meets the backend policy", () => {
    expect(passwordSchema.safeParse("S3cure!pass").success).toBe(true);
  });

  test("reports every missing requirement", () => {
    const messages = [
      "short",
      "lowercase1!",
      "UPPERCASE1!",
      "NoDigits!!",
      "NoSpecial1",
    ].map((value) => {
      const result = passwordSchema.safeParse(value);
      return result.success ? null : result.error.issues[0]?.message;
    });

    expect(messages).toEqual([
      "A senha deve ter no mínimo 8 caracteres",
      "Inclua ao menos uma letra maiúscula",
      "Inclua ao menos uma letra minúscula",
      "Inclua ao menos um dígito",
      "Inclua ao menos um caractere especial",
    ]);
  });
});
