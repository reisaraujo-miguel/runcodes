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
import {
  addOfferingMember,
  listOfferingMembers,
  removeOfferingMember,
  updateOfferingMember,
  type MemberRole,
  type OfferingMember,
} from "@/lib/api";
import { ApiRequestError } from "@/lib/api/client";

interface ClassMembersCardProps {
  offeringId: number;
}

/** The action a member endpoint call belongs to, used to read its failure. */
type MemberAction = "add" | "role" | "ban" | "remove";

/** Roles a professor may assign; `student` is only ever reached by enrolling. */
const ASSIGNABLE_ROLES: MemberRole[] = ["monitor", "professor"];

const ROLE_LABELS: Record<string, string> = {
  student: "Aluno",
  professor: "Professor",
  monitor: "Monitor",
};

type RoleBadgeVariant = "default" | "secondary" | "info" | "warning";

const ROLE_BADGE_VARIANTS: Record<string, RoleBadgeVariant> = {
  student: "secondary",
  professor: "info",
  monitor: "warning",
};

/**
 * Translates the failures a professor can realistically reach into the language
 * of the page. The API answers in English and the status is what identifies the
 * case; the action that was in flight disambiguates the statuses two different
 * failures share (a 409 is "already a member" when adding and "this is the
 * owner" when banning or removing).
 */
function memberErrorMessage(
  error: unknown,
  action: MemberAction,
  fallback: string,
): string {
  if (!(error instanceof ApiRequestError)) return fallback;

  if (error.status === 403) {
    return "Apenas o professor responsável pela turma pode gerenciar os participantes.";
  }
  if (error.status === 422) {
    return "O papel de professor exige uma conta de professor ou administrador.";
  }

  switch (action) {
    case "add":
      if (error.status === 404)
        return "Nenhum usuário cadastrado com esse e-mail.";
      if (error.status === 409) return "Esse usuário já participa da turma.";
      break;
    case "role":
      if (error.status === 400) return "Papel inválido para um participante.";
      break;
    case "ban":
    case "remove":
      if (error.status === 409) {
        return "O responsável pela turma não pode ser banido nem removido.";
      }
      break;
  }

  return fallback;
}

function roleLabel(role: string): string {
  return ROLE_LABELS[role] ?? role;
}

function roleBadgeVariant(role: string): RoleBadgeVariant {
  return ROLE_BADGE_VARIANTS[role] ?? "secondary";
}

/**
 * Manages who takes part in a class: students who enrolled with the code,
 * monitors and co-professors. Every action goes through an owner-only
 * endpoint, so a co-professor sees a note instead of the list.
 */
