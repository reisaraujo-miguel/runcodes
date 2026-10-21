import { useState } from "react";

import { DateTimeField } from "@/components/app/DateTimeField";
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
import { useSubmit } from "@/hooks/use-submit";
import { updateExercise, type Exercise, type ExercisePayload } from "@/lib/api";
import {
  dateTimeInputToTimestamp,
  formatDateInput,
  formatTimeInput,
  isEndOfDay,
} from "@/lib/format";

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
  const [openDate, setOpenDate] = useState(() =>
    formatDateInput(exercise.open_date),
  );
  const [openTime, setOpenTime] = useState(
    () => formatTimeInput(exercise.open_date) || "00:00",
  );
  const [deadline, setDeadline] = useState(() =>
    formatDateInput(exercise.deadline),
  );
  const [deadlineTime, setDeadlineTime] = useState(
    () => formatTimeInput(exercise.deadline) || "23:59",
  );
  const [deadlineEndOfDay, setDeadlineEndOfDay] = useState(() =>
    isEndOfDay(exercise.deadline),
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
    const openDateIso = dateTimeInputToTimestamp(openDate, openTime);
    const deadlineIso = dateTimeInputToTimestamp(
      deadline,
      deadlineTime,
      deadlineEndOfDay,
    );
    if (!openDateIso || !deadlineIso) {
      setError("Informe a data e o horário de abertura e de prazo.");
      return;
    }
    if (new Date(deadlineIso) < new Date(openDateIso)) {
      setError("O prazo não pode ser anterior à abertura.");
      return;
    }

    // Only the fields the professor actually changed are sent.
    const payload: Partial<ExercisePayload> = {};
    if (trimmedTitle !== exercise.title) payload.title = trimmedTitle;

    const openChanged =
      openDate !== formatDateInput(exercise.open_date) ||
      openTime !== formatTimeInput(exercise.open_date);
    if (openChanged) payload.open_date = openDateIso;

    const deadlineChanged =
      deadline !== formatDateInput(exercise.deadline) ||
      deadlineTime !== formatTimeInput(exercise.deadline) ||
      deadlineEndOfDay !== isEndOfDay(exercise.deadline);
    if (deadlineChanged) payload.deadline = deadlineIso;

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
      <DialogContent className="max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>Editar exercício</DialogTitle>
          <DialogDescription>
            Altere o título, o período de entrega e os tipos aceitos. As
            instruções são editadas na página do exercício.
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

          <div className="grid gap-6 md:grid-cols-2">
            <DateTimeField
              id="edit-exercise-open-date"
              label="Abertura"
              date={openDate}
              time={openTime}
              onDateChange={setOpenDate}
              onTimeChange={setOpenTime}
              required
            />
            <DateTimeField
              id="edit-exercise-deadline"
              label="Prazo"
              date={deadline}
              time={deadlineTime}
              onDateChange={setDeadline}
              onTimeChange={setDeadlineTime}
              endOfDay={deadlineEndOfDay}
              onEndOfDayChange={setDeadlineEndOfDay}
              className="md:border-border md:border-l md:pl-6"
              required
            />
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
