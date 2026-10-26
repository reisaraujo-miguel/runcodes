import { useState, type SubmitEvent } from "react";

import { DateTimeField } from "@/components/app/DateTimeField";
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
import {
  dateTimeInputToTimestamp,
  formatDateInput,
  formatTimeInput,
  isEndOfDay,
} from "@/lib/format";

/**
 * Edits a class from the admin panel: its name, description, deadline and
 * enrollment visibility, and transfers ownership by email or institutional id.
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
  const [endTime, setEndTime] = useState(
    () => formatTimeInput(offering.end_date) || "23:59",
  );
  const [endOfDay, setEndOfDay] = useState(() => isEndOfDay(offering.end_date));
  const [visibleToEnroll, setVisibleToEnroll] = useState(
    offering.visible_to_enroll,
  );
  const [ownerReference, setOwnerReference] = useState("");

  function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    const trimmedName = name.trim();
    if (trimmedName === "") {
      submit.setError("O nome da turma é obrigatório.");
      return;
    }

    const endDateIso = dateTimeInputToTimestamp(endDate, endTime, endOfDay);
    if (endDateIso === null) {
      submit.setError("Informe uma data e um horário de encerramento válidos.");
      return;
    }

    const payload: UpdateAdminOfferingPayload = {
      name: trimmedName,
      description: description.trim(),
      end_date: endDateIso,
      visible_to_enroll: visibleToEnroll,
    };

    const reference = ownerReference.trim();
    if (reference !== "") {
      // An address contains "@"; anything else is the institutional id (for
      // example, a university registration number). The internal user id is
      // deliberately not accepted here.
      if (reference.includes("@")) {
        payload.owner_email = reference;
      } else {
        payload.owner_org_id = reference;
      }
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
            <DateTimeField
              id="admin-course-end-date"
              label="Data de encerramento"
              date={endDate}
              time={endTime}
              onDateChange={setEndDate}
              onTimeChange={setEndTime}
              endOfDay={endOfDay}
              onEndOfDayChange={setEndOfDay}
              disabled={submit.pending}
              required
            />

            <div className="space-y-2">
              <Label htmlFor="admin-course-owner">Novo responsável</Label>
              <Input
                id="admin-course-owner"
                value={ownerReference}
                disabled={submit.pending}
                onChange={(event) => {
                  setOwnerReference(event.target.value);
                }}
                placeholder="Email ou identificação institucional"
                autoComplete="off"
              />
              <p className="text-muted-foreground text-xs">
                Deixe em branco para manter. Informe o email ou a identificação
                institucional (como a matrícula) de um professor ou
                administrador; o ID interno da conta não é usado.
              </p>
              <p className="text-muted-foreground text-xs">
                Responsável atual:{" "}
                {offering.owner_id === null
                  ? "sem responsável"
                  : `${offering.owner_name || offering.owner_email} (${offering.owner_email})`}
                {offering.owner_org_id
                  ? ` · identificação ${offering.owner_org_id}`
                  : ""}
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
