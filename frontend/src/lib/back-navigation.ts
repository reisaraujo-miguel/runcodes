/**
 * Navigation history helpers for the back controls.
 *
 * A page can record where the user came from in `location.state.from`; these
 * helpers validate that value and turn an in-app path into a short label. They
 * are pure so the navigation rules can be tested without a browser history.
 */

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
