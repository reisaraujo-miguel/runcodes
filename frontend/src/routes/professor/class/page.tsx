import { useEffect, useState } from "react";
import { Link, useLocation, useParams } from "react-router";

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
  getOffering,
  getOfferingExercises,
  type Exercise,
  type Offering,
} from "@/lib/api";
import { formatDateTime } from "@/lib/format";

/**
 * Page for a class offering. After creating a class, the offering is passed
 * through navigation state to avoid a loading flash; the page always
 * (re)fetches from the API so it also works on refresh/direct visits.
 */
export function ClassPage() {
  const location = useLocation();
  const { offeringId } = useParams();
  const initialOffering = (location.state as Offering | null) ?? null;
  const [offering, setOffering] = useState<Offering | null>(initialOffering);
  const [loading, setLoading] = useState(initialOffering === null);
  const [error, setError] = useState<string | null>(null);

  const [exercises, setExercises] = useState<Exercise[]>([]);
  const [exercisesLoading, setExercisesLoading] = useState(true);
  const [exercisesError, setExercisesError] = useState<string | null>(null);
  const [showNewExercise, setShowNewExercise] = useState(false);

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
    </div>
  );
}
