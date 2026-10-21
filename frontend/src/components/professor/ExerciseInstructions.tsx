import { FileTextIcon, SaveIcon } from "lucide-react";
import { useState } from "react";

import { MarkupEditor } from "@/components/editor/MarkupEditor";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { useSubmit } from "@/hooks/use-submit";
import { updateExercise, type Exercise } from "@/lib/api";

/**
 * The exercise brief / instructions. They are edited inline on the exercise
 * page (rather than in the metadata dialog) because they are the part of the
 * exercise students read first, and they are written in Markdown.
 */
export function ExerciseInstructions({
  exercise,
  onSaved,
  active = true,
}: {
  exercise: Exercise;
  onSaved: (exercise: Exercise) => void;
  /** Whether the surrounding details tab is the visible one. */
  active?: boolean;
}) {
  const [draft, setDraft] = useState(exercise.description);
  const [savedDescription, setSavedDescription] = useState(
    exercise.description,
  );
  const submit = useSubmit("Não foi possível salvar as instruções.");

  const trimmedDraft = draft.trim();
  const dirty = trimmedDraft !== savedDescription;

  function handleChange(value: string) {
    setDraft(value);
    if (submit.error) submit.setError(null);
    if (submit.success) submit.setSuccess(null);
  }

  function handleDiscard() {
    setDraft(savedDescription);
    submit.setError(null);
    submit.setSuccess(null);
  }

  function handleSave() {
    if (!dirty) return;

    void submit.run(async () => {
      const updated = await updateExercise(exercise.id, {
        description: trimmedDraft,
      });
      setSavedDescription(updated.description);
      setDraft(updated.description);
      onSaved(updated);
    }, "Instruções salvas.");
  }

  return (
    <section
      className="space-y-3 border-t pt-6"
      aria-labelledby="exercise-instructions-heading"
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2
            id="exercise-instructions-heading"
            className="flex items-center gap-2 text-base font-medium"
          >
            <FileTextIcon className="size-4 text-primary" aria-hidden />
            Instruções do exercício
          </h2>
          <p className="text-muted-foreground mt-1 text-sm">
            É o enunciado que o aluno vê ao abrir o exercício. Aceita Markdown —
            títulos, listas, ênfases, links e blocos de código — e será
            renderizado com essa formatação.
          </p>
        </div>

        <div className="flex shrink-0 flex-wrap items-center gap-2">
          {dirty ? (
            <Badge variant="warning">Alterações não salvas</Badge>
          ) : null}
          {dirty ? (
            <Button
              type="button"
              size="sm"
              variant="ghost"
              disabled={submit.pending}
              onClick={handleDiscard}
            >
              Descartar
            </Button>
          ) : null}
          <Button
            type="button"
            size="sm"
            disabled={!dirty || submit.pending}
            onClick={handleSave}
          >
            {submit.pending ? <Spinner className="size-4" /> : <SaveIcon />}
            Salvar instruções
          </Button>
        </div>
      </div>

      <MarkupEditor
        format="markdown"
        value={draft}
        onChange={handleChange}
        heightClassName="h-80"
        enabled={active}
        placeholder="Escreva o enunciado ou as instruções do exercício…"
      />

      {submit.error ? (
        <Alert variant="destructive">
          <AlertDescription>{submit.error}</AlertDescription>
        </Alert>
      ) : null}
      {submit.success ? (
        <Alert variant="success">
          <AlertDescription>{submit.success}</AlertDescription>
        </Alert>
      ) : null}
    </section>
  );
}
