import { useState } from "react";

import { DateTimeField } from "@/components/app/DateTimeField";
import { MarkupEditor } from "@/components/editor/MarkupEditor";
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
import { createExercise, type Exercise } from "@/lib/api";
import { dateTimeInputToTimestamp } from "@/lib/format";

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
  const [openTime, setOpenTime] = useState("00:00");
  const [deadline, setDeadline] = useState("");
  const [deadlineTime, setDeadlineTime] = useState("23:59");
  const [deadlineEndOfDay, setDeadlineEndOfDay] = useState(true);
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
      <DialogContent className="max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>Novo exercício</DialogTitle>
          <DialogDescription>
            Defina o período de abertura, o prazo e as instruções do exercício.
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

          <div className="grid gap-6 md:grid-cols-2">
            <DateTimeField
              id="new-exercise-open-date"
              label="Abertura"
              date={openDate}
              time={openTime}
              onDateChange={setOpenDate}
              onTimeChange={setOpenTime}
              required
            />
            <DateTimeField
              id="new-exercise-deadline"
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

          <div className="space-y-2">
            <div>
              <p className="text-sm font-medium">Instruções do exercício</p>
              <p className="text-muted-foreground text-xs">
                O enunciado que os alunos veem. Aceita Markdown e será
                renderizado com essa formatação.
              </p>
            </div>
            <MarkupEditor
              format="markdown"
              value={description}
              onChange={setDescription}
              heightClassName="h-56"
              ariaLabel="Instruções do exercício em Markdown"
              placeholder="Escreva o enunciado ou as instruções do exercício…"
            />
          </div>

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
