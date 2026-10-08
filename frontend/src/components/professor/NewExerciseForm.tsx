import { useState } from "react";

import { AllowedFileTypesField } from "@/components/professor/AllowedFileTypesField";
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
import { createExercise, type Exercise } from "@/lib/api";
import { dateInputToTimestamp } from "@/lib/format";

/** Modal form to create an exercise inside a class. */
export function NewExerciseForm({
  offeringId,
  onClose,
  onCreated,
}: {
  offeringId: number;
  onClose: () => void;
  onCreated: (exercise: Exercise) => void;
}) {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [openDate, setOpenDate] = useState("");
  const [deadline, setDeadline] = useState("");
  const [showBeforeOpenDate, setShowBeforeOpenDate] = useState(false);
  const [selectedTypes, setSelectedTypes] = useState<number[]>([]);
  const { pending, error, setError, run } = useSubmit(
    "Não foi possível criar o exercício.",
  );

  function handleSubmit() {
    const trimmedTitle = title.trim();
    if (!trimmedTitle) {
      setError("O título do exercício é obrigatório.");
      return;
    }
    const openDateIso = dateInputToTimestamp(openDate, "start");
    const deadlineIso = dateInputToTimestamp(deadline, "end");
    if (!openDateIso || !deadlineIso) {
      setError("Informe as datas de abertura e de prazo.");
      return;
    }

    void run(async () => {
      const exercise = await createExercise(offeringId, {
        title: trimmedTitle,
        description: description.trim(),
        open_date: openDateIso,
        deadline: deadlineIso,
        show_before_open_date: showBeforeOpenDate,
        ...(selectedTypes.length > 0
          ? { allowed_file_type_ids: selectedTypes }
          : {}),
      });
      onCreated(exercise);
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
          <DialogTitle>Novo exercício</DialogTitle>
          <DialogDescription>
            Defina o período de abertura e o prazo de entrega.
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
            <Label htmlFor="new-exercise-title">Título</Label>
            <Input
              id="new-exercise-title"
              value={title}
              onChange={(event) => {
                setTitle(event.target.value);
              }}
              placeholder="Ex.: Lista 1 — Introdução"
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="new-exercise-description">Descrição</Label>
            <Textarea
              id="new-exercise-description"
              value={description}
              onChange={(event) => {
                setDescription(event.target.value);
              }}
              placeholder="Enunciado do exercício (opcional)"
            />
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="new-exercise-open-date">Abertura</Label>
              <Input
                id="new-exercise-open-date"
                type="date"
                value={openDate}
                onChange={(event) => {
                  setOpenDate(event.target.value);
                }}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="new-exercise-deadline">Prazo</Label>
              <Input
                id="new-exercise-deadline"
                type="date"
                value={deadline}
                onChange={(event) => {
                  setDeadline(event.target.value);
                }}
                required
              />
            </div>
          </div>

          <Label className="font-normal" htmlFor="new-exercise-show-before">
            <Checkbox
              id="new-exercise-show-before"
              checked={showBeforeOpenDate}
              onCheckedChange={(checked) => {
                setShowBeforeOpenDate(checked);
              }}
            />
            Mostrar o exercício antes da data de abertura
          </Label>

          <AllowedFileTypesField
            idPrefix="new-exercise"
            selected={selectedTypes}
            onChange={setSelectedTypes}
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
              Criar exercício
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
