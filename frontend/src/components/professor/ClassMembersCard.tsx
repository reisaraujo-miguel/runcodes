import { useState } from "react";

import { UsersIcon } from "lucide-react";

import { ConfirmButton } from "@/components/app/ConfirmDialog";
import { EnrollmentRoleBadge } from "@/components/app/RoleBadge";
import { SectionCard } from "@/components/app/SectionCard";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useAsync } from "@/hooks/use-async";
import { useSubmit } from "@/hooks/use-submit";
import {
  addOfferingMember,
  listOfferingMembers,
  removeOfferingMember,
  updateOfferingMember,
  type MemberRole,
  type OfferingMember,
} from "@/lib/api";
import { ApiRequestError } from "@/lib/api/client";
import { errorMessage } from "@/lib/errors";
import { enrollmentRoleLabel } from "@/lib/roles";

const ASSIGNABLE_ROLES: MemberRole[] = ["monitor", "professor"];

/** Translates the failures a professor can realistically hit when adding. */
function addErrorMessage(error: unknown): string {
  if (error instanceof ApiRequestError) {
    switch (error.status) {
      case 403:
        return "Apenas o professor responsável pela turma pode gerenciar os participantes.";
      case 404:
        return "Nenhum usuário cadastrado com esse e-mail.";
      case 409:
        return "Esse usuário já participa da turma.";
      case 422:
        return "O papel de professor exige uma conta de professor ou administrador.";
    }
  }
  return errorMessage(error, "Não foi possível adicionar o participante.");
}

/** Translates the failures of changing a member's role or situation. */
function memberErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiRequestError) {
    if (error.status === 403) {
      return "Apenas o professor responsável pela turma pode gerenciar os participantes.";
    }
    if (error.status === 409) {
      return "O responsável pela turma não pode ser banido nem removido.";
    }
    if (error.status === 422) {
      return "O papel de professor exige uma conta de professor ou administrador.";
    }
  }
  return errorMessage(error, fallback);
}

/**
 * Manages who takes part in a class: enrolled students, monitors and invited
 * professors. Every endpoint is owner-only, so a co-professor sees a note.
 */
