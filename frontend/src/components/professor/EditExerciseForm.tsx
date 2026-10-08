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
import { updateExercise, type Exercise, type ExercisePayload } from "@/lib/api";
import { dateInputToTimestamp, formatDateInput } from "@/lib/format";

/** Compares two sets of file type ids, ignoring their order. */
function sameIds(left: number[], right: number[]): boolean {
  if (left.length !== right.length) return false;
  const sortedLeft = [...left].sort((first, second) => first - second);
  const sortedRight = [...right].sort((first, second) => first - second);
  return sortedLeft.every((id, index) => id === sortedRight[index]);
}

/** Modal form to edit an exercise's data, dates and allowed file types. */
export function EditExerciseForm({
  exercise,
  onClose,
  onSaved,
}: {
  exercise: Exercise;
  onClose: () => void;
  onSaved: (exercise: Exercise) => void;
}) {
  const [title, setTitle] = useState(exercise.title);
  const [description, setDescription] = useState(exercise.description);
  const [openDate, setOpenDate] = useState(() =>
    formatDateInput(exercise.open_date),
  );
  const [deadline, setDeadline] = useState(() =>
    formatDateInput(exercise.deadline),
  );
  const [showBeforeOpenDate, setShowBeforeOpenDate] = useState(
    exercise.show_before_open_date,
  );
  const [selectedTypes, setSelectedTypes] = useState<number[]>(
    exercise.allowed_file_type_ids ?? [],
  );
  const { pending, error, setError, run } = useSubmit(
    "Não foi possível salvar o exercício.",
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

    // Only the fields the professor actually changed are sent.
    const payload: Partial<ExercisePayload> = {};
    if (trimmedTitle !== exercise.title) payload.title = trimmedTitle;

    const trimmedDescription = description.trim();
    if (trimmedDescription !== exercise.description) {
      payload.description = trimmedDescription;
    }
    if (openDate !== formatDateInput(exercise.open_date)) {
      payload.open_date = openDateIso;
    }
    if (deadline !== formatDateInput(exercise.deadline)) {
      payload.deadline = deadlineIso;
    }
    if (showBeforeOpenDate !== exercise.show_before_open_date) {
      payload.show_before_open_date = showBeforeOpenDate;
    }
    if (!sameIds(selectedTypes, exercise.allowed_file_type_ids ?? [])) {
      payload.allowed_file_type_ids = selectedTypes;
    }

    if (Object.keys(payload).length === 0) {
      onSaved(exercise);
      return;
    }

    void run(async () => {
      onSaved(await updateExercise(exercise.id, payload));
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
          <DialogTitle>Editar exercício</DialogTitle>
          <DialogDescription>
            Altere o enunciado, o período de entrega e os tipos aceitos.
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
            <Label htmlFor="edit-exercise-title">Título</Label>
            <Input
              id="edit-exercise-title"
              value={title}
              onChange={(event) => {
                setTitle(event.target.value);
              }}
              placeholder="Ex.: Lista 1 — Introdução"
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="edit-exercise-description">Descrição</Label>
            <Textarea
              id="edit-exercise-description"
              value={description}
              onChange={(event) => {
                setDescription(event.target.value);
              }}
              placeholder="Enunciado do exercício (opcional)"
            />
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="edit-exercise-open-date">Abertura</Label>
              <Input
                id="edit-exercise-open-date"
                type="date"
                value={openDate}
                onChange={(event) => {
                  setOpenDate(event.target.value);
                }}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="edit-exercise-deadline">Prazo</Label>
              <Input
                id="edit-exercise-deadline"
                type="date"
                value={deadline}
                onChange={(event) => {
                  setDeadline(event.target.value);
                }}
                required
              />
            </div>
          </div>

          <Label className="font-normal" htmlFor="edit-exercise-show-before">
            <Checkbox
              id="edit-exercise-show-before"
              checked={showBeforeOpenDate}
              onCheckedChange={(checked) => {
                setShowBeforeOpenDate(checked);
              }}
            />
            Mostrar o exercício antes da data de abertura
          </Label>

          <AllowedFileTypesField
            idPrefix="edit-exercise"
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
              Salvar alterações
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
