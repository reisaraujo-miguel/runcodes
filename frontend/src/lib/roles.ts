import type { EnrollmentRole } from "@/lib/api/enrollments";

/** The badge variants a role can render as. */
export type BadgeTone =
  | "default"
  | "secondary"
  | "outline"
  | "success"
  | "warning"
  | "info"
  | "destructive";

/** Labels for the platform account roles (`user_t`). */
const PLATFORM_ROLE_LABELS: Record<string, string> = {
  student: "Estudante",
  professor: "Professor",
  monitor: "Monitor",
  admin: "Administrador",
  dev: "Desenvolvedor",
};

const PLATFORM_ROLE_TONES: Record<string, BadgeTone> = {
  student: "secondary",
  professor: "info",
  monitor: "info",
  admin: "default",
  dev: "warning",
};

/** Human label for a platform role, falling back to the raw value. */
export function platformRoleLabel(role: string): string {
  return PLATFORM_ROLE_LABELS[role] ?? role;
}

/** Badge tone for a platform role. */
export function platformRoleTone(role: string): BadgeTone {
  return PLATFORM_ROLE_TONES[role] ?? "outline";
}

/** Labels for a role inside a class (`enrollment_role_t`). */
const ENROLLMENT_ROLE_LABELS: Record<EnrollmentRole, string> = {
  student: "Estudante",
  monitor: "Monitor",
  professor: "Professor",
};

const ENROLLMENT_ROLE_TONES: Record<EnrollmentRole, BadgeTone> = {
  student: "secondary",
  monitor: "info",
  professor: "default",
};

/** Human label for an enrollment role. */
export function enrollmentRoleLabel(role: EnrollmentRole): string {
  return ENROLLMENT_ROLE_LABELS[role];
}

/** Badge tone for an enrollment role. */
export function enrollmentRoleTone(role: EnrollmentRole): BadgeTone {
  return ENROLLMENT_ROLE_TONES[role];
}
