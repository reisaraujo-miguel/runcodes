import { z } from "zod";

/**
 * The backend's password policy, mirrored in the forms so a mistake is reported
 * before a request is made. Keep it in step with validation.ValidatePassword.
 */
const SPECIAL_CHARACTER = /[!@#$%^&*()_+\-=[\]{};':"\\|,.<>/?~`]/;

export const passwordSchema = z
  .string()
  .min(8, "A senha deve ter no mínimo 8 caracteres")
  .regex(/[A-Z]/, "Inclua ao menos uma letra maiúscula")
  .regex(/[a-z]/, "Inclua ao menos uma letra minúscula")
  .regex(/[0-9]/, "Inclua ao menos um dígito")
  .regex(SPECIAL_CHARACTER, "Inclua ao menos um caractere especial");
