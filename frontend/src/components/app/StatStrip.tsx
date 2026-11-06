import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { fieldLabelClass } from "@/lib/label";
import { cn } from "@/lib/utils";

const TONE_CLASSES = {
  primary: "text-primary",
  success: "text-success",
  warning: "text-warning",
  info: "text-info",
  neutral: "text-muted-foreground",
} as const;

/** One metric in a {@link StatStrip}. */
export interface StatItem {
  icon: LucideIcon;
  label: string;
  value: ReactNode;
  hint?: string;
  tone?: keyof typeof TONE_CLASSES;
}

/**
 * A horizontal band of metrics.
 *
 * The band is one surface — a rule above it and plain columns — instead of a
 * card per number. Metrics belong together and read better as a strip; a border
 * around each of them only adds frames. Tones colour the icon, not a background.
 */
export function StatStrip({
  items,
  className,
}: {
  items: readonly StatItem[];
  className?: string;
}) {
  return (
    <dl
      className={cn(
        "grid gap-6 border-t pt-5 sm:grid-cols-2 xl:grid-cols-4",
        className,
      )}
    >
      {items.map(({ icon: Icon, label, value, hint, tone = "neutral" }) => (
        <div key={label} className="flex items-start gap-3">
          <Icon
            className={cn("mt-0.5 size-4 shrink-0", TONE_CLASSES[tone])}
            aria-hidden
          />
          <div className="min-w-0">
            <dt className={fieldLabelClass}>{label}</dt>
            <dd className="text-xl leading-tight font-semibold tabular-nums">
              {value}
            </dd>
            {hint ? (
              <p className="text-xs text-muted-foreground">{hint}</p>
            ) : null}
          </div>
        </div>
      ))}
    </dl>
  );
}
