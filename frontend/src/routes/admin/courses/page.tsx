import { useState, type SubmitEvent } from "react";

import {
  GraduationCapIcon,
  PencilIcon,
  SearchIcon,
  Trash2Icon,
  UsersIcon,
} from "lucide-react";

import { OfferingEditDialog } from "@/components/admin/OfferingEditDialog";
import { OfferingMembersDialog } from "@/components/admin/OfferingMembersDialog";
import { useAdminList } from "@/components/admin/use-admin-list";
import { ConfirmButton } from "@/components/app/ConfirmDialog";
import { PageHeader } from "@/components/app/PageHeader";
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
import {
  adminDeleteOffering,
  adminListOfferings,
  type AdminOffering,
} from "@/lib/api";
import { errorMessage } from "@/lib/errors";
import { formatDate } from "@/lib/format";

/**
 * Lists every class on the platform. An admin can edit its data, transfer
 * ownership, inspect its members or delete it (with its exercises and
 * submissions).
 */
export function AdminCoursesPage() {
  const list = useAdminList(
    adminListOfferings,
    "admin-offerings",
    "Não foi possível carregar as turmas.",
  );

  const [search, setSearch] = useState("");
  const [editing, setEditing] = useState<AdminOffering | null>(null);
  const [viewingMembers, setViewingMembers] = useState<AdminOffering | null>(
    null,
  );
  const [notice, setNotice] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  function handleSearchSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    setNotice(null);
    setActionError(null);
    list.search(search);
  }

  async function handleDelete(offering: AdminOffering) {
    try {
      await adminDeleteOffering(offering.id);
      list.remove(offering.id);
      setActionError(null);
      setNotice(`Turma ${offering.name} excluída.`);
    } catch (error) {
      setNotice(null);
      setActionError(errorMessage(error, "Erro ao excluir a turma."));
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Turmas"
        description="Busque por nome da turma ou pelo responsável para editar, transferir a responsabilidade ou excluir."
      />

      <SectionCard
        title="Todas as turmas"
        description="Cada turma reúne os exercícios e as matrículas dos alunos."
      >
        <div className="space-y-4">
          <form
            className="flex flex-wrap items-end gap-2"
            onSubmit={handleSearchSubmit}
          >
            <div className="min-w-64 flex-1 space-y-2">
              <Label htmlFor="admin-course-search">Buscar turma</Label>
              <Input
                id="admin-course-search"
                value={search}
                onChange={(event) => {
                  setSearch(event.target.value);
                }}
                placeholder="Nome da turma, responsável ou email"
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
              icon={GraduationCapIcon}
              title="Nenhuma turma encontrada"
              description={
                list.query === ""
                  ? "Ainda não há turmas cadastradas."
                  : "Nenhuma turma corresponde à busca."
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
                    <TableHead scope="col">Turma</TableHead>
                    <TableHead scope="col">Responsável</TableHead>
                    <TableHead scope="col">Membros</TableHead>
                    <TableHead scope="col">Exercícios</TableHead>
                    <TableHead scope="col">Matrícula</TableHead>
                    <TableHead scope="col">Encerramento</TableHead>
                    <TableHead scope="col" className="text-right">
                      Ações
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {list.items.map((offering) => (
                    <TableRow key={offering.id}>
                      <TableCell>
                        <p className="font-medium">{offering.name}</p>
                        {offering.description ? (
                          <p className="text-muted-foreground text-xs">
                            {offering.description}
                          </p>
                        ) : null}
                        <p className="text-muted-foreground text-xs">
                          #{offering.id}
                        </p>
                      </TableCell>
                      <TableCell>
                        {offering.owner_id === null ? (
                          <span className="text-muted-foreground">
                            Sem responsável
                          </span>
                        ) : (
                          <>
                            <p>{offering.owner_name || "—"}</p>
                            <p className="text-muted-foreground text-xs">
                              {offering.owner_email || "—"}
                            </p>
                          </>
                        )}
                      </TableCell>
                      <TableCell className="tabular-nums">
                        {offering.member_count}
                      </TableCell>
                      <TableCell className="tabular-nums">
                        {offering.exercise_count}
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={
                            offering.visible_to_enroll ? "success" : "secondary"
                          }
                        >
                          {offering.visible_to_enroll
                            ? "Matrícula aberta"
                            : "Matrícula fechada"}
                        </Badge>
                      </TableCell>
                      <TableCell>{formatDate(offering.end_date)}</TableCell>
                      <TableCell>
                        <div className="flex items-center justify-end gap-1">
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => {
                              setActionError(null);
                              setEditing(offering);
                            }}
                          >
                            <PencilIcon />
                            Editar
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => {
                              setViewingMembers(offering);
                            }}
                          >
                            <UsersIcon />
                            Ver membros
                          </Button>
                          <ConfirmButton
                            variant="destructive"
                            size="sm"
                            destructive
                            title="Excluir turma"
                            confirmLabel="Excluir"
                            description={
                              <>
                                Excluir esta turma também apaga os exercícios
                                dela e as submissões dos alunos. Esta ação não
                                pode ser desfeita.
                              </>
                            }
                            onConfirm={() => handleDelete(offering)}
                          >
                            <Trash2Icon />
                            Excluir
                          </ConfirmButton>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>

              <div className="flex flex-wrap items-center justify-between gap-3">
                <p className="text-muted-foreground text-sm">
                  {list.items.length} turma(s) exibida(s).
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
        <OfferingEditDialog
          key={editing.id}
          offering={editing}
          onClose={() => {
            setEditing(null);
          }}
          onSaved={(updated) => {
            list.update(updated.id, updated);
            setNotice(`Turma ${updated.name} atualizada.`);
            setEditing(null);
          }}
        />
      ) : null}

      {viewingMembers ? (
        <OfferingMembersDialog
          key={viewingMembers.id}
          offering={viewingMembers}
          onClose={() => {
            setViewingMembers(null);
          }}
        />
      ) : null}
    </div>
  );
}
