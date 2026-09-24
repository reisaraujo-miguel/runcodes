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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  ADMIN_PAGE_LIMIT,
  adminDeleteOffering,
  adminListOfferings,
  adminUpdateOffering,
  type AdminOffering,
  type UpdateAdminOfferingPayload,
} from "@/lib/api";
import { dateInputToTimestamp, formatDateTime } from "@/lib/format";

/** The API message of a failed request, or a fallback for unexpected errors. */
function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

/**
 * Converts an ISO timestamp into the YYYY-MM-DD value a date input expects.
 * Returns an empty string when the timestamp cannot be parsed.
 */
function toDateInputValue(timestamp: string): string {
  if (!timestamp) return "";
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) return "";
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${String(date.getFullYear()).padStart(4, "0")}-${month}-${day}`;
}

/**
 * Admin page that lists every class of the platform, edits it (including
 * transferring its ownership to another professor) and deletes it.
 */
export function AdminCoursesPage() {
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [reloadToken, setReloadToken] = useState(0);

  const [offerings, setOfferings] = useState<AdminOffering[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [notice, setNotice] = useState<string | null>(null);
  const [rowErrors, setRowErrors] = useState<Record<number, string>>({});
  const [pendingId, setPendingId] = useState<number | null>(null);
  const [confirmingDeleteId, setConfirmingDeleteId] = useState<number | null>(
    null,
  );

  const [edited, setEdited] = useState<AdminOffering | null>(null);
  const [editName, setEditName] = useState("");
  const [editDescription, setEditDescription] = useState("");
  const [editEndDate, setEditEndDate] = useState("");
  const [editVisibleToEnroll, setEditVisibleToEnroll] = useState(true);
  const [editOwnerId, setEditOwnerId] = useState("");
  const [editError, setEditError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let cancelled = false;

    async function loadOfferings() {
      try {
        const data = await adminListOfferings(query);
        if (cancelled) return;
        setOfferings(data);
        setLoadError(null);
      } catch (error) {
        if (!cancelled) {
          setLoadError(
            errorMessage(error, "Não foi possível carregar as turmas."),
          );
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    void loadOfferings();

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

  /** Opens the editor dialog with the row's current values. */
  function openEditor(offering: AdminOffering) {
    setEditName(offering.name);
    setEditDescription(offering.description);
    setEditEndDate(toDateInputValue(offering.end_date));
    setEditVisibleToEnroll(offering.visible_to_enroll);
    setEditOwnerId("");
    setEditError(null);
    setEdited(offering);
  }

  function closeEditor() {
    setEdited(null);
    setEditError(null);
  }

  function handleEditSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    const current = edited;
    if (current === null) return;

    const name = editName.trim();
    if (name === "") {
      setEditError("O nome da turma é obrigatório.");
      return;
    }

    const endDateIso = dateInputToTimestamp(editEndDate, "end");
    if (endDateIso === null) {
      setEditError("Informe uma data de encerramento válida.");
      return;
    }

    const payload: UpdateAdminOfferingPayload = {
      name,
      description: editDescription.trim(),
      end_date: endDateIso,
      visible_to_enroll: editVisibleToEnroll,
    };

    const ownerIdInput = editOwnerId.trim();
    if (ownerIdInput !== "") {
      const ownerId = Number(ownerIdInput);
      if (!Number.isInteger(ownerId) || ownerId <= 0) {
        setEditError("Informe um ID de responsável válido.");
        return;
      }
      payload.owner_id = ownerId;
    }

    setSaving(true);
    setEditError(null);

    void (async () => {
      try {
        const updated = await adminUpdateOffering(current.id, payload);
        // The API answers with the stored row, so it replaces the listed one.
        setOfferings((previous) =>
          previous === null
            ? previous
            : previous.map((offering) =>
                offering.id === updated.id ? updated : offering,
              ),
        );
        setNotice(`Turma ${updated.name} atualizada.`);
        setRowError(current.id, "");
        setEdited(null);
      } catch (error) {
        setEditError(errorMessage(error, "Erro ao atualizar a turma."));
      } finally {
        setSaving(false);
      }
    })();
  }

  async function handleDelete(offering: AdminOffering) {
    setPendingId(offering.id);
    setNotice(null);
    setRowError(offering.id, "");

    try {
      await adminDeleteOffering(offering.id);
      setConfirmingDeleteId(null);
      setNotice(`Turma ${offering.name} excluída.`);
      reload();
    } catch (error) {
      setRowError(offering.id, errorMessage(error, "Erro ao excluir a turma."));
    } finally {
      setPendingId(null);
    }
  }

  return (
    <div className="mx-auto max-w-5xl space-y-4 p-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-2xl">Turmas Cadastradas</CardTitle>
          <CardDescription>
            Busque por nome da turma ou pelo responsável para editar os dados,
            trocar o responsável ou excluir a turma.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
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
                placeholder="Nome da turma ou responsável"
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

          {loading && offerings === null && (
            <div className="flex justify-center p-12">
              <div
                aria-label="Carregando"
                className="h-6 w-6 animate-spin rounded-full border-2 border-muted-foreground/30 border-t-muted-foreground"
                role="status"
              />
            </div>
          )}

          {loading && offerings !== null && (
            <p className="text-muted-foreground text-sm">Atualizando…</p>
          )}

          {offerings !== null && offerings.length === 0 && (
            <p className="text-muted-foreground text-sm">
              Nenhuma turma encontrada.
            </p>
          )}

          {offerings !== null && offerings.length >= ADMIN_PAGE_LIMIT && (
            <p className="text-muted-foreground text-sm">
              Mostrando as primeiras {String(ADMIN_PAGE_LIMIT)} turmas. Refine a
              busca para ver as demais.
            </p>
          )}

          {offerings !== null && offerings.length > 0 && (
            <div className="overflow-x-auto rounded-lg border">
              <table className="w-full text-sm">
                <thead className="bg-muted/50 text-muted-foreground">
                  <tr className="text-left">
                    <th className="px-3 py-2 font-medium">ID</th>
                    <th className="px-3 py-2 font-medium">Nome</th>
                    <th className="px-3 py-2 font-medium">Responsável</th>
                    <th className="px-3 py-2 font-medium">Encerramento</th>
                    <th className="px-3 py-2 font-medium">Matriculados</th>
                    <th className="px-3 py-2 font-medium">Exercícios</th>
                    <th className="px-3 py-2 font-medium">Matrícula</th>
                    <th className="px-3 py-2 font-medium">Ações</th>
                  </tr>
                </thead>
                <tbody>
                  {offerings.map((offering) => {
                    const busy = pendingId === offering.id;
                    const rowError = rowErrors[offering.id];

                    return (
                      <tr key={offering.id} className="border-t align-top">
                        <td className="px-3 py-2 font-mono">{offering.id}</td>
                        <td className="px-3 py-2">
                          <p className="font-medium">{offering.name}</p>
                          {offering.description !== "" && (
                            <p className="text-muted-foreground text-xs">
                              {offering.description}
                            </p>
                          )}
                        </td>
                        <td className="px-3 py-2">
                          {offering.owner_id === null ? (
                            <span className="text-muted-foreground">—</span>
                          ) : (
                            <>
                              <p>{offering.owner_name || "—"}</p>
                              <p className="text-muted-foreground text-xs">
                                {offering.owner_email || "—"}
                              </p>
                            </>
                          )}
                        </td>
                        <td className="px-3 py-2">
                          {formatDateTime(offering.end_date)}
                        </td>
                        <td className="px-3 py-2">{offering.member_count}</td>
                        <td className="px-3 py-2">{offering.exercise_count}</td>
                        <td className="px-3 py-2">
                          <Badge
                            variant={
                              offering.visible_to_enroll
                                ? "success"
                                : "secondary"
                            }
                          >
                            {offering.visible_to_enroll
                              ? "Matrícula aberta"
                              : "Matrícula fechada"}
                          </Badge>
                        </td>
                        <td className="px-3 py-2">
                          <div className="flex flex-wrap items-center gap-2">
                            <Button
                              variant="outline"
                              size="sm"
                              disabled={busy}
                              onClick={() => {
                                openEditor(offering);
                              }}
                            >
                              Editar
                            </Button>
                            {confirmingDeleteId === offering.id ? (
                              <>
                                <Button
                                  variant="destructive"
                                  size="sm"
                                  disabled={busy}
                                  onClick={() => {
                                    void handleDelete(offering);
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
                                disabled={busy}
                                onClick={() => {
                                  setRowError(offering.id, "");
                                  setConfirmingDeleteId(offering.id);
                                }}
                              >
                                Excluir
                              </Button>
                            )}
                          </div>
                          {confirmingDeleteId === offering.id && (
                            <p className="text-destructive text-sm">
                              Excluir esta turma também apaga os exercícios dela
                              e as submissões dos alunos.
                            </p>
                          )}
                          {rowError && (
                            <p className="text-destructive text-sm">
                              {rowError}
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

      {edited !== null && (
        <Dialog
          open
          onOpenChange={(open) => {
            if (!open) closeEditor();
          }}
        >
          <DialogContent className="sm:max-w-2xl max-h-[calc(100vh-2rem)] overflow-y-auto">
            <DialogHeader>
              <DialogTitle>Editar turma</DialogTitle>
              <DialogDescription>
                Altere os dados da turma {edited.name} (id {edited.id}).
              </DialogDescription>
            </DialogHeader>

            <form className="space-y-4" onSubmit={handleEditSubmit}>
              <div className="space-y-2">
                <Label htmlFor="admin-course-name">Nome</Label>
                <Input
                  id="admin-course-name"
                  value={editName}
                  disabled={saving}
                  onChange={(event) => {
                    setEditName(event.target.value);
                  }}
                  required
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="admin-course-description">Descrição</Label>
                <Textarea
                  id="admin-course-description"
                  value={editDescription}
                  disabled={saving}
                  className="min-h-24 font-sans"
                  onChange={(event) => {
                    setEditDescription(event.target.value);
                  }}
                  placeholder="Descrição da turma (opcional)"
                />
              </div>

              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="admin-course-end-date">
                    Data de encerramento
                  </Label>
                  <Input
                    id="admin-course-end-date"
                    type="date"
                    value={editEndDate}
                    disabled={saving}
                    onChange={(event) => {
                      setEditEndDate(event.target.value);
                    }}
                    required
                  />
                </div>

                <div className="space-y-2">
                  <Label htmlFor="admin-course-owner-id">
                    ID do novo responsável
                  </Label>
                  <Input
                    id="admin-course-owner-id"
                    type="number"
                    min={1}
                    value={editOwnerId}
                    disabled={saving}
                    onChange={(event) => {
                      setEditOwnerId(event.target.value);
                    }}
                    placeholder="Deixe vazio para manter o responsável"
                  />
                  <p className="text-muted-foreground text-xs">
                    Responsável atual:{" "}
                    {edited.owner_id === null
                      ? "sem responsável"
                      : `${edited.owner_name || "—"} (id ${String(edited.owner_id)})`}
                  </p>
                </div>
              </div>

              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  className="accent-primary size-4"
                  checked={editVisibleToEnroll}
                  disabled={saving}
                  onChange={(event) => {
                    setEditVisibleToEnroll(event.target.checked);
                  }}
                />
                Matrícula aberta para os alunos
              </label>

              {editError && (
                <p className="text-destructive text-sm">{editError}</p>
              )}

              <DialogFooter>
                <Button
                  type="button"
                  variant="outline"
                  disabled={saving}
                  onClick={closeEditor}
                >
                  Cancelar
                </Button>
                <Button type="submit" disabled={saving}>
                  {saving ? "Salvando…" : "Salvar"}
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}
