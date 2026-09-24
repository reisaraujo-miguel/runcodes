import { useEffect, useState } from "react";
import { Link } from "react-router";

import { buttonVariants } from "@/components/ui/button";
import { getMyOpenExercises, type OpenExercise } from "@/lib/api";
import { formatDateTime } from "@/lib/format";

interface OpenExercisesProps {
  /** Bumped by the home page to reload the list after an enrollment. */
  refreshKey: number;
}

/** Exercises that are open right now across every class the caller belongs to. */
export function OpenExercises({ refreshKey }: OpenExercisesProps) {
  const [exercises, setExercises] = useState<OpenExercise[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function loadOpenExercises() {
      setLoading(true);
      setError(null);
      try {
        const data = await getMyOpenExercises();
        if (!cancelled) setExercises(data);
      } catch {
        if (!cancelled) {
          setError("Não foi possível carregar os exercícios em aberto.");
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    void loadOpenExercises();

    return () => {
      cancelled = true;
    };
  }, [refreshKey]);

  if (loading) {
    return <p className="text-muted-foreground text-sm">Carregando…</p>;
  }

  if (error) {
    return <p className="text-destructive text-sm">{error}</p>;
  }

  if (exercises.length === 0) {
    return (
      <p className="text-muted-foreground text-sm">
        Nenhum exercício em aberto no momento.
      </p>
    );
  }

  return (
    <ul className="space-y-3">
      {exercises.map((exercise) => (
        <li
          key={exercise.id}
          className="flex flex-wrap items-start justify-between gap-3 rounded-lg border p-3"
        >
          <div className="min-w-0 space-y-1">
            <p className="font-medium">{exercise.title}</p>
            <p className="text-muted-foreground text-sm">
              Turma: {exercise.offering_name}
            </p>
            <p className="text-muted-foreground text-xs">
              Prazo: {formatDateTime(exercise.deadline)}
            </p>
          </div>
          <Link
            to={`/exercises/${String(exercise.id)}/submit`}
            className={buttonVariants({ variant: "outline", size: "sm" })}
          >
            Enviar solução
          </Link>
        </li>
      ))}
    </ul>
  );
}
