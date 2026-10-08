import { useState } from "react";

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
import { updateOffering, type Offering } from "@/lib/api";
import { dateInputToTimestamp, formatDateInput } from "@/lib/format";

/** Modal form to edit a class the caller owns. */
export function EditClassForm({
  offering,
  onClose,
  onSaved,
}: {
  offering: Offering;
  onClose: () => void;
  onSaved: (offering: Offering) => void;
}) {
  const [name, setName] = useState(offering.name);
  const [description, setDescription] = useState(offering.description);
  const [endDate, setEndDate] = useState(() =>
    formatDateInput(offering.end_date),
  );
  const [visibleToEnroll, setVisibleToEnroll] = useState(
    offering.visible_to_enroll,
  );
  const { pending, error, setError, run } = useSubmit(
    "Não foi possível salvar a turma.",
  );

  function handleSubmit() {
    const trimmedName = name.trim();
    if (!trimmedName) {
      setError("O nome da turma é obrigatório.");
      return;
    }
    const endDateIso = dateInputToTimestamp(endDate, "end");
    if (!endDateIso) {
      setError("Informe a data limite da turma.");
      return;
    }

    void run(async () => {
      const updated = await updateOffering(offering.id, {
        name: trimmedName,
        description: description.trim(),
        end_date: endDateIso,
        visible_to_enroll: visibleToEnroll,
      });
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
      <DialogContent className="max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Editar turma</DialogTitle>
          <DialogDescription>
            Atualize os dados da turma. O código de matrícula não muda.
          </DialogDescription>
        </DialogHeader>

        <form
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            handleSubmit();
          }}
        >
          <div className="space-y-2">
            <Label htmlFor="edit-class-name">Nome da turma</Label>
            <Input
              id="edit-class-name"
              value={name}
              onChange={(event) => {
                setName(event.target.value);
              }}
              placeholder="Ex.: Algoritmos — Turma A"
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="edit-class-description">Descrição</Label>
            <Textarea
              id="edit-class-description"
              value={description}
              onChange={(event) => {
                setDescription(event.target.value);
              }}
              placeholder="Descrição da turma (opcional)"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="edit-class-end-date">Disponível até</Label>
            <Input
              id="edit-class-end-date"
              type="date"
              value={endDate}
              onChange={(event) => {
                setEndDate(event.target.value);
              }}
              required
            />
          </div>

          <Label className="font-normal" htmlFor="edit-class-visible">
            <Checkbox
              id="edit-class-visible"
              checked={visibleToEnroll}
              onCheckedChange={(checked) => {
                setVisibleToEnroll(checked);
              }}
            />
            Permitir que novos alunos se matriculem com o código
          </Label>

          {error ? (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={pending}
              onClick={onClose}
            >
              Cancelar
            </Button>
            <Button type="submit" disabled={pending}>
              {pending ? <Spinner className="size-4" /> : null}
              Salvar alterações
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
