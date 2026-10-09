import {
  BookOpenIcon,
  GraduationCapIcon,
  LayoutListIcon,
  TicketIcon,
} from "lucide-react";
import { NavLink } from "react-router";

import { PageHeader } from "@/components/app/PageHeader";
import { SectionCard } from "@/components/app/SectionCard";
import { StatCard } from "@/components/app/StatCard";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { ClassList } from "@/components/classes/ClassList";
import { EnrollForm } from "@/components/classes/EnrollForm";
import { ExerciseCard } from "@/components/exercises/ExerciseCard";
import { Button } from "@/components/ui/button";
import { useAsync } from "@/hooks/use-async";
import { useAuth } from "@/hooks/use-auth";
import { getMyOfferings, getMyOpenExercises, unenroll } from "@/lib/api";

function firstName(name: string): string {
  return name.trim().split(/\s+/)[0] ?? name;
}

export function Dashboard() {
  const { user } = useAuth();
  const isStaff = user?.role === "professor" || user?.role === "admin";

  const offerings = useAsync(() => getMyOfferings(), "my-offerings");
  const exercises = useAsync(() => getMyOpenExercises(), "open-exercises");

  const classes = offerings.data ?? [];

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
            ? "Resumo das suas turmas e das entregas abertas."
            : "Veja o que está para vencer e as suas disciplinas."
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

      <div className="grid gap-4 sm:grid-cols-2">
        <StatCard
          icon={BookOpenIcon}
          label="Entregas abertas"
          value={exercises.loading ? "…" : (exercises.data?.length ?? 0)}
          hint="Exercícios disponíveis agora"
        />
        <StatCard
          icon={GraduationCapIcon}
          label="Minhas turmas"
          value={offerings.loading ? "…" : classes.length}
          hint="Disciplinas em que você está matriculado"
          tone="info"
        />
      </div>

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
          ) : (exercises.data?.length ?? 0) === 0 ? (
            <EmptyState
              icon={BookOpenIcon}
              title="Nenhuma entrega aberta"
              description="Quando um exercício for liberado nas suas turmas, ele aparece aqui."
            />
          ) : (
            <div className="space-y-3">
              {exercises.data?.map((exercise) => (
                <ExerciseCard key={exercise.id} exercise={exercise} />
              ))}
            </div>
          )}
        </div>

        <aside className="space-y-6">
          <SectionCard title="Nova matrícula" description="Entre em uma turma.">
            <EnrollForm
              onEnrolled={() => {
                offerings.reload();
                exercises.reload();
              }}
            />
          </SectionCard>

          <SectionCard
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
          </SectionCard>
        </aside>
      </div>
    </div>
  );
}
