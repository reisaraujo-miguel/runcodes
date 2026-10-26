import type { ReactNode } from "react";

import { Link, useLocation, useNavigate } from "react-router";

import { originFromState, resolveBackNavigation } from "@/lib/back-navigation";
import { cn } from "@/lib/utils";

/**
 * The number of in-app history entries before the current one.
 *
 * React Router stores its history index in `window.history.state.idx`, starting
 * at zero for the entry the app was opened on. Anything above zero means a back
 * action can pop to a previous page of this app instead of the login flow or an
 * external site.
 */
function historyIndex(): number {
  if (typeof window === "undefined") return 0;
  const state = window.history.state as { idx?: unknown } | null;
  return typeof state?.idx === "number" && Number.isFinite(state.idx)
    ? state.idx
    : 0;
}

/**
 * A back control for pages reached from more than one place.
 *
 * Whenever there is in-app history, the control pops it, so repeated presses
 * keep walking back instead of bouncing between a detail page and its origin.
 * A link that knows its origin stores it in `location.state.from`; that origin
 * labels the control and is the destination when the page was opened directly
 * (bookmark, new tab, search result), where there is no history to pop. The
 * static `to` destination is the last fallback.
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
  const target = resolveBackNavigation({
    origin: from,
    currentPath: current,
    canGoBack: historyIndex() > 0,
    fallbackPath: to,
  });

  const classes = cn(
    "text-muted-foreground hover:text-foreground focus-visible:ring-ring inline-flex items-center gap-1 rounded-sm text-sm font-medium outline-none focus-visible:ring-2",
    className,
  );
  const text = label ?? target.label;

  if (target.kind === "link") {
    return (
      <Link to={target.path} className={classes}>
        {icon ?? <span aria-hidden>←</span>}
        {text}
      </Link>
    );
  }

  return (
    <button
      type="button"
      className={classes}
      onClick={() => {
        if (target.kind === "history") {
          void navigate(-1);
          return;
        }
        // A direct visit has no entry to pop, so replace it instead of leaving
        // the detail page behind in history.
        void navigate(target.path, { replace: true });
      }}
    >
      {icon ?? <span aria-hidden>←</span>}
      {text}
    </button>
  );
}
