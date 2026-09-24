import { useEffect, useState, type SubmitEvent } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { useAuth } from "@/hooks/use-auth";
import {
  ADMIN_PAGE_LIMIT,
  USER_ROLES,
  adminDeleteUser,
  adminListUsers,
  adminUpdateUser,
  type AdminUser,
} from "@/lib/api";
import { formatDateTime } from "@/lib/format";

const ROLE_LABELS: Record<string, string> = {
  student: "Aluno",
  professor: "Professor",
  admin: "Administrador",
  dev: "Desenvolvedor",
};

/** The API message of a failed request, or a fallback for unexpected errors. */
function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

/**
 * Admin page that lists every account of the platform and edits it in place:
 * the role, the confirmation status and the deletion of the account (which also
 * removes the classes the user owns and their submissions).
 */
export function AdminUsersPage() {
  const { user: currentUser } = useAuth();

  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [reloadToken, setReloadToken] = useState(0);

  const [users, setUsers] = useState<AdminUser[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [draftRoles, setDraftRoles] = useState<Record<number, string>>({});
  const [rowErrors, setRowErrors] = useState<Record<number, string>>({});
  const [notice, setNotice] = useState<string | null>(null);
  const [pendingUserId, setPendingUserId] = useState<number | null>(null);
  const [confirmingDeleteId, setConfirmingDeleteId] = useState<number | null>(
    null,
  );

  useEffect(() => {
    let cancelled = false;

    async function loadUsers() {
      try {
        const data = await adminListUsers(query);
        if (cancelled) return;
        setUsers(data);
        setLoadError(null);
      } catch (error) {
        if (!cancelled) {
          setLoadError(
            errorMessage(error, "Não foi possível carregar os usuários."),
          );
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    void loadUsers();

    return () => {
      cancelled = true;
    };
  }, [query, reloadToken]);

  /** Re-reads the current search from the API, e.g. after a mutation. */
  function reload() {
    setLoading(true);
    setReloadToken((previous) => previous + 1);
  }

  function handleSearchSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    setNotice(null);
    setLoading(true);
    const next = search.trim();
    // An unchanged term must still re-run the request, which the token forces.
    if (next === query) {
      setReloadToken((previous) => previous + 1);
      return;
    }
    setQuery(next);
  }

  function setRowError(id: number, message: string) {
    setRowErrors((previous) => ({ ...previous, [id]: message }));
  }

  async function handleRoleSave(user: AdminUser) {
    const role = draftRoles[user.id] ?? user.role;

    setPendingUserId(user.id);
    setNotice(null);
    setRowError(user.id, "");

    try {
      await adminUpdateUser(user.id, { role });
      setNotice(`Papel de ${user.name} atualizado.`);
      reload();
    } catch (error) {
      setRowError(
        user.id,
        errorMessage(error, "Erro ao atualizar o papel do usuário."),
      );
    } finally {
      setPendingUserId(null);
    }
  }

  async function handleConfirmToggle(user: AdminUser) {
    setPendingUserId(user.id);
    setNotice(null);
    setRowError(user.id, "");

    try {
      await adminUpdateUser(user.id, { confirmed: !user.confirmed });
      setNotice(
        user.confirmed
          ? `${user.name} não está mais confirmado.`
          : `${user.name} foi confirmado.`,
      );
      reload();
    } catch (error) {
      setRowError(
        user.id,
        errorMessage(error, "Erro ao alterar a confirmação do usuário."),
      );
    } finally {
      setPendingUserId(null);
    }
  }

  async function handleDelete(user: AdminUser) {
    setPendingUserId(user.id);
    setNotice(null);
    setRowError(user.id, "");

    try {
      await adminDeleteUser(user.id);
      setConfirmingDeleteId(null);
      setNotice(`${user.name} foi excluído.`);
      reload();
    } catch (error) {
      setRowError(user.id, errorMessage(error, "Erro ao excluir o usuário."));
    } finally {
      setPendingUserId(null);
    }
  }

  const currentUserId = currentUser?.id ?? null;

  return (
    <div className="mx-auto max-w-5xl space-y-4 p-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-2xl">Usuários</CardTitle>
          <CardDescription>
            Busque por nome, email ou id da organização para ajustar o papel, a
            confirmação ou excluir a conta.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <form
            className="flex flex-wrap items-end gap-2"
            onSubmit={handleSearchSubmit}
          >
            <div className="min-w-64 flex-1 space-y-2">
              <Label htmlFor="admin-user-search">Buscar usuário</Label>
              <Input
                id="admin-user-search"
                value={search}
                onChange={(event) => {
                  setSearch(event.target.value);
                }}
                placeholder="Nome, email ou id da organização"
              />
            </div>
            <Button type="submit" disabled={loading}>
              Buscar
            </Button>
          </form>

          {notice && (
            <p className="text-sm text-emerald-600 dark:text-emerald-400">
              {notice}
            </p>
          )}

          {loadError && <p className="text-destructive text-sm">{loadError}</p>}

          {loading && users === null && (
            <div className="flex justify-center p-12">
              <div
                aria-label="Carregando"
                className="h-6 w-6 animate-spin rounded-full border-2 border-muted-foreground/30 border-t-muted-foreground"
                role="status"
              />
            </div>
          )}

          {loading && users !== null && (
            <p className="text-muted-foreground text-sm">Atualizando…</p>
          )}

          {users !== null && users.length === 0 && (
            <p className="text-muted-foreground text-sm">
              Nenhum usuário encontrado.
            </p>
          )}

          {users !== null && users.length >= ADMIN_PAGE_LIMIT && (
            <p className="text-muted-foreground text-sm">
              Mostrando os primeiros {String(ADMIN_PAGE_LIMIT)} usuários. Refine
              a busca para ver os demais.
            </p>
          )}

          {users !== null && users.length > 0 && (
            <div className="overflow-x-auto rounded-lg border">
              <table className="w-full text-sm">
                <thead className="bg-muted/50 text-muted-foreground">
                  <tr className="text-left">
                    <th className="px-3 py-2 font-medium">ID</th>
                    <th className="px-3 py-2 font-medium">Nome</th>
                    <th className="px-3 py-2 font-medium">Email</th>
                    <th className="px-3 py-2 font-medium">Organização</th>
                    <th className="px-3 py-2 font-medium">Papel</th>
                    <th className="px-3 py-2 font-medium">Confirmado</th>
                    <th className="px-3 py-2 font-medium">Criado em</th>
                    <th className="px-3 py-2 font-medium">Ações</th>
                  </tr>
                </thead>
                <tbody>
                  {users.map((user) => {
                    const isSelf = currentUserId === user.id;
                    const busy = pendingUserId === user.id;
                    const draftRole = draftRoles[user.id] ?? user.role;
                    const rowError = rowErrors[user.id];

                    return (
                      <tr key={user.id} className="border-t align-top">
                        <td className="px-3 py-2 font-mono">{user.id}</td>
                        <td className="px-3 py-2">
                          <p className="font-medium">{user.name}</p>
                          {isSelf && (
                            <p className="text-muted-foreground text-xs">
                              Você
                            </p>
                          )}
                        </td>
                        <td className="px-3 py-2">{user.email}</td>
                        <td className="px-3 py-2">{user.org_id || "—"}</td>
                        <td className="px-3 py-2">
                          <div className="flex flex-wrap items-center gap-2">
                            <Select
                              aria-label={`Papel de ${user.name}`}
                              className="w-36"
                              value={draftRole}
                              disabled={isSelf || busy}
                              onChange={(event) => {
                                const role = event.target.value;
                                setDraftRoles((previous) => ({
                                  ...previous,
                                  [user.id]: role,
                                }));
                              }}
                            >
                              {USER_ROLES.map((role) => (
                                <option key={role} value={role}>
                                  {ROLE_LABELS[role] ?? role}
                                </option>
                              ))}
                            </Select>
                            <Button
                              size="sm"
                              disabled={
                                isSelf || busy || draftRole === user.role
                              }
                              onClick={() => {
                                void handleRoleSave(user);
                              }}
                            >
                              Salvar
                            </Button>
                          </div>
                          {isSelf && (
                            <p className="text-muted-foreground text-xs">
                              O papel da sua conta não pode ser alterado.
                            </p>
                          )}
                        </td>
                        <td className="px-3 py-2">
                          <Badge
                            variant={user.confirmed ? "success" : "warning"}
                          >
                            {user.confirmed ? "Confirmado" : "Pendente"}
                          </Badge>
                        </td>
                        <td className="px-3 py-2">
                          {formatDateTime(user.created_at)}
                        </td>
                        <td className="px-3 py-2">
                          <div className="flex flex-wrap items-center gap-2">
                            <Button
                              variant="outline"
                              size="sm"
                              disabled={isSelf || busy}
                              onClick={() => {
                                void handleConfirmToggle(user);
                              }}
                            >
                              {user.confirmed ? "Desconfirmar" : "Confirmar"}
                            </Button>
                            {confirmingDeleteId === user.id ? (
                              <>
                                <Button
                                  variant="destructive"
                                  size="sm"
                                  disabled={busy}
                                  onClick={() => {
                                    void handleDelete(user);
                                  }}
                                >
                                  Confirmar exclusão
                                </Button>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => {
                                    setConfirmingDeleteId(null);
                                  }}
                                >
                                  Cancelar
                                </Button>
                              </>
                            ) : (
                              <Button
                                variant="destructive"
                                size="sm"
                                disabled={isSelf || busy}
                                onClick={() => {
                                  setRowError(user.id, "");
                                  setConfirmingDeleteId(user.id);
                                }}
                              >
                                Excluir
                              </Button>
                            )}
                          </div>
                          {confirmingDeleteId === user.id && (
                            <p className="text-destructive text-sm">
                              Excluir esta conta também apaga as turmas que ela
                              possui e as submissões enviadas por ela.
                            </p>
                          )}
                          {rowError && (
                            <p className="text-destructive text-sm">
                              {rowError}
                            </p>
                          )}
                          {isSelf && (
                            <p className="text-muted-foreground text-xs">
                              A sua própria conta não pode ser excluída.
                            </p>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
