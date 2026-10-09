import { useState, type SubmitEvent } from "react";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
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
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { useSubmit } from "@/hooks/use-submit";
import {
  adminUpdateOffering,
  type AdminOffering,
  type UpdateAdminOfferingPayload,
} from "@/lib/api";
import { dateInputToTimestamp, formatDateInput } from "@/lib/format";

/**
 * Edits a class from the admin panel: its name, description, deadline and
 * enrollment visibility, and transfers ownership to another user id.
 */
export function OfferingEditDialog({
  offering,
  onClose,
  onSaved,
}: {
  offering: AdminOffering;
  onClose: () => void;
  onSaved: (offering: AdminOffering) => void;
}) {
  const submit = useSubmit("Não foi possível salvar a turma.");
  const [name, setName] = useState(offering.name);
  const [description, setDescription] = useState(offering.description);
  const [endDate, setEndDate] = useState(() =>
    formatDateInput(offering.end_date),
  );
  const [visibleToEnroll, setVisibleToEnroll] = useState(
    offering.visible_to_enroll,
  );
  const [ownerId, setOwnerId] = useState(
    offering.owner_id === null ? "" : String(offering.owner_id),
  );

  function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    const trimmedName = name.trim();
    if (trimmedName === "") {
      submit.setError("O nome da turma é obrigatório.");
      return;
    }

    const endDateIso = dateInputToTimestamp(endDate, "end");
    if (endDateIso === null) {
      submit.setError("Informe uma data de encerramento válida.");
      return;
    }

    const payload: UpdateAdminOfferingPayload = {
      name: trimmedName,
      description: description.trim(),
      end_date: endDateIso,
      visible_to_enroll: visibleToEnroll,
    };

    const ownerIdInput = ownerId.trim();
    if (ownerIdInput !== "") {
      const parsed = Number(ownerIdInput);
      if (!Number.isInteger(parsed) || parsed <= 0) {
        submit.setError("Informe um ID de responsável válido.");
        return;
      }
      payload.owner_id = parsed;
    }

    void submit.run(async () => {
      const updated = await adminUpdateOffering(offering.id, payload);
      onSaved(updated);
    });
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Editar turma</DialogTitle>
          <DialogDescription>
            Altere os dados da turma {offering.name} (id {offering.id}).
          </DialogDescription>
        </DialogHeader>

        <form className="space-y-4" onSubmit={handleSubmit}>
          <div className="space-y-2">
            <Label htmlFor="admin-course-name">Nome</Label>
            <Input
              id="admin-course-name"
              value={name}
              disabled={submit.pending}
              onChange={(event) => {
                setName(event.target.value);
              }}
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="admin-course-description">Descrição</Label>
            <Textarea
              id="admin-course-description"
              value={description}
              disabled={submit.pending}
              className="min-h-24"
              onChange={(event) => {
                setDescription(event.target.value);
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
                value={endDate}
                disabled={submit.pending}
                onChange={(event) => {
                  setEndDate(event.target.value);
                }}
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="admin-course-owner-id">ID do responsável</Label>
              <Input
                id="admin-course-owner-id"
                type="number"
                min={1}
                value={ownerId}
                disabled={submit.pending}
                onChange={(event) => {
                  setOwnerId(event.target.value);
                }}
                placeholder="Deixe vazio para manter"
              />
              <p className="text-muted-foreground text-xs">
                Responsável atual:{" "}
                {offering.owner_id === null
                  ? "sem responsável"
                  : `${offering.owner_name || "—"} (id ${String(offering.owner_id)})`}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <Checkbox
              id="admin-course-visible"
              checked={visibleToEnroll}
              disabled={submit.pending}
              onCheckedChange={(checked) => {
                setVisibleToEnroll(checked);
              }}
            />
            <Label htmlFor="admin-course-visible">
              Matrícula aberta para os alunos
            </Label>
          </div>

          {submit.error ? (
            <Alert variant="destructive">
              <AlertDescription>{submit.error}</AlertDescription>
            </Alert>
          ) : null}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={submit.pending}
              onClick={onClose}
            >
              Cancelar
            </Button>
            <Button type="submit" disabled={submit.pending}>
              {submit.pending ? <Spinner className="size-4" /> : null}
              Salvar
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
