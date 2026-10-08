import { useState } from "react";

import {
  BookOpenIcon,
  CalendarIcon,
  GraduationCapIcon,
  PlusIcon,
  TicketIcon,
  UsersIcon,
} from "lucide-react";
import { Link, useNavigate } from "react-router";

import { PageHeader } from "@/components/app/PageHeader";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { NewClassModal } from "@/components/professor/NewClassModal";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { useAsync } from "@/hooks/use-async";
import { listOfferings, type ManagedOffering, type Offering } from "@/lib/api";
import { formatDate } from "@/lib/format";

/** Renders a count with the right noun: "1 aluno" but "N alunos". */
function countLabel(count: number, singular: string, plural: string): string {
  return `${String(count)} ${count === 1 ? singular : plural}`;
}

/** A single managed class, linking to its detail page. */
function ClassCard({ offering }: { offering: ManagedOffering }) {
  return (
    <Card className="flex flex-col">
      <CardContent className="flex flex-1 flex-col gap-4">
        <div className="space-y-1.5">
          <div className="flex flex-wrap items-center gap-2">
            <p className="font-medium">{offering.name}</p>
            <Badge variant={offering.is_owner ? "default" : "info"}>
              {offering.is_owner ? "Responsável" : "Professor convidado"}
            </Badge>
          </div>
          {offering.description ? (
            <p className="text-muted-foreground line-clamp-2 text-sm">
              {offering.description}
            </p>
          ) : null}
        </div>

        <div className="text-muted-foreground flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
          <span className="inline-flex items-center gap-1.5">
            <UsersIcon className="size-3.5" aria-hidden />
            {countLabel(offering.member_count, "aluno", "alunos")}
          </span>
          <span className="inline-flex items-center gap-1.5">
            <BookOpenIcon className="size-3.5" aria-hidden />
            {countLabel(offering.exercise_count, "exercício", "exercícios")}
          </span>
          <span className="inline-flex items-center gap-1.5">
            <CalendarIcon className="size-3.5" aria-hidden />
            Até {formatDate(offering.end_date)}
          </span>
        </div>

        <div>
          <Badge variant={offering.enrollment_open ? "success" : "secondary"}>
            {offering.enrollment_open
              ? "Matrícula aberta"
              : "Matrícula encerrada"}
          </Badge>
        </div>

        <div className="bg-muted/50 flex items-center justify-between gap-2 rounded-lg px-3 py-2">
          <div className="min-w-0">
            <p className="text-muted-foreground text-xs">Código de matrícula</p>
            <code className="font-mono text-sm tracking-widest">
              {offering.enrollment_code}
            </code>
          </div>
          <TicketIcon
            className="text-muted-foreground size-4 shrink-0"
            aria-hidden
          />
        </div>

        <Button
          variant="outline"
          className="mt-auto"
          render={<Link to={`/professor/class/${String(offering.id)}`} />}
        >
          Abrir turma
        </Button>
      </CardContent>
    </Card>
  );
}

/**
 * Lists the classes the caller manages: the ones they own and the ones they
 * teach. Creating a class opens it directly, so the list is loaded once.
 */
export function ProfessorClassesPage() {
  const navigate = useNavigate();
  const [showNew, setShowNew] = useState(false);
  const offerings = useAsync(
    () => listOfferings(),
    "managed-offerings",
    "Não foi possível carregar as suas turmas.",
  );

  function handleCreated(offering: Offering) {
    setShowNew(false);
    void navigate(`/professor/class/${String(offering.id)}`, {
      state: offering,
    });
  }

  const list = offerings.data ?? [];

  return (
    <div className="space-y-6">
      <PageHeader
        title="Gerenciar Turmas"
        description="Turmas que você criou ou nas quais leciona."
        actions={
          <Button
            onClick={() => {
              setShowNew(true);
            }}
          >
            <PlusIcon />
            Nova turma
          </Button>
        }
      />

      {offerings.loading && offerings.data === null ? (
        <LoadingState />
      ) : offerings.error ? (
        <ErrorState description={offerings.error} onRetry={offerings.reload} />
      ) : list.length === 0 ? (
        <EmptyState
          icon={GraduationCapIcon}
          title="Você ainda não criou nenhuma turma"
          description="Crie uma turma e compartilhe o código de matrícula com os alunos."
          action={
            <Button
              onClick={() => {
                setShowNew(true);
              }}
            >
              <PlusIcon />
              Nova turma
            </Button>
          }
        />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {list.map((offering) => (
            <ClassCard key={offering.id} offering={offering} />
          ))}
        </div>
      )}

      {showNew ? (
        <NewClassModal
          onClose={() => {
            setShowNew(false);
          }}
          onCreated={handleCreated}
        />
      ) : null}
    </div>
  );
}
