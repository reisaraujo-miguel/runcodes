import { useState } from "react";

import { Section } from "@/components/app/Section";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { useAsync } from "@/hooks/use-async";
import { useSubmit } from "@/hooks/use-submit";
import {
  getExerciseLanguageLimits,
  replaceExerciseLanguageLimits,
  type LanguageLimits,
} from "@/lib/api";

import {
  LanguageLimitsTable,
  type LanguageLimitRow,
} from "./LanguageLimitsTable";
import {
  bytesToMiBInput,
  limitToInput,
  parseLimit,
  parseMiBInput,
} from "./test-case-draft";

/** Seeds the editor with the exercise's own overrides; empty means inherit. */
function rowsFrom(
  limits: readonly LanguageLimits[],
): Record<number, LanguageLimitRow> {
  const rows: Record<number, LanguageLimitRow> = {};
  for (const language of limits) {
    const overrides = language.overrides;
    if (
      overrides.cpu_time_limit_seconds === null &&
      overrides.mem_usage_limit_bytes === null &&
      overrides.stack_limit_bytes === null &&
      overrides.file_size_limit_bytes === null
    ) {
      continue;
    }
    rows[language.allowed_file_type_id] = {
      cpu: limitToInput(overrides.cpu_time_limit_seconds),
      mem: bytesToMiBInput(overrides.mem_usage_limit_bytes),
      stack: bytesToMiBInput(overrides.stack_limit_bytes),
      fileSize: bytesToMiBInput(overrides.file_size_limit_bytes),
    };
  }
  return rows;
}

/**
 * The exercise's per-language limit defaults. They apply to every test case that
 * does not override them, which is the middle layer of the resolution chain:
 * platform, then exercise, then test case.
 */
export function ExerciseLimitsPanel({ exerciseId }: { exerciseId: number }) {
  const limits = useAsync(
    () => getExerciseLanguageLimits(exerciseId),
    `exercise-limits-${String(exerciseId)}`,
    "Não foi possível carregar os limites desta lista de exercícios.",
  );

  const data = limits.data;
  // Remounting the editor when the stored overrides change is what seeds its
  // inputs; deriving state in an effect would write state on every render pass.
  const version = data
    ? JSON.stringify(data.map((language) => language.overrides))
    : "";

  return (
    <Section
      title="Limites por linguagem"
      description="Estes valores valem para todos os casos de teste desta lista de exercícios. Um caso pode substituir qualquer um deles."
    >
      <div className="space-y-4">
        {limits.error ? (
          <Alert variant="destructive">
            <AlertDescription>{limits.error}</AlertDescription>
          </Alert>
        ) : null}

        {data ? (
          <ExerciseLimitsEditor
            key={version}
            exerciseId={exerciseId}
            limits={data}
            onSaved={limits.reload}
          />
        ) : (
          <p className="text-sm text-muted-foreground">Carregando…</p>
        )}
      </div>
    </Section>
  );
}

function ExerciseLimitsEditor({
  exerciseId,
  limits,
  onSaved,
}: {
  exerciseId: number;
  limits: readonly LanguageLimits[];
  onSaved: () => void;
}) {
  const [rows, setRows] = useState<Record<number, LanguageLimitRow>>(() =>
    rowsFrom(limits),
  );
  const { pending, error, success, run } = useSubmit(
    "Não foi possível salvar os limites.",
  );

  function handleSave() {
    void run(async () => {
      await replaceExerciseLanguageLimits(
        exerciseId,
        Object.entries(rows).map(([id, row]) => ({
          allowed_file_type_id: Number(id),
          cpu_time_limit_seconds: parseLimit(row.cpu) ?? null,
          mem_usage_limit_bytes: parseMiBInput(row.mem) ?? null,
          stack_limit_bytes: parseMiBInput(row.stack) ?? null,
          file_size_limit_bytes: parseMiBInput(row.fileSize) ?? null,
        })),
      );
      onSaved();
    }, "Limites do exercício salvos.");
  }

  return (
    <div className="space-y-4">
      <LanguageLimitsTable
        languages={limits}
        values={rows}
        idPrefix="exercise-limit"
        onChange={(languageId, field, value) => {
          setRows((current) => ({
            ...current,
            [languageId]: {
              cpu: current[languageId]?.cpu ?? "",
              mem: current[languageId]?.mem ?? "",
              stack: current[languageId]?.stack ?? "",
              fileSize: current[languageId]?.fileSize ?? "",
              [field]: value,
            },
          }));
        }}
      />

      {error ? (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}
      {success ? (
        <Alert variant="success">
          <AlertDescription>{success}</AlertDescription>
        </Alert>
      ) : null}

      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-xs text-muted-foreground">
          Campos vazios usam o padrão da plataforma, mostrado como placeholder.
        </p>
        <Button size="sm" disabled={pending} onClick={handleSave}>
          {pending ? <Spinner /> : null}
          Salvar limites
        </Button>
      </div>
    </div>
  );
}
