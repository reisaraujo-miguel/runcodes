import { cn } from "@/lib/utils";
import { NavLink } from "react-router";

import GitHubLogo from "@/assets/svg-icons/github.svg?react";

/**
 * A link to the RunCodes GitHub repository.
 */
export function GitHubLink({ className }: { className?: string }) {
  return (
    <NavLink
      to="https://github.com/reisaraujo-miguel/runcodes"
      target="_blank"
      className={cn(
        "flex items-center  rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
        className,
      )}
      aria-label="RunCodes — GitHub repository"
    >
      <GitHubLogo className="size-[1em]"></GitHubLogo>
    </NavLink>
  );
}
