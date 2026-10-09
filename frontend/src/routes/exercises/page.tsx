import { BookOpenIcon } from "lucide-react";

import { PageHeader } from "@/components/app/PageHeader";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { ExerciseCard } from "@/components/exercises/ExerciseCard";
import { useAsync } from "@/hooks/use-async";
import { getMyOpenExercises } from "@/lib/api";

export function ExercisesPage() {
  const exercises = useAsync(() => getMyOpenExercises(), "open-exercises");
  const data = exercises.data ?? [];

  const groups = new Map<string, typeof data>();
  for (const exercise of data) {
    const bucket = groups.get(exercise.offering_name) ?? [];
    bucket.push(exercise);
    groups.set(exercise.offering_name, bucket);
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Exercícios"
        description="Exercícios abertos agora nas suas turmas, ordenados pelo prazo."
      />

      {exercises.loading ? (
        <LoadingState />
      ) : exercises.error ? (
        <ErrorState description={exercises.error} onRetry={exercises.reload} />
      ) : data.length === 0 ? (
        <EmptyState
          icon={BookOpenIcon}
          title="Nenhum exercício aberto"
          description="Quando um exercício for liberado nas suas turmas, ele aparece aqui."
        />
      ) : (
        <div className="space-y-8">
          {[...groups.entries()].map(([offeringName, items]) => (
            <section key={offeringName} className="space-y-3">
              <h2 className="text-muted-foreground text-sm font-semibold tracking-wide uppercase">
                {offeringName}
              </h2>
              {items.map((exercise) => (
                <ExerciseCard key={exercise.id} exercise={exercise} />
              ))}
            </section>
          ))}
        </div>
      )}
    </div>
  );
}
