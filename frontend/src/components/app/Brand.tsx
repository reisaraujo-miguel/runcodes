import { NavLink } from "react-router";

import logoDark from "@/assets/runcodes-logo/runcodes-logo-dark.svg";
import logoLight from "@/assets/runcodes-logo/runcodes-logo-light.svg";
import { cn } from "@/lib/utils";

/**
 * The RunCodes wordmark. Two source images exist — a dark-blue wordmark for light
 * surfaces and a white one for dark surfaces — so the right one is shown per
 * theme without recoloring the artwork.
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
      <img src={logoLight} alt="RunCodes" className="h-7 w-auto dark:hidden" />
      <img
        src={logoDark}
        alt="RunCodes"
        className="hidden h-7 w-auto dark:block"
      />
    </NavLink>
  );
}