export function ClassMembersCard({
  offeringId,
  isOwner,
  ownerId,
}: {
  offeringId: number;
  isOwner: boolean;
  ownerId: number | null;
}) {
  const members = useAsync(
    () => (isOwner ? listOfferingMembers(offeringId) : Promise.resolve([])),
    `members-${String(offeringId)}-${isOwner ? "owner" : "guest"}`,
    "Não foi possível carregar os participantes.",
  );

  const [roleDrafts, setRoleDrafts] = useState<Record<number, string>>({});
  const [pendingUserId, setPendingUserId] = useState<number | null>(null);
  const [rowErrors, setRowErrors] = useState<Record<number, string>>({});

  const [newEmail, setNewEmail] = useState("");
  const [newRole, setNewRole] = useState<MemberRole>("monitor");
  const add = useSubmit("Não foi possível adicionar o participante.");

  if (!isOwner) {
    return (
      <SectionCard
        title="Participantes"
        description="Alunos matriculados, monitores e professores convidados."
      >
        <p className="text-muted-foreground text-sm">
          Apenas o professor responsável pela turma pode gerenciar os
          participantes.
        </p>
      </SectionCard>
    );
  }

  const list = members.data ?? [];

  function setRowError(userId: number, message: string) {
    setRowErrors((previous) => ({ ...previous, [userId]: message }));
  }

  async function handleSaveRole(member: OfferingMember) {
    const role = roleDrafts[member.user_id] ?? member.role;
    if (role !== "monitor" && role !== "professor") return;
    if (role === member.role) return;

    setPendingUserId(member.user_id);
    setRowError(member.user_id, "");
    try {
      await updateOfferingMember(offeringId, member.user_id, { role });
      members.reload();
    } catch (error) {
      setRowError(
        member.user_id,
        memberErrorMessage(error, "Erro ao alterar o papel do participante."),
      );
    } finally {
      setPendingUserId(null);
    }
  }

  async function handleToggleBan(member: OfferingMember) {
    setPendingUserId(member.user_id);
    setRowError(member.user_id, "");
    try {
      await updateOfferingMember(offeringId, member.user_id, {
        banned: !member.banned,
      });
      members.reload();
    } catch (error) {
      setRowError(
        member.user_id,
        memberErrorMessage(
          error,
          "Erro ao alterar a situação do participante.",
        ),
      );
    } finally {
      setPendingUserId(null);
    }
  }

  async function handleRemove(userId: number) {
    setPendingUserId(userId);
    setRowError(userId, "");
    try {
      await removeOfferingMember(offeringId, userId);
      members.reload();
    } catch (error) {
      setRowError(
        userId,
        memberErrorMessage(error, "Erro ao remover o participante."),
      );
    } finally {
      setPendingUserId(null);
    }
  }

  function handleAdd() {
    const email = newEmail.trim();
    if (!email) {
      add.setError("Informe o e-mail do participante.");
      return;
    }

    void add.run(async () => {
      let member: OfferingMember;
      try {
        member = await addOfferingMember(offeringId, email, newRole);
      } catch (error) {
        // Rethrow with the translated message so `useSubmit` surfaces it.
        throw new Error(addErrorMessage(error), { cause: error });
      }
      setNewEmail("");
      members.reload();
      add.setSuccess(
        `${member.name} foi adicionado como ${enrollmentRoleLabel(member.role).toLowerCase()}.`,
      );
    });
  }

  return (
    <SectionCard
      title="Participantes"
      description="Alunos matriculados, monitores e professores convidados."
    >
      {members.loading && members.data === null ? (
        <LoadingState className="py-8" />
      ) : members.error ? (
        <ErrorState description={members.error} onRetry={members.reload} />
      ) : list.length === 0 ? (
        <EmptyState
          icon={UsersIcon}
          title="Nenhum participante ainda"
          description="Compartilhe o código de matrícula ou adicione um monitor ou professor."
        />
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Participante</TableHead>
              <TableHead>Papel</TableHead>
              <TableHead className="text-right">Ações</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {list.map((member) => {
              const isResponsible = ownerId === member.user_id;
              const busy = pendingUserId === member.user_id;
              const rowError = rowErrors[member.user_id];
              const draft = roleDrafts[member.user_id] ?? member.role;

              return (
                <TableRow key={member.user_id}>
                  <TableCell>
                    <p className="font-medium">{member.name}</p>
                    <p className="text-muted-foreground text-xs break-all">
                      {member.email}
                    </p>
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap items-center gap-2">
                      <EnrollmentRoleBadge role={member.role} />
                      {member.banned ? (
                        <Badge variant="destructive">Banido</Badge>
                      ) : null}
                      {isResponsible ? (
                        <Badge variant="outline">Responsável</Badge>
                      ) : null}
                    </div>
                  </TableCell>
                  <TableCell>
                    {isResponsible ? (
                      <p className="text-muted-foreground text-right text-xs">
                        O responsável não pode ser alterado.
                      </p>
                    ) : (
                      <div className="flex flex-wrap items-center justify-end gap-2">
                        <Select
                          aria-label={`Papel de ${member.name}`}
                          className="w-32"
                          value={draft}
                          disabled={busy}
                          onChange={(event) => {
                            const selected = event.target.value;
                            setRoleDrafts((previous) => ({
                              ...previous,
                              [member.user_id]: selected,
                            }));
                          }}
                        >
                          {member.role === "student" ? (
                            <option value="student">Aluno</option>
                          ) : null}
                          {ASSIGNABLE_ROLES.map((assignable) => (
                            <option key={assignable} value={assignable}>
                              {enrollmentRoleLabel(assignable)}
                            </option>
                          ))}
                        </Select>
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={busy || draft === member.role}
                          onClick={() => {
                            void handleSaveRole(member);
                          }}
                        >
                          {busy ? <Spinner className="size-4" /> : null}
                          Salvar
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={busy}
                          onClick={() => {
                            void handleToggleBan(member);
                          }}
                        >
                          {member.banned ? "Desbanir" : "Banir"}
                        </Button>
                        <ConfirmButton
                          size="sm"
                          destructive
                          variant="destructive"
                          disabled={busy}
                          title="Remover participante"
                          description={`${member.name} deixará de participar da turma.`}
                          confirmLabel="Remover"
                          onConfirm={() => handleRemove(member.user_id)}
                        >
                          Remover
                        </ConfirmButton>
                      </div>
                    )}
                    {rowError ? (
                      <p className="text-destructive mt-2 text-right text-xs">
                        {rowError}
                      </p>
                    ) : null}
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      )}

      <form
        className="mt-4 space-y-3 border-t pt-4"
        onSubmit={(event) => {
          event.preventDefault();
          handleAdd();
        }}
      >
        <p className="text-sm font-medium">Adicionar participante</p>
        <div className="flex flex-wrap items-end gap-3">
          <div className="min-w-56 flex-1 space-y-2">
            <Label htmlFor="member-email">E-mail</Label>
            <Input
              id="member-email"
              type="email"
              value={newEmail}
              onChange={(event) => {
                setNewEmail(event.target.value);
              }}
              placeholder="pessoa@exemplo.com"
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="member-role">Papel</Label>
            <Select
              id="member-role"
              className="w-36"
              value={newRole}
              onChange={(event) => {
                setNewRole(
                  event.target.value === "professor" ? "professor" : "monitor",
                );
              }}
            >
              {ASSIGNABLE_ROLES.map((assignable) => (
                <option key={assignable} value={assignable}>
                  {enrollmentRoleLabel(assignable)}
                </option>
              ))}
            </Select>
          </div>
          <Button type="submit" disabled={add.pending}>
            {add.pending ? <Spinner className="size-4" /> : null}
            Adicionar
          </Button>
        </div>

        {add.error ? (
          <Alert variant="destructive">
            <AlertDescription>{add.error}</AlertDescription>
          </Alert>
        ) : null}
        {add.success ? (
          <Alert variant="success">
            <AlertDescription>{add.success}</AlertDescription>
          </Alert>
        ) : null}
      </form>
    </SectionCard>
  );
}
