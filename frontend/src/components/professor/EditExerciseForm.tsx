import { useEffect, useState, type SubmitEvent } from "react";

import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  getAllowedFileTypes,
  updateExercise,
  type AllowedFileType,
  type Exercise,
  type ExercisePayload,
} from "@/lib/api";
import { dateInputToTimestamp } from "@/lib/format";

/** Converts an RFC3339 timestamp into the YYYY-MM-DD value a date input expects. */
function toDateInputValue(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${String(date.getFullYear())}-${month}-${day}`;
}

/** Compares two sets of file type ids, ignoring their order. */
function sameIds(left: number[], right: number[]): boolean {
  if (left.length !== right.length) return false;
  const sortedLeft = [...left].sort((first, second) => first - second);
  const sortedRight = [...right].sort((first, second) => first - second);
  return sortedLeft.every((id, index) => id === sortedRight[index]);
}

interface EditExerciseFormProps {
  exercise: Exercise;
  onSaved: (exercise: Exercise) => void;
  onCancel: () => void;
}

/** Inline form to edit an exercise's data, dates and allowed file types. */
export function EditExerciseForm({
  exercise,
  onSaved,
  onCancel,
}: EditExerciseFormProps) {
  const [title, setTitle] = useState(exercise.title);
  const [description, setDescription] = useState(exercise.description);
  const [openDate, setOpenDate] = useState(() =>
    toDateInputValue(exercise.open_date),
  );
  const [deadline, setDeadline] = useState(() =>
    toDateInputValue(exercise.deadline),
  );
  const [showBeforeOpenDate, setShowBeforeOpenDate] = useState(
    exercise.show_before_open_date,
  );
  const [selectedTypes, setSelectedTypes] = useState<number[]>(
    exercise.allowed_file_type_ids ?? [],
  );
  const [allowedTypes, setAllowedTypes] = useState<AllowedFileType[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void getAllowedFileTypes()
      .then((types) => {
        if (!cancelled) setAllowedTypes(types);
      })
      .catch(() => {
        // The allowed-type list is optional when editing an exercise.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  function toggleType(id: number) {
    setSelectedTypes((previous) =>
      previous.includes(id)
        ? previous.filter((value) => value !== id)
        : [...previous, id],
    );
  }

  function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    const trimmedTitle = title.trim();
    if (!trimmedTitle) {
      setError("O título do exercício é obrigatório");
      return;
    }
    const openDateIso = dateInputToTimestamp(openDate, "start");
    const deadlineIso = dateInputToTimestamp(deadline, "end");
    if (!openDateIso || !deadlineIso) {
      setError("Informe as datas de abertura e de prazo");
      return;
    }

    // Only the fields the professor actually changed are sent.
    const payload: Partial<ExercisePayload> = {};
    if (trimmedTitle !== exercise.title) {
      payload.title = trimmedTitle;
    }
    const trimmedDescription = description.trim();
    if (trimmedDescription !== exercise.description) {
      payload.description = trimmedDescription;
    }
    if (openDate !== toDateInputValue(exercise.open_date)) {
      payload.open_date = openDateIso;
    }
    if (deadline !== toDateInputValue(exercise.deadline)) {
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

    setSubmitting(true);
    setError(null);
    void (async () => {
      try {
        const updated = await updateExercise(exercise.id, payload);
        onSaved(updated);
      } catch (submitError) {
        setError(
          submitError instanceof Error
            ? submitError.message
            : "Erro ao salvar o exercício",
        );
      } finally {
        setSubmitting(false);
      }
    })();
  }

  // A currently selected type stays listed even when it is no longer available,
  // so editing another field never drops it silently.
  const selectableTypes = allowedTypes.filter(
    (type) => type.is_available || selectedTypes.includes(type.id),
  );

  return (
    <Card size="sm">
      <CardContent>
        <form className="space-y-4" onSubmit={handleSubmit}>
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
              className="font-sans"
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

          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              className="accent-primary size-4"
              checked={showBeforeOpenDate}
              onChange={(event) => {
                setShowBeforeOpenDate(event.target.checked);
              }}
            />
            Mostrar antes da data de abertura
          </label>

          {selectableTypes.length > 0 && (
            <fieldset className="space-y-2">
              <legend className="text-sm font-medium">
                Tipos de arquivo permitidos
              </legend>
              <div className="grid gap-2 sm:grid-cols-2">
                {selectableTypes.map((type) => (
                  <label
                    key={type.id}
                    className="flex items-center gap-2 text-sm"
                  >
                    <input
                      type="checkbox"
                      className="accent-primary size-4"
                      checked={selectedTypes.includes(type.id)}
                      onChange={() => {
                        toggleType(type.id);
                      }}
                    />
                    {type.name} ({type.extension})
                  </label>
                ))}
              </div>
            </fieldset>
          )}

          {error && <p className="text-destructive text-sm">{error}</p>}

          <div className="flex gap-2">
            <Button type="submit" disabled={submitting}>
              {submitting ? "Salvando…" : "Salvar"}
            </Button>
            <Button type="button" variant="outline" onClick={onCancel}>
              Cancelar
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
