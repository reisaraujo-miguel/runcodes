import { useState, type SubmitEvent } from "react";

import { PencilIcon, SearchIcon, Trash2Icon, UsersIcon } from "lucide-react";

import { UserEditDialog } from "@/components/admin/UserEditDialog";
import { useAdminList } from "@/components/admin/use-admin-list";
import { ConfirmButton } from "@/components/app/ConfirmDialog";
import { PageHeader } from "@/components/app/PageHeader";
import { PlatformRoleBadge } from "@/components/app/RoleBadge";
import { SectionCard } from "@/components/app/SectionCard";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useAuth } from "@/hooks/use-auth";
import { adminDeleteUser, adminListUsers, type AdminUser } from "@/lib/api";
import { errorMessage } from "@/lib/errors";
import { formatDate } from "@/lib/format";

/**
 * Lists every account and lets an admin edit it (name, email, org id, role and
 * confirmation) or delete it — which also removes the classes the user owns.
 * The signed-in admin cannot change their own role or delete themselves.
 */
export function AdminUsersPage() {
  const { user: currentUser } = useAuth();
  const list = useAdminList(
    adminListUsers,
    "admin-users",
    "Não foi possível carregar os usuários.",
  );

  const [search, setSearch] = useState("");
  const [editing, setEditing] = useState<AdminUser | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const currentUserId = currentUser?.id ?? null;

  function handleSearchSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    setNotice(null);
    setActionError(null);
    list.search(search);
  }

  async function handleDelete(user: AdminUser) {
    try {
      await adminDeleteUser(user.id);
      list.remove(user.id);
      setActionError(null);
      setNotice(`${user.name} foi excluído.`);
    } catch (error) {
      setNotice(null);
      setActionError(errorMessage(error, "Erro ao excluir o usuário."));
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Usuários"
        description="Busque por nome, email ou id da organização para editar ou excluir contas."
      />

      <SectionCard
        title="Contas da plataforma"
        description="Edite os dados de uma conta ou remova-a; a exclusão apaga as turmas que ela possui."
      >
        <div className="space-y-4">
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
            <Button type="submit" disabled={list.loading}>
              <SearchIcon />
              Buscar
            </Button>
          </form>

          {notice ? (
            <Alert variant="success">
              <AlertDescription>{notice}</AlertDescription>
            </Alert>
          ) : null}
          {actionError ? (
            <Alert variant="destructive">
              <AlertDescription>{actionError}</AlertDescription>
            </Alert>
          ) : null}

          {list.loading ? (
            <LoadingState />
          ) : list.error && list.items.length === 0 ? (
            <ErrorState description={list.error} onRetry={list.reload} />
          ) : list.items.length === 0 ? (
            <EmptyState
              icon={UsersIcon}
              title="Nenhum usuário encontrado"
              description={
                list.query === ""
                  ? "Ainda não há contas cadastradas."
                  : "Nenhuma conta corresponde à busca."
              }
            />
          ) : (
            <div className="space-y-4">
              {list.error ? (
                <Alert variant="destructive">
                  <AlertDescription>
                    <div>{list.error}</div>
                    <Button variant="outline" size="sm" onClick={list.retry}>
                      Tentar novamente
                    </Button>
                  </AlertDescription>
                </Alert>
              ) : null}

              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead scope="col">Nome</TableHead>
                    <TableHead scope="col">Email</TableHead>
                    <TableHead scope="col">Organização</TableHead>
                    <TableHead scope="col">Papel</TableHead>
                    <TableHead scope="col">Confirmado</TableHead>
                    <TableHead scope="col">Criado em</TableHead>
                    <TableHead scope="col" className="text-right">
                      Ações
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {list.items.map((user) => {
                    const isSelf = currentUserId === user.id;
                    return (
                      <TableRow key={user.id}>
                        <TableCell>
                          <p className="font-medium">{user.name}</p>
                          <p className="text-muted-foreground text-xs">
                            {isSelf ? "Você · " : ""}#{user.id}
                          </p>
                        </TableCell>
                        <TableCell>{user.email}</TableCell>
                        <TableCell>{user.org_id || "—"}</TableCell>
                        <TableCell>
                          <PlatformRoleBadge role={user.role} />
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant={user.confirmed ? "success" : "warning"}
                          >
                            {user.confirmed ? "Confirmado" : "Pendente"}
                          </Badge>
                        </TableCell>
                        <TableCell>{formatDate(user.created_at)}</TableCell>
                        <TableCell>
                          <div className="flex items-center justify-end gap-1">
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => {
                                setActionError(null);
                                setEditing(user);
                              }}
                            >
                              <PencilIcon />
                              Editar
                            </Button>
                            <ConfirmButton
                              variant="destructive"
                              size="sm"
                              destructive
                              disabled={isSelf}
                              title="Excluir usuário"
                              confirmLabel="Excluir"
                              description={
                                <>
                                  Excluir {user.name} também apaga as turmas que
                                  a conta possui e as submissões enviadas por
                                  ela. Esta ação não pode ser desfeita.
                                </>
                              }
                              onConfirm={() => handleDelete(user)}
                            >
                              <Trash2Icon />
                              Excluir
                            </ConfirmButton>
                          </div>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>

              <div className="flex flex-wrap items-center justify-between gap-3">
                <p className="text-muted-foreground text-sm">
                  {list.items.length} usuário(s) exibido(s).
                </p>
                {list.hasMore ? (
                  <Button
                    variant="outline"
                    disabled={list.loadingMore}
                    onClick={list.loadMore}
                  >
                    {list.loadingMore ? <Spinner className="size-4" /> : null}
                    Carregar mais
                  </Button>
                ) : null}
              </div>
            </div>
          )}
        </div>
      </SectionCard>

      {editing ? (
        <UserEditDialog
          key={editing.id}
          user={editing}
          isSelf={currentUserId === editing.id}
          onClose={() => {
            setEditing(null);
          }}
          onSaved={(updated) => {
            list.update(updated.id, updated);
            setNotice(`${updated.name} foi atualizado.`);
            setEditing(null);
          }}
        />
      ) : null}
    </div>
  );
}
