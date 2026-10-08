import { Badge } from "@/components/ui/badge";
import type { EnrollmentRole } from "@/lib/api/enrollments";
import {
  enrollmentRoleLabel,
  enrollmentRoleTone,
  platformRoleLabel,
  platformRoleTone,
} from "@/lib/roles";

/** A badge for a platform account role (`student` / `professor` / `admin`). */
export function PlatformRoleBadge({
  role,
  className,
}: {
  role: string;
  className?: string;
}) {
  return (
    <Badge variant={platformRoleTone(role)} className={className}>
      {platformRoleLabel(role)}
    </Badge>
  );
}

/** A badge for a role inside a class; a banned member is shown as such. */
export function EnrollmentRoleBadge({
  role,
  banned,
  className,
}: {
  role: EnrollmentRole;
  banned?: boolean;
  className?: string;
}) {
  if (banned) {
    return (
      <Badge variant="destructive" className={className}>
        Banido
      </Badge>
    );
  }
  return (
    <Badge variant={enrollmentRoleTone(role)} className={className}>
      {enrollmentRoleLabel(role)}
    </Badge>
  );
}
