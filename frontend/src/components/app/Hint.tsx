import type { ReactNode } from "react";

import { ChevronDownIcon, InfoIcon } from "lucide-react";

import { cn } from "@/lib/utils";

/**
 * A collapsible explanation, closed by default.
 *
 * Long methodological notes belong behind a disclosure, not in a banner above
 * the content: they are read once and then only take space. The summary is a
 * quiet line with an info icon, so the hint is still discoverable next to what
 * it explains.
 */
export function Hint({
  summary = "Como ler estes números",
  children,
  className,
}: {
  summary?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <details className={cn("group text-sm", className)}>
      <summary className="text-muted-foreground hover:text-foreground flex w-fit cursor-pointer list-none items-center gap-1.5 select-none [&::-webkit-details-marker]:hidden">
        <InfoIcon className="size-4 shrink-0" aria-hidden />
        <span>{summary}</span>
        <ChevronDownIcon
          className="size-3.5 shrink-0 transition-transform group-open:rotate-180"
          aria-hidden
        />
      </summary>
      <div className="text-muted-foreground mt-2 max-w-3xl space-y-2 leading-relaxed">
        {children}
      </div>
    </details>
  );
}
