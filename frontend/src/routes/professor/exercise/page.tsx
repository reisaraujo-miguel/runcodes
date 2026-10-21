import { useState } from "react";

import { PencilIcon, Trash2Icon } from "lucide-react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";

import { ConfirmButton } from "@/components/app/ConfirmDialog";
import { PageHeader } from "@/components/app/PageHeader";
import { ErrorState, LoadingState } from "@/components/app/states";
import { AttachedFilesCard } from "@/components/professor/AttachedFilesCard";
import { CompilationFilesCard } from "@/components/professor/CompilationFilesCard";
import { EditExerciseForm } from "@/components/professor/EditExerciseForm";
import { ExerciseInstructions } from "@/components/professor/ExerciseInstructions";
import { TestCasesCard } from "@/components/professor/TestCasesCard";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { TabBar, type TabBarItem } from "@/components/ui/tab-bar";
import { useAsync } from "@/hooks/use-async";
import { deleteExercise, getExercise } from "@/lib/api";
import { errorMessage } from "@/lib/errors";
import { formatDateTime } from "@/lib/format";

type ExerciseTab = "details" | "tests" | "compilation" | "materials";

const DEFAULT_TAB: ExerciseTab = "details";

/** Reads the selected tab from `?tab=`, falling back to the details tab. */
function parseTab(value: string | null): ExerciseTab {
  return value === "tests" || value === "compilation" || value === "materials"
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
      { replace: true },
    );
  }

  if (exercise.loading && current === null) return <LoadingState />;

  if (!current) {
    return (
      <div className="space-y-6">
        <PageHeader
          title="Exercício"
          breadcrumb={
            <Link
              to="/professor"
              className="text-muted-foreground text-sm hover:underline"
            >
              ← Gerenciar turmas
            </Link>
          }
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
  ];

  return (
    <div className="space-y-6">
      <PageHeader
        title={current.title}
        description="Gerencie o enunciado, os casos de teste e os arquivos deste exercício."
        breadcrumb={
          <Link
            to={classLink}
            className="text-muted-foreground text-sm hover:underline"
          >
            ← Voltar para a turma
          </Link>
        }
        actions={
          current.removed ? (
            <Badge variant="destructive">Removido</Badge>
          ) : undefined
        }
      />

      <Card className="gap-0 py-0">
        <TabBar
          items={tabs}
          value={tab}
          onChange={selectTab}
          label="Seções do exercício"
          idPrefix="exercise"
          className="px-2"
        />

        <div className="p-4 sm:p-5">
          <div
            role="tabpanel"
            id="exercise-panel-details"
            aria-labelledby="exercise-tab-details"
            hidden={tab !== "details"}
            className="space-y-6"
          >
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <h2 className="text-base font-medium">
                  Informações do exercício
                </h2>
                <p className="text-muted-foreground text-sm">
                  Prazos, visibilidade e tipos de arquivo aceitos.
                </p>
              </div>
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
            </div>

            <dl className="grid gap-4 sm:grid-cols-2">
              <div>
                <dt className="text-muted-foreground text-sm">Abertura</dt>
                <dd className="mt-1">{formatDateTime(current.open_date)}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground text-sm">Prazo</dt>
                <dd className="mt-1">{formatDateTime(current.deadline)}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground text-sm">
                  Antes da abertura
                </dt>
                <dd className="mt-1">
                  <Badge
                    variant={
                      current.show_before_open_date ? "success" : "secondary"
                    }
                  >
                    {current.show_before_open_date ? "Visível" : "Oculto"}
                  </Badge>
                </dd>
              </div>
              <div>
                <dt className="text-muted-foreground text-sm">
                  Tipos de arquivo permitidos
                </dt>
                <dd className="mt-1 text-sm">{allowedSummary}</dd>
              </div>
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
          </div>

          <div
            role="tabpanel"
            id="exercise-panel-tests"
            aria-labelledby="exercise-tab-tests"
            hidden={tab !== "tests"}
          >
            <TestCasesCard exerciseId={current.id} embedded />
          </div>

          <div
            role="tabpanel"
            id="exercise-panel-compilation"
            aria-labelledby="exercise-tab-compilation"
            hidden={tab !== "compilation"}
          >
            <CompilationFilesCard exerciseId={current.id} embedded />
          </div>

          <div
            role="tabpanel"
            id="exercise-panel-materials"
            aria-labelledby="exercise-tab-materials"
            hidden={tab !== "materials"}
          >
            <AttachedFilesCard exerciseId={current.id} embedded />
          </div>
        </div>
      </Card>

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
