import type { ReactNode } from "react";

import { fieldLabelClass } from "@/lib/label";
import { cn } from "@/lib/utils";

/**
 * One labelled value inside a `<dl>`.
 *
 * The label is a micro-label (small, uppercase, muted) and the value is regular
 * body text, matching the stat strip and the table headers. Keeping labels in
 * that register is what stops a field list from reading like a stack of
 * headings — the section title stays the only strong element on the page.
 */
export function Field({
  label,
  children,
  className,
}: {
  label: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("min-w-0", className)}>
      <dt className={fieldLabelClass}>{label}</dt>
      <dd className="mt-1.5 text-sm">{children}</dd>
    </div>
  );
}

/**
 * A label and its value on one line, for a compact list such as a sidebar:
 * the label keeps the micro-label voice and the value sits at the right edge.
 */
export function FieldRow({
  label,
  children,
  className,
}: {
  label: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex items-start justify-between gap-3", className)}>
      <dt className={cn(fieldLabelClass, "shrink-0 pt-0.5")}>{label}</dt>
      <dd className="min-w-0 text-right">{children}</dd>
    </div>
  );
}
