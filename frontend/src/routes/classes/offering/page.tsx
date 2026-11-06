import { useMemo, useState } from "react";

import { BookOpenIcon, FilterXIcon, LayoutListIcon } from "lucide-react";
import { Link, useLocation, useParams, useSearchParams } from "react-router";

import { BackLink } from "@/components/app/BackLink";
import { FieldRow } from "@/components/app/Field";
import { PageHeader } from "@/components/app/PageHeader";
import { EnrollmentRoleBadge } from "@/components/app/RoleBadge";
import { Section } from "@/components/app/Section";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { BulletinBoard } from "@/components/classes/BulletinBoard";
import { ClassExerciseCard } from "@/components/classes/ClassExerciseCard";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { TabBar, type TabBarItem } from "@/components/ui/tab-bar";
import { useAsync } from "@/hooks/use-async";
import { useNow } from "@/hooks/use-now";
import { getMyOffering } from "@/lib/api";
import {
  arrangeExercises,
  summarizeExercises,
  type ClassExerciseFilter,
  type ClassExerciseSort,
} from "@/lib/class-exercises";
import { formatDateTime } from "@/lib/format";
import { fieldLabelClass } from "@/lib/label";

const breadcrumb = <BackLink to="/classes" />;

const classTabs: TabBarItem<ClassPageTab>[] = [
  { value: "exercises", label: "Exercícios" },
  { value: "bulletin", label: "Mural" },
];

const FILTERS: readonly { value: ClassExerciseFilter; label: string }[] = [
  { value: "all", label: "Todos" },
  { value: "open", label: "Posso enviar" },
  { value: "unsubmitted", label: "Sem submissão" },
  { value: "graded", label: "Com nota" },
];

const SORTS = [
  { value: "deadline", label: "Prazo" },
  { value: "score-desc", label: "Maior nota" },
  { value: "score-asc", label: "Menor nota" },
  { value: "recent", label: "Última entrega" },
] as const satisfies readonly { value: ClassExerciseSort; label: string }[];

function parseSort(value: string): ClassExerciseSort {
  return SORTS.find((option) => option.value === value)?.value ?? "deadline";
}

/**
 * The student's view of a class: class information, every visible exercise and
 * the caller's score/progress on it. The exercise list can be filtered to what
 * can still be submitted, what has no submission yet, and what has a grade.
 */
type ClassPageTab = "exercises" | "bulletin";

/** Reads the selected tab from `?tab=`, falling back to the exercises. */
function parseClassTab(value: string | null): ClassPageTab {
  return value === "bulletin" ? "bulletin" : "exercises";
}

