import { useState, type SubmitEvent } from "react";

import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { updateOffering, type Offering } from "@/lib/api";
import { dateInputToTimestamp } from "@/lib/format";

interface EditClassFormProps {
  offering: Offering;
  onSaved: (offering: Offering) => void;
  onCancel: () => void;
}

/**
 * Converts an API timestamp into the `YYYY-MM-DD` value a date input expects.
 * Local getters keep the calendar day the professor picked: the API stores the
 * end of that day in the same timezone, so the round trip is stable.
 */
function toDateInputValue(timestamp: string): string {
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) return "";
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${String(date.getFullYear())}-${month}-${day}`;
}

/** Inline form that edits the details of a class the caller owns. */
export function EditClassForm({
  offering,
  onSaved,
  onCancel,
}: EditClassFormProps) {
  const [name, setName] = useState(offering.name);
  const [description, setDescription] = useState(offering.description);
  const [endDate, setEndDate] = useState(() =>
    toDateInputValue(offering.end_date),
  );
  const [visibleToEnroll, setVisibleToEnroll] = useState(
    offering.visible_to_enroll,
  );
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmedName = name.trim();
    if (!trimmedName) {
      setError("O nome da turma é obrigatório");
      return;
    }
    const endDateIso = dateInputToTimestamp(endDate, "end");
    if (!endDateIso) {
      setError("Informe a data limite da turma");
      return;
    }

    setSubmitting(true);
    setError(null);
    void (async () => {
      try {
        const updated = await updateOffering(offering.id, {
          name: trimmedName,
          description: description.trim(),
          end_date: endDateIso,
          visible_to_enroll: visibleToEnroll,
        });
        onSaved(updated);
      } catch (submitError) {
        setError(
          submitError instanceof Error
            ? submitError.message
            : "Erro ao salvar a turma",
        );
      } finally {
        setSubmitting(false);
      }
    })();
  }

  return (
    <Card size="sm">
      <CardContent>
        <form className="space-y-4" onSubmit={handleSubmit}>
          <div className="space-y-2">
            <Label htmlFor="class-name">Nome da turma</Label>
            <Input
              id="class-name"
              value={name}
              onChange={(event) => {
                setName(event.target.value);
              }}
              placeholder="Ex.: Algoritmos — Turma A"
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="class-description">Descrição</Label>
            <Textarea
              id="class-description"
              value={description}
              onChange={(event) => {
                setDescription(event.target.value);
              }}
              placeholder="Descrição da turma (opcional)"
              className="font-sans"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="class-end-date">Disponível até</Label>
            <Input
              id="class-end-date"
              type="date"
              value={endDate}
              onChange={(event) => {
                setEndDate(event.target.value);
              }}
              required
            />
          </div>

          <label className="flex items-start gap-2 text-sm">
            <input
              type="checkbox"
              className="accent-primary mt-0.5 size-4"
              checked={visibleToEnroll}
              onChange={(event) => {
                setVisibleToEnroll(event.target.checked);
              }}
            />
            Permitir que novos alunos se matriculem com o código
          </label>

          {error && <p className="text-destructive text-sm">{error}</p>}

          <div className="flex gap-2">
            <Button type="submit" disabled={submitting}>
              {submitting ? "Salvando…" : "Salvar alterações"}
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={submitting}
              onClick={onCancel}
            >
              Cancelar
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
