import { useState } from "react";

import { GraduationCapIcon } from "lucide-react";

import { PageHeader } from "@/components/app/PageHeader";
import { SectionCard } from "@/components/app/SectionCard";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { ClassList } from "@/components/classes/ClassList";
import { EnrollForm } from "@/components/classes/EnrollForm";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { useAsync } from "@/hooks/use-async";
import { getMyOfferings, unenroll } from "@/lib/api";
import { errorMessage } from "@/lib/errors";

export function ClassesPage() {
  const offerings = useAsync(() => getMyOfferings(), "my-offerings");
  const [actionError, setActionError] = useState<string | null>(null);
  const classes = offerings.data ?? [];

  async function handleLeave(offeringId: number) {
    setActionError(null);
    try {
      await unenroll(offeringId);
      offerings.reload();
    } catch (error) {
      setActionError(errorMessage(error, "Não foi possível sair da turma."));
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Minhas Turmas"
        description="As disciplinas em que você está matriculado ou que administra."
      />

      {actionError ? (
        <Alert variant="destructive">
          <AlertDescription>{actionError}</AlertDescription>
        </Alert>
      ) : null}

      <div className="grid gap-6 lg:grid-cols-3">
        <div className="lg:col-span-2">
          {offerings.loading ? (
            <LoadingState />
          ) : offerings.error ? (
            <ErrorState
              description={offerings.error}
              onRetry={offerings.reload}
            />
          ) : classes.length === 0 ? (
            <EmptyState
              icon={GraduationCapIcon}
              title="Você ainda não está em nenhuma turma"
              description="Use o código de matrícula para entrar na sua primeira disciplina."
            />
          ) : (
            <ClassList classes={classes} onLeave={handleLeave} />
          )}
        </div>

        <aside>
          <SectionCard
            title="Nova matrícula"
            description="Entre em uma turma com o código do professor."
          >
            <EnrollForm
              onEnrolled={() => {
                offerings.reload();
              }}
            />
          </SectionCard>
        </aside>
      </div>
    </div>
  );
}
