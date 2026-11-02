import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

/**
 * A page section: a heading, an optional description and action, and the
 * content, with no border or background of its own.
 *
 * This is the default container for a page or a tab. Cards are reserved for
 * discrete objects that stand on their own (an exercise, a dialog, a callout);
 * wrapping a whole section in one — and never nesting one card inside another —
 * is what the design language avoids. See `frontend/README.md`.
 *
 * The heading level is configurable because a section can be the page's top
 * level (`h2`) or a sub-section inside another one (`h3`).
 */
export function Section({
  id,
  title,
  description,
  action,
  children,
  className,
  contentClassName,
  headingLevel = 2,
}: {
  id?: string;
  /**
   * The section heading. Omit it when the surrounding context already names the
   * section (a tab, for example); the description then acts as the lead.
   */
  title?: ReactNode;
  description?: ReactNode;
  action?: ReactNode;
  children: ReactNode;
  className?: string;
  contentClassName?: string;
  headingLevel?: 2 | 3;
}) {
  const Heading = headingLevel === 2 ? "h2" : "h3";
  const titleId = id && title ? `${id}-title` : undefined;
  const headingClass =
    headingLevel === 2
      ? "text-lg leading-snug font-semibold tracking-tight"
      : "text-base leading-snug font-semibold";

  return (
    <section
      id={id}
      aria-labelledby={titleId}
      className={cn("space-y-4", className)}
    >
      {title || description || action ? (
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0 space-y-1">
            {title ? (
              <Heading id={titleId} className={headingClass}>
                {title}
              </Heading>
            ) : null}
            {description ? (
              <p className="text-muted-foreground text-sm">{description}</p>
            ) : null}
          </div>
          {action ? <div className="shrink-0">{action}</div> : null}
        </div>
      ) : null}

      <div className={cn("space-y-4", contentClassName)}>{children}</div>
    </section>
  );
}