export function OfferingPage() {
  const { offeringId } = useParams();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const origin = `${location.pathname}${location.search}`;
  const id = Number(offeringId);
  const valid = Number.isInteger(id) && id > 0;
  const key = offeringId ?? "";

  const data = useAsync(
    () =>
      valid ? getMyOffering(id) : Promise.reject(new Error("Turma inválida.")),
    `my-offering-${key}`,
    "Não foi possível carregar esta turma.",
  );

  const [filter, setFilter] = useState<ClassExerciseFilter>("all");
  const [sort, setSort] = useState<ClassExerciseSort>("deadline");
  const [tab, setTab] = useState<ClassPageTab>(() =>
    parseClassTab(searchParams.get("tab")),
  );
  const now = useNow();

  function selectTab(next: ClassPageTab) {
    setTab(next);
    const params = new URLSearchParams(searchParams);
    params.set("tab", next);
    setSearchParams(params, { replace: true });
  }

  const offering = data.data;
  const exercises = useMemo(() => offering?.exercises ?? [], [offering]);
  const visible = useMemo(
    () => arrangeExercises(exercises, filter, sort, now),
    [exercises, filter, sort, now],
  );
  const summary = useMemo(
    () => summarizeExercises(exercises, now),
    [exercises, now],
  );

  if (data.loading && offering === null) return <LoadingState />;

  if (!offering) {
    return (
      <div className="space-y-6">
        <PageHeader title="Turma" breadcrumb={breadcrumb} />
        <ErrorState
          description={data.error ?? "Não foi possível carregar esta turma."}
          onRetry={data.reload}
        />
      </div>
    );
  }

  const canManage = offering.is_owner || offering.role === "professor";
  const ended = Date.parse(offering.end_date) < now;
  const progress =
    summary.total === 0
      ? 0
      : Math.round((summary.submitted / summary.total) * 100);

  return (
    <div className="space-y-6">
      <PageHeader
        title={offering.name}
        description={
          offering.description ||
          (offering.owner_name ? `Turma de ${offering.owner_name}.` : undefined)
        }
        breadcrumb={breadcrumb}
        actions={
          <>
            <EnrollmentRoleBadge role={offering.role} />
            {offering.is_owner ? (
              <Badge variant="outline">Responsável</Badge>
            ) : null}
            {ended ? <Badge variant="secondary">Encerrada</Badge> : null}
            {canManage ? (
              <Button
                size="sm"
                variant="outline"
                render={
                  <Link
                    to={`/professor/class/${String(offering.id)}`}
                    state={{ from: origin }}
                  />
                }
              >
                <LayoutListIcon />
                Gerenciar turma
              </Button>
            ) : null}
          </>
        }
      />

      <TabBar
        items={classTabs}
        value={tab}
        onChange={selectTab}
        label="Seções da turma"
        idPrefix="class-page"
      />

      <div
        role="tabpanel"
        id="class-page-panel-exercises"
        aria-labelledby="class-page-tab-exercises"
        hidden={tab !== "exercises"}
      >
        <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_22rem]">
          <section
            className="min-w-0 space-y-4"
            aria-labelledby="class-exercises-heading"
          >
            <div className="flex flex-col gap-1">
              <h2
                id="class-exercises-heading"
                className="text-lg font-semibold"
              >
                Exercícios
              </h2>
              <p className="text-sm text-muted-foreground">
                Acompanhe seus prazos, entregas e notas nesta turma.
              </p>
            </div>

            {exercises.length > 0 ? (
              <div className="flex flex-col gap-3">
                <div
                  role="group"
                  aria-label="Filtrar exercícios"
                  className="flex flex-wrap gap-1"
                >
                  {FILTERS.map((option) => (
                    <Button
                      key={option.value}
                      type="button"
                      size="sm"
                      variant={filter === option.value ? "secondary" : "ghost"}
                      aria-pressed={filter === option.value}
                      onClick={() => {
                        setFilter(option.value);
                      }}
                    >
                      {option.label}
                    </Button>
                  ))}
                </div>

                <div className="flex flex-wrap items-center justify-between gap-3">
                  <p
                    className="text-xs text-muted-foreground"
                    aria-live="polite"
                  >
                    {visible.length} de {exercises.length} exercício
                    {exercises.length === 1 ? "" : "s"}
                  </p>
                  <div className="flex items-center gap-2">
                    <Label
                      htmlFor="class-exercise-sort"
                      className="text-xs whitespace-nowrap text-muted-foreground"
                    >
                      Ordenar por
                    </Label>
                    <Select
                      id="class-exercise-sort"
                      className="h-8 w-36 text-xs"
                      value={sort}
                      onChange={(event) => {
                        setSort(parseSort(event.target.value));
                      }}
                    >
                      {SORTS.map((option) => (
                        <option key={option.value} value={option.value}>
                          {option.label}
                        </option>
                      ))}
                    </Select>
                  </div>
                </div>
              </div>
            ) : null}

            {exercises.length === 0 ? (
              <EmptyState
                icon={BookOpenIcon}
                title="Nenhum exercício publicado"
                description="Quando o professor liberar um exercício, ele aparece aqui com o prazo e sua nota."
              />
            ) : visible.length === 0 ? (
              <EmptyState
                icon={FilterXIcon}
                title="Nenhum exercício neste filtro"
                description="Ajuste o filtro para ver os outros exercícios desta turma."
                action={
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      setFilter("all");
                    }}
                  >
                    Limpar filtro
                  </Button>
                }
              />
            ) : (
              <div className="space-y-3">
                {visible.map((exercise) => (
                  <ClassExerciseCard
                    key={exercise.id}
                    exercise={exercise}
                    now={now}
                  />
                ))}
              </div>
            )}
          </section>

          <aside className="space-y-8">
            <Section title="Sobre a turma" headingLevel={3}>
              <dl className="space-y-3 text-sm">
                <FieldRow label="Professor">
                  {offering.owner_name || "—"}
                </FieldRow>
                <FieldRow label="Termina em">
                  {formatDateTime(offering.end_date)}
                </FieldRow>
                <FieldRow label="Participantes">
                  <span className="tabular-nums">{offering.member_count}</span>
                </FieldRow>
                <FieldRow label="Exercícios">
                  <span className="tabular-nums">{exercises.length}</span>
                </FieldRow>
              </dl>
            </Section>

            <Section
              title="Seu progresso"
              description="Entregas e notas nesta turma."
              headingLevel={3}
            >
              <div className="space-y-4">
                <div>
                  <div className="mb-1.5 flex items-center justify-between text-xs">
                    <span className="text-muted-foreground">
                      Exercícios com entrega
                    </span>
                    <span className="font-medium tabular-nums">
                      {progress}%
                    </span>
                  </div>
                  <div
                    role="progressbar"
                    aria-label="Exercícios com entrega"
                    aria-valuemin={0}
                    aria-valuemax={100}
                    aria-valuenow={progress}
                    className="h-2 overflow-hidden rounded-full bg-muted"
                  >
                    <div
                      className="h-full rounded-full bg-primary"
                      style={{ width: `${String(progress)}%` }}
                    />
                  </div>
                </div>

                <dl className="grid grid-cols-2 gap-x-4 gap-y-3 border-t pt-4">
                  <div>
                    <dt className={fieldLabelClass}>Entregues</dt>
                    <dd className="mt-0.5 text-lg font-semibold tabular-nums">
                      {summary.submitted}/{summary.total}
                    </dd>
                  </div>
                  <div>
                    <dt className={fieldLabelClass}>Abertos</dt>
                    <dd className="mt-0.5 text-lg font-semibold tabular-nums">
                      {summary.open}
                    </dd>
                  </div>
                  <div>
                    <dt className={fieldLabelClass}>Com nota</dt>
                    <dd className="mt-0.5 text-lg font-semibold tabular-nums">
                      {summary.graded}
                    </dd>
                  </div>
                  <div>
                    <dt className={fieldLabelClass}>Média</dt>
                    <dd className="mt-0.5 text-lg font-semibold tabular-nums">
                      {summary.averageScore !== null
                        ? summary.averageScore.toFixed(2)
                        : "—"}
                    </dd>
                  </div>
                </dl>
              </div>
            </Section>
          </aside>
        </div>
      </div>

      <div
        role="tabpanel"
        id="class-page-panel-bulletin"
        aria-labelledby="class-page-tab-bulletin"
        hidden={tab !== "bulletin"}
      >
        <BulletinBoard offeringId={offering.id} showTitle={false} />
      </div>
    </div>
  );
}
