import type { ReactNode } from "react";

import { Link, useLocation, useNavigate } from "react-router";

import { backLabelForPath, originFromState } from "@/lib/back-navigation";
import { cn } from "@/lib/utils";

/**
 * A back control for pages reached from more than one place.
 *
 * A link that knows its origin stores it in `location.state.from`; the control
 * then returns there exactly. Links that do not carry state fall back to the
 * browser history when there is an in-app entry to go back to, and finally to
 * the static `to` destination for a direct visit (bookmark, new tab, search
 * result), where there is no previous page to remember.
 *
 * The visible label follows the remembered origin, so a class opened from the
 * dashboard says "Início" while the same class opened from "Minhas turmas" says
 * "Minhas turmas".
 */
export function BackLink({
  to,
  label,
  icon,
  className,
}: {
  /** Destination when there is no in-app origin (direct visit). */
  to: string;
  /** Override the label derived from the origin. */
  label?: string;
  /** Optional leading icon; defaults to the arrow character. */
  icon?: ReactNode;
  className?: string;
}) {
  const navigate = useNavigate();
  const location = useLocation();
  const from = originFromState(location.state);
  const current = `${location.pathname}${location.search}`;
  const remembered = from && from !== current ? from : null;
  const canUseHistory =
    !remembered &&
    typeof window !== "undefined" &&
    ((window.history.state as { idx?: number } | null)?.idx ?? 0) > 0;

  const classes = cn(
    "text-muted-foreground hover:text-foreground focus-visible:ring-ring inline-flex items-center gap-1 rounded-sm text-sm font-medium outline-none focus-visible:ring-2",
    className,
  );

  if (!remembered && !canUseHistory) {
    return (
      <Link to={to} className={classes}>
        {icon ?? <span aria-hidden>←</span>}
        {label ?? backLabelForPath(to)}
      </Link>
    );
  }

  const text = label ?? (remembered ? backLabelForPath(remembered) : "Voltar");

  return (
    <button
      type="button"
      className={classes}
      onClick={() => {
        if (remembered) {
          void navigate(remembered);
          return;
        }
        void navigate(-1);
      }}
    >
      {icon ?? <span aria-hidden>←</span>}
      {text}
    </button>
  );
}
