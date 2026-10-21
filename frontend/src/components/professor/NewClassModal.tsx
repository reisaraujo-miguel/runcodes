import { useState } from "react";

import { DateTimeField } from "@/components/app/DateTimeField";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
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
import { createOffering, type Offering } from "@/lib/api";
import { dateTimeInputToTimestamp } from "@/lib/format";

/**
 * Modal to create a new class. Owns the form but not the navigation: the page
 * receives the created class and decides what to do next.
 */
export function NewClassModal({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: (offering: Offering) => void;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [endDate, setEndDate] = useState("");
  const [endTime, setEndTime] = useState("23:59");
  const [endOfDay, setEndOfDay] = useState(true);
  const { pending, error, setError, run } = useSubmit(
    "Não foi possível criar a turma.",
  );

  function handleSubmit() {
    const trimmedName = name.trim();
    if (!trimmedName) {
      setError("O nome da turma é obrigatório.");
      return;
    }
    const endDateIso = dateTimeInputToTimestamp(endDate, endTime, endOfDay);
    if (!endDateIso) {
      setError("Informe a data e o horário limite da turma.");
      return;
    }

    void run(async () => {
      const offering = await createOffering({
        name: trimmedName,
        end_date: endDateIso,
        description: description.trim(),
      });
      onCreated(offering);
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
          <DialogTitle>Nova turma</DialogTitle>
          <DialogDescription>
            Crie uma disciplina e compartilhe o código de matrícula com a turma.
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
            <Label htmlFor="new-class-name">Nome da turma</Label>
            <Input
              id="new-class-name"
              value={name}
              onChange={(event) => {
                setName(event.target.value);
              }}
              placeholder="Ex.: Algoritmos — Turma A"
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="new-class-description">Descrição</Label>
            <Textarea
              id="new-class-description"
              value={description}
              onChange={(event) => {
                setDescription(event.target.value);
              }}
              placeholder="Descrição da turma (opcional)"
            />
          </div>

          <DateTimeField
            id="new-class-end-date"
            label="Disponível até"
            date={endDate}
            time={endTime}
            onDateChange={setEndDate}
            onTimeChange={setEndTime}
            endOfDay={endOfDay}
            onEndOfDayChange={setEndOfDay}
            required
          />

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
              Criar turma
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
