import { useState } from "react";

import { PencilIcon, Trash2Icon } from "lucide-react";
import { Link, useNavigate, useParams } from "react-router";

import { ConfirmButton } from "@/components/app/ConfirmDialog";
import { PageHeader } from "@/components/app/PageHeader";
import { SectionCard } from "@/components/app/SectionCard";
import { ErrorState, LoadingState } from "@/components/app/states";
import { AttachedFilesCard } from "@/components/professor/AttachedFilesCard";
import { CompilationFilesCard } from "@/components/professor/CompilationFilesCard";
import { EditExerciseForm } from "@/components/professor/EditExerciseForm";
import { TestCasesCard } from "@/components/professor/TestCasesCard";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useAsync } from "@/hooks/use-async";
import { deleteExercise, getExercise } from "@/lib/api";
import { errorMessage } from "@/lib/errors";
import { formatDateTime } from "@/lib/format";

/**
 * Page to manage a single exercise: its data, test cases, compilation files and
 * attached materials. Deleting an exercise is a soft delete, so it must be
 * confirmed before navigating back to the class.
 */
export function ExercisePage() {
  const { exerciseId } = useParams();
  const navigate = useNavigate();
  const id = Number(exerciseId);
  const valid = Number.isInteger(id) && id > 0;

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

  return (
    <div className="space-y-6">
      <PageHeader
        title={current.title}
        description={current.description || undefined}
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

      <SectionCard
        title="Detalhes do exercício"
        action={
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              setEditing(true);
            }}
          >
            <PencilIcon />
            Editar exercício
          </Button>
        }
      >
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
            <dt className="text-muted-foreground text-sm">Antes da abertura</dt>
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
      </SectionCard>

      <TestCasesCard exerciseId={current.id} />

      <CompilationFilesCard exerciseId={current.id} />

      <AttachedFilesCard exerciseId={current.id} />

      <SectionCard
        title="Excluir exercício"
        description="O exercício deixa de aparecer para os alunos; os casos de teste, arquivos e submissões são preservados para consulta."
      >
        <div className="space-y-3">
          {deleteError ? (
            <Alert variant="destructive">
              <AlertDescription>{deleteError}</AlertDescription>
            </Alert>
          ) : null}
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
      </SectionCard>

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
