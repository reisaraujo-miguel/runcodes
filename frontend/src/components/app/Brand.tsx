import { NavLink } from "react-router";

import LogoDark from "@/assets/runcodes-logo/runcodes-logo-dark.svg?react";
import LogoLight from "@/assets/runcodes-logo/runcodes-logo-light.svg?react";
import { cn } from "@/lib/utils";

/**
 * The RunCodes wordmark. Two source images exist — a dark-blue wordmark for light
 * surfaces and a white one for dark surfaces — so the right one is shown per
 * theme without recoloring the artwork.
 *
 * The wordmarks are imported as React components (the `?react` query) so Vite
 * runs them through SVGR/SVGO at build time; importing them as plain URLs would
 * bypass the optimisation entirely.
 */
export function Brand({ className }: { className?: string }) {
  return (
    <NavLink
      to="/"
      className={cn(
        "flex items-center gap-2 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
        className,
      )}
      aria-label="RunCodes — início"
    >
      <LogoLight
        role="img"
        aria-label="RunCodes"
        className="h-8 w-auto dark:hidden"
      />
      <LogoDark
        role="img"
        aria-label="RunCodes"
        className="hidden h-8 w-auto dark:block"
      />
    </NavLink>
  );
}
