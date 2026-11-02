import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

/**
 * A list of rows separated by rules, with no surrounding card.
 *
 * It is the container for a set of equally weighted actions or items that read
 * as one list — the class tools, for example. Each row keeps its title and
 * description on the left and its control on the right, which is what a card
 * per item would space out without adding information.
 */
export function List({
  children,
  className,
  "aria-label": ariaLabel,
}: {
  children: ReactNode;
  className?: string;
  "aria-label"?: string;
}) {
  return (
    <div
      role="list"
      aria-label={ariaLabel}
      className={cn("divide-border divide-y border-t", className)}
    >
      {children}
    </div>
  );
}

/** One row of a {@link List}. */
export function ListRow({
  icon: Icon,
  title,
  description,
  children,
  className,
}: {
  icon?: LucideIcon;
  title: ReactNode;
  description?: ReactNode;
  children?: ReactNode;
  className?: string;
}) {
  return (
    <div
      role="listitem"
      className={cn(
        "flex flex-col gap-3 py-4 sm:flex-row sm:items-center sm:justify-between",
        className,
      )}
    >
      <div className="flex min-w-0 items-start gap-3">
        {Icon ? (
          <Icon
            className="text-muted-foreground mt-0.5 size-4 shrink-0"
            aria-hidden
          />
        ) : null}
        <div className="min-w-0 space-y-0.5">
          <p className="text-sm font-medium">{title}</p>
          {description ? (
            <p className="text-muted-foreground text-sm">{description}</p>
          ) : null}
        </div>
      </div>
      {children ? (
        <div className="flex shrink-0 flex-wrap items-center gap-2 sm:pl-4">
          {children}
        </div>
      ) : null}
    </div>
  );
}
