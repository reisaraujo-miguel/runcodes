import { useEffect, useState } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router";

import { ClassMembersCard } from "@/components/professor/ClassMembersCard";
import { EditClassForm } from "@/components/professor/EditClassForm";
import { NewExerciseForm } from "@/components/professor/NewExerciseForm";
import { Button, buttonVariants } from "@/components/ui/button";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

import {
  deleteOffering,
  getOffering,
  getOfferingExercises,
  type Exercise,
  type Offering,
} from "@/lib/api";
import { ApiRequestError } from "@/lib/api/client";
import { formatDateTime } from "@/lib/format";

/**
 * The class page is reachable by a co-professor too, for whom deleting is
 * forbidden: the API answers 403 and this reports it as a permission problem
 * instead of showing a failure.
 */
function deleteErrorMessage(error: unknown): string {
  if (error instanceof ApiRequestError && error.status === 403) {
    return "Apenas o professor responsável pela turma pode excluí-la.";
  }
  return error instanceof Error ? error.message : "Erro ao excluir a turma";
}

/**
 * Page for a class offering. After creating a class, the offering is passed
 * through navigation state to avoid a loading flash; the page always
 * (re)fetches from the API so it also works on refresh/direct visits.
 */
export function ClassPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const { offeringId } = useParams();
  const initialOffering = (location.state as Offering | null) ?? null;
  const [offering, setOffering] = useState<Offering | null>(initialOffering);
  const [loading, setLoading] = useState(initialOffering === null);
  const [error, setError] = useState<string | null>(null);

  const [exercises, setExercises] = useState<Exercise[]>([]);
  const [exercisesLoading, setExercisesLoading] = useState(true);
  const [exercisesError, setExercisesError] = useState<string | null>(null);
  const [showNewExercise, setShowNewExercise] = useState(false);
  const [showEditClass, setShowEditClass] = useState(false);

  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const offeringIdNumber = Number(offeringId);

  useEffect(() => {
    let cancelled = false;
    const id = offeringIdNumber;

    async function loadOffering() {
      if (!Number.isInteger(id) || id <= 0) {
        setError("Turma inválida");
        setLoading(false);
        return;
      }
      try {
        const data = await getOffering(id);
        if (!cancelled) setOffering(data);
      } catch {
        if (!cancelled) setError("Turma não encontrada");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    void loadOffering();

    return () => {
      cancelled = true;
    };
  }, [offeringIdNumber]);

  useEffect(() => {
    let cancelled = false;

    async function loadExercises() {
      if (!Number.isInteger(offeringIdNumber) || offeringIdNumber <= 0) {
        setExercisesLoading(false);
        return;
      }
      try {
        const data = await getOfferingExercises(offeringIdNumber);
        if (!cancelled) setExercises(data);
      } catch {
        if (!cancelled) {
          setExercisesError("Não foi possível carregar os exercícios.");
        }
      } finally {
        if (!cancelled) setExercisesLoading(false);
      }
    }

    void loadExercises();

    return () => {
      cancelled = true;
    };
  }, [offeringIdNumber]);

  /**
   * Deletes the class and everything in it. The API is the only thing that can
   * confirm the deletion, so the page navigates back to the class list once it
   * answers and only reports the failure otherwise.
   */
  function handleDeleteClass(id: number) {
    setDeleting(true);
    setDeleteError(null);
    void (async () => {
      try {
        await deleteOffering(id);
        void navigate("/professor");
      } catch (apiError) {
        setDeleteError(deleteErrorMessage(apiError));
        setDeleting(false);
      }
    })();
  }

  if (loading) {
    return (
      <div className="flex justify-center p-12">
        <div
          aria-label="Carregando"
          className="h-6 w-6 animate-spin rounded-full border-2 border-muted-foreground/30 border-t-muted-foreground"
          role="status"
        />
      </div>
    );
  }

  if (!offering) {
    return (
      <div className="mx-auto max-w-3xl p-6">
        <Card>
          <CardHeader>
            <CardTitle>Turma não encontrada</CardTitle>
            <CardDescription>
              {error ?? "Não foi possível carregar os dados desta turma."}
            </CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  const parsedEndDate = offering.end_date ? new Date(offering.end_date) : null;
  const endDate =
    parsedEndDate && !Number.isNaN(parsedEndDate.getTime())
      ? parsedEndDate.toLocaleString(undefined, {
          dateStyle: "long",
          timeStyle: "short",
        })
      : null;

  return (
    <div className="mx-auto max-w-3xl space-y-4 p-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-2xl">{offering.name}</CardTitle>
          {offering.description && (
            <CardDescription>{offering.description}</CardDescription>
          )}
          <CardAction>
            <Button
              size="sm"
              variant={showEditClass ? "outline" : "default"}
              onClick={() => {
                setShowEditClass((previous) => !previous);
              }}
            >
              {showEditClass ? "Fechar" : "Editar turma"}
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent className="space-y-6">
          <div>
            <p className="text-sm text-muted-foreground">Código de matrícula</p>
            <p className="font-mono text-lg tracking-widest">
              {offering.enrollment_code}
            </p>
          </div>
          {endDate && (
            <div>
              <p className="text-sm text-muted-foreground">Disponível até</p>
              <p>{endDate}</p>
            </div>
          )}

          {showEditClass && (
            <EditClassForm
              offering={offering}
              onSaved={(updated) => {
                setOffering(updated);
                setShowEditClass(false);
              }}
              onCancel={() => {
                setShowEditClass(false);
              }}
            />
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Exercícios</CardTitle>
          <CardDescription>
            Crie e gerencie os exercícios desta turma.
          </CardDescription>
          <CardAction>
            <Button
              size="sm"
              variant={showNewExercise ? "outline" : "default"}
              onClick={() => {
                setShowNewExercise((previous) => !previous);
              }}
            >
              {showNewExercise ? "Fechar" : "Novo exercício"}
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent className="space-y-4">
          {showNewExercise && (
            <NewExerciseForm
              offeringId={offering.id}
              onCancel={() => {
                setShowNewExercise(false);
              }}
              onCreated={(exercise) => {
                setExercises((previous) => [exercise, ...previous]);
                setShowNewExercise(false);
              }}
            />
          )}

          {exercisesLoading && (
            <p className="text-muted-foreground text-sm">
              Carregando exercícios…
            </p>
          )}

          {exercisesError && (
            <p className="text-destructive text-sm">{exercisesError}</p>
          )}

          {!exercisesLoading && !exercisesError && exercises.length === 0 && (
            <p className="text-muted-foreground text-sm">
              Nenhum exercício cadastrado ainda.
            </p>
          )}

          {exercises.length > 0 && (
            <ul className="space-y-3">
              {exercises.map((exercise) => (
                <li
                  key={exercise.id}
                  className="flex flex-wrap items-start justify-between gap-3 rounded-lg border p-3"
                >
                  <div className="min-w-0 space-y-1">
                    <p className="font-medium">{exercise.title}</p>
                    {exercise.description && (
                      <p className="text-muted-foreground text-sm">
                        {exercise.description}
                      </p>
                    )}
                    <p className="text-muted-foreground text-xs">
                      Prazo: {formatDateTime(exercise.deadline)}
                    </p>
                  </div>
                  <div className="flex shrink-0 gap-2">
                    <Link
                      to={`/exercises/${String(exercise.id)}/submit`}
                      className={buttonVariants({
                        variant: "outline",
                        size: "sm",
                      })}
                    >
                      Enviar solução
                    </Link>
                    <Link
                      to={`/professor/exercise/${String(exercise.id)}`}
                      className={buttonVariants({
                        variant: "secondary",
                        size: "sm",
                      })}
                    >
                      Gerenciar
                    </Link>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      <ClassMembersCard offeringId={offering.id} />

      <Card>
        <CardHeader>
          <CardTitle>Excluir turma</CardTitle>
          <CardDescription>
            Excluir a turma apaga também os exercícios, os casos de teste e as
            submissões dos alunos. Esta ação não pode ser desfeita.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {deleteError && (
            <p className="text-destructive text-sm">{deleteError}</p>
          )}

          {confirmingDelete ? (
            <div className="flex flex-wrap gap-2">
              <Button
                variant="destructive"
                disabled={deleting}
                onClick={() => {
                  handleDeleteClass(offering.id);
                }}
              >
                {deleting ? "Excluindo…" : "Confirmar exclusão"}
              </Button>
              <Button
                variant="outline"
                disabled={deleting}
                onClick={() => {
                  setConfirmingDelete(false);
                  setDeleteError(null);
                }}
              >
                Cancelar
              </Button>
            </div>
          ) : (
            <Button
              variant="destructive"
              onClick={() => {
                setConfirmingDelete(true);
                setDeleteError(null);
              }}
            >
              Excluir turma
            </Button>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
