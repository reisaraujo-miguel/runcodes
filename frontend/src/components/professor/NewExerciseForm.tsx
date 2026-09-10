import { useEffect, useState, type SubmitEvent } from "react";

import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  createExercise,
  getAllowedFileTypes,
  type AllowedFileType,
  type Exercise,
} from "@/lib/api";
import { dateInputToTimestamp } from "@/lib/format";

interface NewExerciseFormProps {
  offeringId: number;
  onCreated: (exercise: Exercise) => void;
  onCancel: () => void;
}

/** Inline form that creates an exercise inside an offering. */
export function NewExerciseForm({
  offeringId,
  onCreated,
  onCancel,
}: NewExerciseFormProps) {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [deadline, setDeadline] = useState("");
  const [openDate, setOpenDate] = useState("");
  const [selectedTypes, setSelectedTypes] = useState<number[]>([]);
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
        // The allowed-type list is optional for creating an exercise.
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
    const deadlineIso = dateInputToTimestamp(deadline, "end");
    const openDateIso = dateInputToTimestamp(openDate, "start");
    if (!title.trim()) {
      setError("O título do exercício é obrigatório");
      return;
    }
    if (!deadlineIso || !openDateIso) {
      setError("Informe as datas de abertura e de prazo");
      return;
    }

    setSubmitting(true);
    setError(null);
    void (async () => {
      try {
        const exercise = await createExercise(offeringId, {
          title: title.trim(),
          description: description.trim(),
          deadline: deadlineIso,
          open_date: openDateIso,
          ...(selectedTypes.length > 0
            ? { allowed_file_type_ids: selectedTypes }
            : {}),
        });
        onCreated(exercise);
      } catch (submitError) {
        setError(
          submitError instanceof Error
            ? submitError.message
            : "Erro ao criar o exercício",
        );
      } finally {
        setSubmitting(false);
      }
    })();
  }

  const availableTypes = allowedTypes.filter((type) => type.is_available);

  return (
    <Card size="sm">
      <CardContent>
        <form className="space-y-4" onSubmit={handleSubmit}>
          <div className="space-y-2">
            <Label htmlFor="exercise-title">Título</Label>
            <Input
              id="exercise-title"
              value={title}
              onChange={(event) => {
                setTitle(event.target.value);
              }}
              placeholder="Ex.: Lista 1 — Introdução"
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="exercise-description">Descrição</Label>
            <Textarea
              id="exercise-description"
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
              <Label htmlFor="exercise-open-date">Abertura</Label>
              <Input
                id="exercise-open-date"
                type="date"
                value={openDate}
                onChange={(event) => {
                  setOpenDate(event.target.value);
                }}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="exercise-deadline">Prazo</Label>
              <Input
                id="exercise-deadline"
                type="date"
                value={deadline}
                onChange={(event) => {
                  setDeadline(event.target.value);
                }}
                required
              />
            </div>
          </div>

          {availableTypes.length > 0 && (
            <fieldset className="space-y-2">
              <legend className="text-sm font-medium">
                Tipos de arquivo permitidos
              </legend>
              <div className="grid gap-2 sm:grid-cols-2">
                {availableTypes.map((type) => (
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
              {submitting ? "Criando…" : "Criar exercício"}
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