export function ClassMembersCard({ offeringId }: ClassMembersCardProps) {
  const [members, setMembers] = useState<OfferingMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [forbidden, setForbidden] = useState(false);

  // Unsaved role choices, keyed by member. A member without a draft shows the
  // role the API stored, so no draft has to be cleared after a save.
  const [roleDrafts, setRoleDrafts] = useState<Record<number, MemberRole>>({});
  const [memberAction, setMemberAction] = useState<{
    userId: number;
    kind: "role" | "ban" | "remove";
  } | null>(null);
  const [confirmingRemoveId, setConfirmingRemoveId] = useState<number | null>(
    null,
  );
  const [memberError, setMemberError] = useState<{
    userId: number;
    message: string;
  } | null>(null);

  const [newEmail, setNewEmail] = useState("");
  const [newRole, setNewRole] = useState<MemberRole>("monitor");
  const [adding, setAdding] = useState(false);
  const [addError, setAddError] = useState<string | null>(null);
  const [addSuccess, setAddSuccess] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function loadMembers() {
      try {
        const data = await listOfferingMembers(offeringId);
        if (!cancelled) setMembers(data);
      } catch (loadError) {
        if (cancelled) return;
        if (loadError instanceof ApiRequestError && loadError.status === 403) {
          setForbidden(true);
        } else {
          setError("Não foi possível carregar os participantes.");
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    void loadMembers();

    return () => {
      cancelled = true;
    };
  }, [offeringId]);

  function upsertMember(member: OfferingMember) {
    setMembers((previous) =>
      previous.some((item) => item.user_id === member.user_id)
        ? previous.map((item) =>
            item.user_id === member.user_id ? member : item,
          )
        : [...previous, member],
    );
  }

  /** The role the select shows: an unsaved draft, else the stored role. */
  function assignableRole(member: OfferingMember): MemberRole | null {
    const draft = roleDrafts[member.user_id];
    if (draft) return draft;
    if (member.role === "professor" || member.role === "monitor") {
      return member.role;
    }
    // Students keep the role they enrolled with: it is not a professor's to set.
    return null;
  }

  function handleSaveRole(member: OfferingMember) {
    const role = assignableRole(member);
    if (!role || role === member.role) return;
    setMemberAction({ userId: member.user_id, kind: "role" });
    setMemberError(null);
    void (async () => {
      try {
        upsertMember(
          await updateOfferingMember(offeringId, member.user_id, { role }),
        );
      } catch (actionError) {
        setMemberError({
          userId: member.user_id,
          message: memberErrorMessage(
            actionError,
            "role",
            "Erro ao alterar o papel do participante",
          ),
        });
      } finally {
        setMemberAction(null);
      }
    })();
  }

  function handleToggleBan(member: OfferingMember) {
    setMemberAction({ userId: member.user_id, kind: "ban" });
    setMemberError(null);
    void (async () => {
      try {
        upsertMember(
          await updateOfferingMember(offeringId, member.user_id, {
            banned: !member.banned,
          }),
        );
      } catch (actionError) {
        setMemberError({
          userId: member.user_id,
          message: memberErrorMessage(
            actionError,
            "ban",
            "Erro ao alterar a situação do participante",
          ),
        });
      } finally {
        setMemberAction(null);
      }
    })();
  }

  function handleRemove(userId: number) {
    setMemberAction({ userId, kind: "remove" });
    setMemberError(null);
    void (async () => {
      try {
        await removeOfferingMember(offeringId, userId);
        setMembers((previous) =>
          previous.filter((member) => member.user_id !== userId),
        );
        setConfirmingRemoveId(null);
      } catch (actionError) {
        setMemberError({
          userId,
          message: memberErrorMessage(
            actionError,
            "remove",
            "Erro ao remover o participante",
          ),
        });
      } finally {
        setMemberAction(null);
      }
    })();
  }

  function handleAdd(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    const email = newEmail.trim();
    if (!email) {
      setAddError("Informe o e-mail do participante");
      return;
    }

    setAdding(true);
    setAddError(null);
    setAddSuccess(null);
    void (async () => {
      try {
        const member = await addOfferingMember(offeringId, email, newRole);
        upsertMember(member);
        setNewEmail("");
        setAddSuccess(
          `${member.name} foi adicionado como ${roleLabel(member.role).toLowerCase()}.`,
        );
      } catch (addFailure) {
        setAddError(
          memberErrorMessage(
            addFailure,
            "add",
            "Erro ao adicionar o participante",
          ),
        );
      } finally {
        setAdding(false);
      }
    })();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Participantes</CardTitle>
        <CardDescription>
          Alunos matriculados, monitores e professores convidados.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {loading && (
          <p className="text-muted-foreground text-sm">
            Carregando participantes…
          </p>
        )}

        {forbidden && (
          <p className="text-muted-foreground text-sm">
            Apenas o professor responsável pela turma pode gerenciar os
            participantes.
          </p>
        )}

        {error && <p className="text-destructive text-sm">{error}</p>}

        {!loading && !forbidden && !error && members.length === 0 && (
          <p className="text-muted-foreground text-sm">
            Nenhum participante na turma ainda.
          </p>
        )}

        {!forbidden && members.length > 0 && (
          <ul className="space-y-3">
            {members.map((member) => {
              // The id of the action in flight, so every control of the row can
              // be disabled and the one that was pressed can report progress.
              const busyKind =
                memberAction?.userId === member.user_id
                  ? memberAction.kind
                  : null;
              const role = assignableRole(member);
              const confirming = confirmingRemoveId === member.user_id;
              const itemError =
                memberError?.userId === member.user_id
                  ? memberError.message
                  : null;

              return (
                <li
                  key={member.user_id}
                  className="space-y-2 rounded-lg border p-3"
                >
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="min-w-0 space-y-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <p className="font-medium">{member.name}</p>
                        <Badge variant={roleBadgeVariant(member.role)}>
                          {roleLabel(member.role)}
                        </Badge>
                        {member.banned && (
                          <Badge variant="destructive">Banido</Badge>
                        )}
                      </div>
                      <p className="text-muted-foreground text-sm break-all">
                        {member.email}
                      </p>
                    </div>

                    <div className="flex flex-wrap items-center justify-end gap-2">
                      {role && (
                        <>
                          <Select
                            aria-label={`Papel de ${member.name}`}
                            className="w-32"
                            value={role}
                            disabled={busyKind !== null}
                            onChange={(event) => {
                              const selected = event.target.value;
                              setRoleDrafts((previous) => ({
                                ...previous,
                                [member.user_id]:
                                  selected === "professor"
                                    ? "professor"
                                    : "monitor",
                              }));
                            }}
                          >
                            {ASSIGNABLE_ROLES.map((assignable) => (
                              <option key={assignable} value={assignable}>
                                {roleLabel(assignable)}
                              </option>
                            ))}
                          </Select>
                          <Button
                            size="sm"
                            variant="outline"
                            disabled={busyKind !== null || role === member.role}
                            onClick={() => {
                              handleSaveRole(member);
                            }}
                          >
                            {busyKind === "role" ? "Salvando…" : "Salvar"}
                          </Button>
                        </>
                      )}

                      <Button
                        size="sm"
                        variant="outline"
                        disabled={busyKind !== null}
                        onClick={() => {
                          handleToggleBan(member);
                        }}
                      >
                        {member.banned ? "Desbanir" : "Banir"}
                      </Button>

                      {confirming ? (
                        <>
                          <Button
                            size="sm"
                            variant="destructive"
                            disabled={busyKind !== null}
                            onClick={() => {
                              handleRemove(member.user_id);
                            }}
                          >
                            {busyKind === "remove"
                              ? "Removendo…"
                              : "Confirmar remoção"}
                          </Button>
                          <Button
                            size="sm"
                            variant="outline"
                            disabled={busyKind !== null}
                            onClick={() => {
                              setConfirmingRemoveId(null);
                            }}
                          >
                            Cancelar
                          </Button>
                        </>
                      ) : (
                        <Button
                          size="sm"
                          variant="destructive"
                          disabled={busyKind !== null}
                          onClick={() => {
                            setConfirmingRemoveId(member.user_id);
                            setMemberError(null);
                          }}
                        >
                          Remover
                        </Button>
                      )}
                    </div>
                  </div>

                  {itemError && (
                    <p className="text-destructive text-sm">{itemError}</p>
                  )}
                </li>
              );
            })}
          </ul>
        )}

        {!forbidden && (
          <form className="space-y-3 border-t pt-4" onSubmit={handleAdd}>
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
                      event.target.value === "professor"
                        ? "professor"
                        : "monitor",
                    );
                  }}
                >
                  {ASSIGNABLE_ROLES.map((assignable) => (
                    <option key={assignable} value={assignable}>
                      {roleLabel(assignable)}
                    </option>
                  ))}
                </Select>
              </div>
              <Button type="submit" disabled={adding}>
                {adding ? "Adicionando…" : "Adicionar"}
              </Button>
            </div>

            {addError && <p className="text-destructive text-sm">{addError}</p>}
            {addSuccess && (
              <p className="text-muted-foreground text-sm">{addSuccess}</p>
            )}
          </form>
        )}
      </CardContent>
    </Card>
  );
}
