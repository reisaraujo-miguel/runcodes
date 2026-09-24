import { useState, type SubmitEvent } from "react";

import { Button } from "@/components/ui/button";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { enroll } from "@/lib/api";

interface EnrollFormProps {
  /** Called after a successful enrollment so the page can reload its data. */
  onEnrolled: () => void;
}

/** Joins a class from the enrollment code its professor shared. */
export function EnrollForm({ onEnrolled }: EnrollFormProps) {
  const [code, setCode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const trimmedCode = code.trim();

  function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!trimmedCode || submitting) return;

    setSubmitting(true);
    setError(null);
    setSuccess(null);
    void (async () => {
      try {
        const offering = await enroll(trimmedCode);
        setCode("");
        setSuccess(`Matrícula realizada em ${offering.name}.`);
        onEnrolled();
      } catch (apiError) {
        setError(
          apiError instanceof Error
            ? apiError.message
            : "Erro ao realizar a matrícula",
        );
      } finally {
        setSubmitting(false);
      }
    })();
  }

  return (
    <form className="space-y-3" onSubmit={handleSubmit}>
      <Field>
        <FieldLabel htmlFor="enrollment-code">Código da turma</FieldLabel>
        <Input
          id="enrollment-code"
          value={code}
          autoCapitalize="characters"
          autoComplete="off"
          maxLength={4}
          placeholder="ABCD"
          spellCheck={false}
          className="font-mono tracking-widest uppercase"
          onChange={(event) => {
            setCode(event.target.value.toUpperCase());
            setError(null);
          }}
          required
        />
      </Field>

      {error && <p className="text-destructive text-sm">{error}</p>}
      {success && (
        <p className="text-sm text-emerald-700 dark:text-emerald-400">
          {success}
        </p>
      )}

      <Button type="submit" size="sm" disabled={!trimmedCode || submitting}>
        {submitting ? "Matriculando…" : "Matricular-se"}
      </Button>
    </form>
  );
}
