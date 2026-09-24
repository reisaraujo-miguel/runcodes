import { useEffect, useState } from "react";
import { Link } from "react-router";

import { NewClassModal } from "@/components/professor/NewClassModal";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { listOfferings, type ManagedOffering } from "@/lib/api";
import { formatDateTime } from "@/lib/format";

/** Renders a count with the right noun: "1 aluno" but "N alunos". */
function countLabel(count: number, singular: string, plural: string): string {
  return `${String(count)} ${count === 1 ? singular : plural}`;
}

/**
 * Lists the classes the caller manages: the ones they own and the ones they
 * were invited to teach. Creating a class happens in the shared modal, which
 * navigates to the new class page on success, so the list is only loaded once.
 */
export function ProfessorClassesPage() {
  const [offerings, setOfferings] = useState<ManagedOffering[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isNewClassModalOpen, setIsNewClassModalOpen] = useState(false);

  useEffect(() => {
    let cancelled = false;

    async function loadOfferings() {
      try {
        const data = await listOfferings();
        if (!cancelled) setOfferings(data);
      } catch {
        if (!cancelled) {
          setError("Não foi possível carregar as suas turmas.");
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    void loadOfferings();

    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="mx-auto max-w-3xl space-y-4 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold">Minhas Turmas</h1>
          <p className="text-muted-foreground text-sm">
            Turmas que você criou ou nas quais leciona.
          </p>
        </div>
        <Button
          onClick={() => {
            setIsNewClassModalOpen(true);
          }}
        >
          Nova turma
        </Button>
      </div>

      {loading && (
        <div className="flex justify-center p-12">
          <div
            aria-label="Carregando"
            className="h-6 w-6 animate-spin rounded-full border-2 border-muted-foreground/30 border-t-muted-foreground"
            role="status"
          />
        </div>
      )}

      {error && <p className="text-destructive text-sm">{error}</p>}

      {!loading && !error && offerings.length === 0 && (
        <p className="text-muted-foreground text-sm">
          Você ainda não criou nenhuma turma.
        </p>
      )}

      {offerings.length > 0 && (
        <ul className="space-y-3">
          {offerings.map((offering) => (
            <li key={offering.id} className="rounded-lg border p-3">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0 space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="font-medium">{offering.name}</p>
                    <Badge variant={offering.is_owner ? "default" : "info"}>
                      {offering.is_owner
                        ? "Você é o responsável"
                        : "Professor convidado"}
                    </Badge>
                  </div>
                  {offering.description && (
                    <p className="text-muted-foreground text-sm">
                      {offering.description}
                    </p>
                  )}
                  <p className="text-muted-foreground text-xs">
                    Disponível até {formatDateTime(offering.end_date)}
                  </p>
                  <div className="text-muted-foreground flex flex-wrap gap-x-3 gap-y-1 text-xs">
                    <span>
                      {countLabel(offering.member_count, "aluno", "alunos")}
                    </span>
                    <span>
                      {countLabel(
                        offering.exercise_count,
                        "exercício",
                        "exercícios",
                      )}
                    </span>
                    {!offering.enrollment_open && (
                      <span>Matrícula encerrada</span>
                    )}
                  </div>
                </div>

                <div className="flex flex-col items-end gap-2">
                  <div className="text-right">
                    <p className="text-muted-foreground text-xs">
                      Código de matrícula
                    </p>
                    <p className="font-mono tracking-widest">
                      {offering.enrollment_code}
                    </p>
                  </div>
                  <Link
                    to={`/professor/class/${String(offering.id)}`}
                    className={buttonVariants({
                      variant: "outline",
                      size: "sm",
                    })}
                  >
                    Abrir turma
                  </Link>
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}

      {isNewClassModalOpen && (
        <NewClassModal
          onClose={() => {
            setIsNewClassModalOpen(false);
          }}
        />
      )}
    </div>
  );
}
