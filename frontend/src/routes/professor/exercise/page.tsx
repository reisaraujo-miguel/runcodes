import { useState } from "react";

import { PencilIcon, Trash2Icon } from "lucide-react";
import {
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router";

import { BackLink } from "@/components/app/BackLink";
import { ConfirmButton } from "@/components/app/ConfirmDialog";
import { Field } from "@/components/app/Field";
import { PageHeader } from "@/components/app/PageHeader";
import { Section } from "@/components/app/Section";
import { ErrorState, LoadingState } from "@/components/app/states";
import { AttachedFilesPanel } from "@/components/professor/AttachedFilesPanel";
import { CompilationFilesPanel } from "@/components/professor/CompilationFilesPanel";
import { EditExerciseForm } from "@/components/professor/EditExerciseForm";
import { ExerciseInstructions } from "@/components/professor/ExerciseInstructions";
import { ExerciseLimitAlerts } from "@/components/professor/ExerciseLimitAlerts";
import { ExerciseLimitsPanel } from "@/components/professor/ExerciseLimitsPanel";
import { TestCasesPanel } from "@/components/professor/TestCasesPanel";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { TabBar, type TabBarItem } from "@/components/ui/tab-bar";
import { useAsync } from "@/hooks/use-async";
import { deleteExercise, getExercise } from "@/lib/api";
import { errorMessage } from "@/lib/errors";
import { formatDateTime } from "@/lib/format";

type ExerciseTab = "details" | "tests" | "compilation" | "materials" | "limits";

const DEFAULT_TAB: ExerciseTab = "details";

/** Reads the selected tab from `?tab=`, falling back to the details tab. */
function parseTab(value: string | null): ExerciseTab {
  return value === "tests" ||
    value === "compilation" ||
    value === "materials" ||
    value === "limits"
    ? value
    : DEFAULT_TAB;
}

/**
 * Page to manage a single exercise. Its metadata, instructions, test cases,
 * compilation files and attached materials live in tabs instead of a stack of
 * cards. Deleting an exercise is a soft delete, so it must be confirmed before
 * navigating back to the class.
 */
export function ExercisePage() {
  const { exerciseId } = useParams();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const id = Number(exerciseId);
  const valid = Number.isInteger(id) && id > 0;
  const tab = parseTab(searchParams.get("tab"));

  const exercise = useAsync(
    () =>
      valid
        ? getExercise(id)
        : Promise.reject(new Error("Exercício inválido.")),
    `exercise-${exerciseId ?? ""}`,
    "Exercício não encontrado.",
  );

  const [editing, setEditing] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const current = exercise.data;

  function selectTab(next: ExerciseTab) {
    setSearchParams(
      (previous) => {
        const nextParams = new URLSearchParams(previous);
        nextParams.set("tab", next);
        return nextParams;
      },
      // Keep the remembered origin across tab changes; setSearchParams would
      // otherwise replace the entry with an empty state.
      { replace: true, state: location.state as unknown },
    );
  }

  if (exercise.loading && current === null) return <LoadingState />;

  if (!current) {
    return (
      <div className="space-y-6">
        <PageHeader
          title="Exercício"
          breadcrumb={<BackLink to="/professor" />}
        />
        <ErrorState
          description={
            exercise.error ?? "Não foi possível carregar este exercício."
          }
          onRetry={exercise.reload}
        />
      </div>
    );
  }

  const classLink = `/professor/class/${String(current.offering_id)}`;
  const currentId = current.id;

  async function handleDelete() {
    setDeleteError(null);
    try {
      await deleteExercise(currentId);
      void navigate(classLink);
    } catch (error) {
      setDeleteError(errorMessage(error, "Erro ao excluir o exercício."));
    }
  }

  const allowedLabels = current.allowed_file_types?.map(
    (type) => `${type.name} (${type.extension})`,
  );
  const allowedSummary =
    allowedLabels && allowedLabels.length > 0
      ? allowedLabels.join(", ")
      : current.allowed_file_type_ids &&
          current.allowed_file_type_ids.length > 0
        ? `${String(current.allowed_file_type_ids.length)} tipo(s) específico(s)`
        : "Qualquer tipo disponível";

  const tabs: TabBarItem<ExerciseTab>[] = [
    { value: "details", label: "Detalhes" },
    { value: "tests", label: "Testes" },
    { value: "compilation", label: "Compilação" },
    { value: "materials", label: "Materiais" },
    { value: "limits", label: "Limites" },
  ];

  return (
    <div className="space-y-6">
      <PageHeader
        title={current.title}
        description="Gerencie o enunciado, os casos de teste e os arquivos deste exercício."
        breadcrumb={<BackLink to={classLink} />}
        actions={
          current.removed ? (
            <Badge variant="destructive">Removido</Badge>
          ) : undefined
        }
      />

      <TabBar
        items={tabs}
        value={tab}
        onChange={selectTab}
        label="Seções do exercício"
        idPrefix="exercise"
      />

      <ExerciseLimitAlerts exerciseId={current.id} />

      <div>
        <div
          role="tabpanel"
          id="exercise-panel-details"
          aria-labelledby="exercise-tab-details"
          hidden={tab !== "details"}
          className="space-y-6"
        >
          <Section
            title="Informações do exercício"
            description="Prazos, visibilidade e tipos de arquivo aceitos."
            contentClassName="space-y-6"
            action={
              <Button
                size="sm"
                variant="outline"
                onClick={() => {
                  setEditing(true);
                }}
              >
                <PencilIcon />
                Editar informações
              </Button>
            }
          >
            <dl className="grid gap-x-6 gap-y-5 sm:grid-cols-2">
              <Field label="Abertura">
                {formatDateTime(current.open_date)}
              </Field>
              <Field label="Prazo">{formatDateTime(current.deadline)}</Field>
              <Field label="Antes da abertura">
                <Badge
                  variant={
                    current.show_before_open_date ? "success" : "secondary"
                  }
                >
                  {current.show_before_open_date ? "Visível" : "Oculto"}
                </Badge>
              </Field>
              <Field label="Tipos de arquivo permitidos">
                {allowedSummary}
              </Field>
            </dl>

            <ExerciseInstructions
              key={current.id}
              exercise={current}
              active={tab === "details"}
              onSaved={() => {
                exercise.reload();
              }}
            />

            <div className="border-destructive/30 bg-destructive/5 rounded-xl border border-dashed p-4">
              <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div className="space-y-1">
                  <p className="font-medium">Excluir exercício</p>
                  <p className="text-muted-foreground text-sm">
                    O exercício deixa de aparecer para os alunos; os casos de
                    teste, arquivos e submissões são preservados para consulta.
                  </p>
                </div>
                <ConfirmButton
                  destructive
                  variant="destructive"
                  title="Excluir exercício"
                  description={`Excluir “${current.title}”? Os alunos deixarão de vê-lo.`}
                  confirmLabel="Excluir exercício"
                  onConfirm={handleDelete}
                >
                  <Trash2Icon />
                  Excluir exercício
                </ConfirmButton>
              </div>
              {deleteError ? (
                <Alert variant="destructive" className="mt-3">
                  <AlertDescription>{deleteError}</AlertDescription>
                </Alert>
              ) : null}
            </div>
          </Section>
        </div>

        <div
          role="tabpanel"
          id="exercise-panel-tests"
          aria-labelledby="exercise-tab-tests"
          hidden={tab !== "tests"}
        >
          <TestCasesPanel exerciseId={current.id} />
        </div>

        <div
          role="tabpanel"
          id="exercise-panel-compilation"
          aria-labelledby="exercise-tab-compilation"
          hidden={tab !== "compilation"}
        >
          <CompilationFilesPanel exerciseId={current.id} />
        </div>

        <div
          role="tabpanel"
          id="exercise-panel-materials"
          aria-labelledby="exercise-tab-materials"
          hidden={tab !== "materials"}
        >
          <AttachedFilesPanel exerciseId={current.id} />
        </div>

        <div
          role="tabpanel"
          id="exercise-panel-limits"
          aria-labelledby="exercise-tab-limits"
          hidden={tab !== "limits"}
        >
          <ExerciseLimitsPanel exerciseId={current.id} />
        </div>
      </div>

      {editing ? (
        <EditExerciseForm
          exercise={current}
          onClose={() => {
            setEditing(false);
          }}
          onSaved={() => {
            setEditing(false);
            exercise.reload();
          }}
        />
      ) : null}
    </div>
  );
}
