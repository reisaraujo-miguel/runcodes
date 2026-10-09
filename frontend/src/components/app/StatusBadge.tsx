import { Badge } from "@/components/ui/badge";
import type { BadgeTone } from "@/lib/roles";
import type { StatusTone, StatusToneValue } from "@/lib/submission-status";

const TONE_TO_BADGE: Record<StatusTone, BadgeTone> = {
  neutral: "secondary",
  info: "info",
  success: "success",
  warning: "warning",
  danger: "destructive",
};

/** Renders a `{ label, tone }` status value as a badge. */
export function StatusBadge({
  value,
  className,
}: {
  value: StatusToneValue;
  className?: string;
}) {
  return (
    <Badge variant={TONE_TO_BADGE[value.tone]} className={className}>
      {value.label}
    </Badge>
  );
}
