import {
  BookOpenIcon,
  CheckCircle2Icon,
  GraduationCapIcon,
  LayoutListIcon,
  TicketIcon,
} from "lucide-react";
import { NavLink } from "react-router";

import { PageHeader } from "@/components/app/PageHeader";
import { Section } from "@/components/app/Section";
import { StatStrip } from "@/components/app/StatStrip";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { ClassList } from "@/components/classes/ClassList";
import { EnrollForm } from "@/components/classes/EnrollForm";
import { ExerciseCard } from "@/components/exercises/ExerciseCard";
import { Button } from "@/components/ui/button";
import { useAsync } from "@/hooks/use-async";
import { useAuth } from "@/hooks/use-auth";
import { getMyExercises, getMyOfferings, unenroll } from "@/lib/api";
import { pendingExercises } from "@/lib/exercise-progress";

function firstName(name: string): string {
  return name.trim().split(/\s+/)[0] ?? name;
}

export function Dashboard() {
  const { user } = useAuth();
  const isStaff = user?.role === "professor" || user?.role === "admin";

  const offerings = useAsync(() => getMyOfferings(), "my-offerings");
  const exercises = useAsync(() => getMyExercises(), "my-exercises");

  const classes = offerings.data ?? [];
  const allExercises = exercises.data ?? [];
  const pending = pendingExercises(allExercises);
  const hasOpenExercises = allExercises.some((exercise) => exercise.is_open);

  async function handleLeave(offeringId: number) {
    await unenroll(offeringId);
    offerings.reload();
    exercises.reload();
  }

  return (
    <div className="space-y-8">
      <PageHeader
        title={`Olá, ${firstName(user?.name ?? "estudante")}`}
        description={
          isStaff
            ? "Resumo das suas turmas e das entregas pendentes."
            : "Veja o que ainda falta concluir e as suas disciplinas."
        }
        actions={
          isStaff ? (
            <Button render={<NavLink to="/professor" />}>
              <LayoutListIcon />
              Gerenciar turmas
            </Button>
          ) : undefined
        }
      />

      <StatStrip
        className="xl:grid-cols-2"
        items={[
          {
            icon: BookOpenIcon,
            label: "Entregas pendentes",
            value: exercises.loading ? "…" : pending.length,
            hint: "Exercícios abertos que faltam concluir",
            tone: "primary",
          },
          {
            icon: GraduationCapIcon,
            label: "Minhas turmas",
            value: offerings.loading ? "…" : classes.length,
            hint: "Disciplinas em que você está matriculado",
            tone: "info",
          },
        ]}
      />

      <div className="grid gap-6 lg:grid-cols-3">
        <div className="space-y-4 lg:col-span-2">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold">Próximas entregas</h2>
            <Button
              variant="link"
              size="sm"
              render={<NavLink to="/exercises" />}
            >
              Ver todas
            </Button>
          </div>

          {exercises.loading ? (
            <LoadingState />
          ) : exercises.error ? (
            <ErrorState
              description={exercises.error}
              onRetry={exercises.reload}
            />
          ) : pending.length === 0 ? (
            hasOpenExercises ? (
              <EmptyState
                icon={CheckCircle2Icon}
                title="Tudo em dia"
                description="Você já acertou todos os testes dos exercícios abertos no momento."
                action={
                  <Button
                    variant="outline"
                    size="sm"
                    render={<NavLink to="/exercises?tab=done" />}
                  >
                    Ver concluídos
                  </Button>
                }
              />
            ) : (
              <EmptyState
                icon={BookOpenIcon}
                title="Nenhuma entrega aberta"
                description="Quando um exercício for liberado nas suas turmas, ele aparece aqui."
              />
            )
          ) : (
            <div className="space-y-3">
              {pending.map((exercise) => (
                <ExerciseCard key={exercise.id} exercise={exercise} />
              ))}
            </div>
          )}
        </div>

        <aside className="space-y-6">
          <Section title="Nova matrícula" description="Entre em uma turma.">
            <EnrollForm
              onEnrolled={() => {
                offerings.reload();
                exercises.reload();
              }}
            />
          </Section>

          <Section
            title="Minhas turmas"
            action={
              <Button
                variant="ghost"
                size="sm"
                render={<NavLink to="/classes" />}
              >
                Gerenciar
              </Button>
            }
          >
            {offerings.loading ? (
              <LoadingState className="py-8" />
            ) : offerings.error ? (
              <ErrorState
                description={offerings.error}
                onRetry={offerings.reload}
              />
            ) : classes.length === 0 ? (
              <EmptyState
                icon={TicketIcon}
                title="Sem turmas"
                description="Use o código da turma para se matricular."
                className="py-8"
              />
            ) : (
              <ClassList classes={classes} onLeave={handleLeave} />
            )}
          </Section>
        </aside>
      </div>
    </div>
  );
}
