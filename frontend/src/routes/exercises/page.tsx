import { useMemo, type ReactNode } from "react";

import { BookOpenIcon, CheckCircle2Icon } from "lucide-react";
import { useSearchParams } from "react-router";

import { PageHeader } from "@/components/app/PageHeader";
import { Section } from "@/components/app/Section";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { ExerciseCard } from "@/components/exercises/ExerciseCard";
import { Button } from "@/components/ui/button";
import { TabBar } from "@/components/ui/tab-bar";
import { useAsync } from "@/hooks/use-async";
import { getMyExercises, type UserExercise } from "@/lib/api";
import { completedExercises, pendingExercises } from "@/lib/exercise-progress";

/** Which exercise list the page shows. */
type ExerciseTab = "open" | "done";

/** Reads the selected tab from `?tab=`, falling back to the pending list. */
function parseTab(value: string | null): ExerciseTab {
  return value === "done" ? "done" : "open";
}

interface ExerciseGroup {
  offeringId: number;
  offeringName: string;
  exercises: UserExercise[];
}

/** Groups an exercise list by class, preserving the server's deadline order. */
function groupByOffering(exercises: UserExercise[]): ExerciseGroup[] {
  const groups = new Map<number, ExerciseGroup>();
  for (const exercise of exercises) {
    let group = groups.get(exercise.offering_id);
    if (!group) {
      group = {
        offeringId: exercise.offering_id,
        offeringName: exercise.offering_name,
        exercises: [],
      };
      groups.set(exercise.offering_id, group);
    }
    group.exercises.push(exercise);
  }
  return [...groups.values()];
}

function ExerciseGroups({
  groups,
  empty,
}: {
  groups: ExerciseGroup[];
  empty: ReactNode;
}) {
  if (groups.length === 0) return <>{empty}</>;

  return (
    <div className="space-y-8">
      {groups.map((group) => (
        <Section
          key={group.offeringId}
          title={group.offeringName}
          contentClassName="space-y-3"
        >
          {group.exercises.map((exercise) => (
            <ExerciseCard key={exercise.id} exercise={exercise} />
          ))}
        </Section>
      ))}
    </div>
  );
}

/**
 * Every exercise that has opened in the caller's classes, split into what is
 * still worth solving (open and not fully passed) and what has been passed.
 * Completed exercises stay visible after the deadline instead of disappearing
 * from the list.
 */
export function ExercisesPage() {
  const exercises = useAsync(() => getMyExercises(), "my-exercises");
  const [searchParams, setSearchParams] = useSearchParams();
  const tab = parseTab(searchParams.get("tab"));

  const data = useMemo(() => exercises.data ?? [], [exercises.data]);
  const pending = useMemo(() => pendingExercises(data), [data]);
  const completed = useMemo(() => completedExercises(data), [data]);
  const pendingGroups = useMemo(() => groupByOffering(pending), [pending]);
  const completedGroups = useMemo(
    () => groupByOffering(completed),
    [completed],
  );

  function selectTab(next: ExerciseTab) {
    const params = new URLSearchParams(searchParams);
    params.set("tab", next);
    setSearchParams(params, { replace: true });
  }

  const tabItems = [
    { value: "open" as const, label: "Em aberto", badge: pending.length },
    { value: "done" as const, label: "Concluídos", badge: completed.length },
  ];

  return (
    <div className="space-y-6">
      <PageHeader
        title="Exercícios"
        description="Seus exercícios nas turmas, separados entre o que falta resolver e o que você já acertou por completo."
      />

      {exercises.loading ? (
        <LoadingState />
      ) : exercises.error ? (
        <ErrorState description={exercises.error} onRetry={exercises.reload} />
      ) : (
        <>
          <TabBar
            items={tabItems}
            value={tab}
            onChange={selectTab}
            label="Exercícios"
            idPrefix="exercises-page"
          />

          <div
            role="tabpanel"
            id="exercises-page-panel-open"
            aria-labelledby="exercises-page-tab-open"
            hidden={tab !== "open"}
          >
            <ExerciseGroups
              groups={pendingGroups}
              empty={
                <EmptyState
                  icon={BookOpenIcon}
                  title="Nenhum exercício em aberto"
                  description={
                    completed.length > 0
                      ? "Você concluiu todos os exercícios abertos. Seus concluídos estão na outra aba."
                      : "Quando um exercício for liberado nas suas turmas, ele aparece aqui."
                  }
                  action={
                    completed.length > 0 ? (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => {
                          selectTab("done");
                        }}
                      >
                        Ver concluídos
                      </Button>
                    ) : undefined
                  }
                />
              }
            />
          </div>

          <div
            role="tabpanel"
            id="exercises-page-panel-done"
            aria-labelledby="exercises-page-tab-done"
            hidden={tab !== "done"}
          >
            <ExerciseGroups
              groups={completedGroups}
              empty={
                <EmptyState
                  icon={CheckCircle2Icon}
                  title="Nenhum exercício concluído"
                  description="Quando você acertar todos os testes de um exercício, ele aparece aqui com a sua nota."
                  action={
                    pending.length > 0 ? (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => {
                          selectTab("open");
                        }}
                      >
                        Ver em aberto
                      </Button>
                    ) : undefined
                  }
                />
              }
            />
          </div>
        </>
      )}
    </div>
  );
}
