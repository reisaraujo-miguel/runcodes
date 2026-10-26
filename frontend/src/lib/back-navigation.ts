/**
 * Navigation history helpers for the back controls.
 *
 * A page can record where the user came from in `location.state.from`; these
 * helpers validate that value and turn an in-app path into a short label. They
 * are pure so the navigation rules can be tested without a browser history.
 */

/**
 * How a back control should leave the current page.
 *
 * `history` pops the previous in-app entry, `navigate` replaces the current
 * entry with a remembered path (there is no history to pop), and `link` renders
 * the page's natural parent as an ordinary link for a direct visit.
 */
export type BackNavigation =
  | { kind: "history"; label: string }
  | { kind: "navigate"; path: string; label: string }
  | { kind: "link"; path: string; label: string };

/**
 * Reads the origin a page stored in `location.state.from`. Only an in-app
 * absolute path is accepted, so a crafted state cannot send the browser to
 * another origin (or a `javascript:` URL).
 */
export function originFromState(state: unknown): string | null {
  if (typeof state !== "object" || state === null) return null;
  const from = (state as { from?: unknown }).from;
  if (typeof from !== "string") return null;
  if (!from.startsWith("/") || from.startsWith("//")) return null;
  return from;
}

/**
 * Picks the back target for a page.
 *
 * History wins whenever there is an in-app entry to pop. That entry is the page
 * the user actually came from, and popping it keeps the history stack
 * consistent. Returning to the remembered origin with a push instead would
 * leave a duplicate of the current page behind, so a second back would bounce
 * between the detail page and its origin. The remembered origin remains the
 * fallback for a direct visit (new tab, bookmark), where there is no entry to
 * pop, and the static parent is the fallback when even that is unknown.
 */
export function resolveBackNavigation({
  origin,
  currentPath,
  canGoBack,
  fallbackPath,
}: {
  /** Raw `location.state.from`, already validated by `originFromState`. */
  origin: string | null;
  /** The page the control is rendered on, including its query string. */
  currentPath: string;
  /** True when the browser has an in-app history entry to pop. */
  canGoBack: boolean;
  /** The page's natural parent, used for a direct visit. */
  fallbackPath: string;
}): BackNavigation {
  const remembered = origin !== null && origin !== currentPath ? origin : null;

  if (canGoBack) {
    return {
      kind: "history",
      label: remembered ? backLabelForPath(remembered) : "Voltar",
    };
  }

  if (remembered) {
    return {
      kind: "navigate",
      path: remembered,
      label: backLabelForPath(remembered),
    };
  }

  return {
    kind: "link",
    path: fallbackPath,
    label: backLabelForPath(fallbackPath),
  };
}

/** A short, human label for the page represented by an in-app path. */
export function backLabelForPath(path: string): string {
  const pathname = path.split("?")[0] ?? path;

  if (pathname === "/") return "Início";
  if (pathname === "/classes") return "Minhas turmas";
  if (pathname === "/exercises") return "Exercícios";
  if (pathname === "/professor") return "Gerenciar turmas";
  if (pathname === "/admin") return "Painel";
  if (pathname === "/admin/courses") return "Turmas";
  if (pathname === "/admin/users") return "Usuários";
  if (pathname === "/admin/settings") return "Configurações";
  if (pathname === "/admin/file-types") return "Tipos de arquivo";
  if (pathname === "/profile") return "Meu perfil";

  if (pathname.startsWith("/professor/class/")) return "Turma";
  if (pathname.startsWith("/professor/exercise/")) return "Exercício";
  if (pathname.startsWith("/classes/")) return "Turma";
  if (pathname.startsWith("/exercises/")) return "Exercício";

  return "Voltar";
}
