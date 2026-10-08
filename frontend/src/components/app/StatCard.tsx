import type { LucideIcon } from "lucide-react";

import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

const TONES = {
  primary: "bg-primary/10 text-primary",
  success: "bg-success/15 text-success",
  warning: "bg-warning/20 text-warning",
  info: "bg-info/15 text-info",
  neutral: "bg-muted text-muted-foreground",
} as const;

/** A single metric tile used across the dashboards. */
export function StatCard({
  icon: Icon,
  label,
  value,
  hint,
  tone = "primary",
}: {
  icon: LucideIcon;
  label: string;
  value: string | number;
  hint?: string;
  tone?: keyof typeof TONES;
}) {
  return (
    <Card>
      <CardContent className="flex items-center gap-4">
        <div
          className={cn(
            "flex size-11 shrink-0 items-center justify-center rounded-lg",
            TONES[tone],
          )}
        >
          <Icon className="size-5" aria-hidden />
        </div>
        <div className="min-w-0">
          <p className="text-muted-foreground text-sm">{label}</p>
          <p className="text-2xl font-semibold tabular-nums">{value}</p>
          {hint ? (
            <p className="text-muted-foreground truncate text-xs">{hint}</p>
          ) : null}
        </div>
      </CardContent>
    </Card>
  );
}
