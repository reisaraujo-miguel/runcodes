import { useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { unenroll, type UserOffering } from "@/lib/api";
import { formatDateTime } from "@/lib/format";

interface MyClassesProps {
  classes: UserOffering[];
  loading: boolean;
  error: string | null;
  /** Called with the offering id once the API confirms the unenrollment. */
  onUnenrolled: (offeringId: number) => void;
}

type RoleBadgeVariant = "default" | "secondary" | "info" | "warning";

const ROLE_LABELS: Record<string, string> = {
  student: "Aluno",
  professor: "Professor",
  monitor: "Monitor",
};

const ROLE_BADGE_VARIANTS: Record<string, RoleBadgeVariant> = {
  student: "secondary",
  professor: "info",
  monitor: "warning",
};

/** Translates the API role; ownership takes precedence over the stored role. */
function roleLabel(offering: UserOffering): string {
  if (offering.is_owner) return "Você é o responsável";
  return ROLE_LABELS[offering.role] ?? offering.role;
}

function roleBadgeVariant(offering: UserOffering): RoleBadgeVariant {
  if (offering.is_owner) return "default";
  return ROLE_BADGE_VARIANTS[offering.role] ?? "secondary";
}

/**
 * Lists the classes the caller belongs to. Leaving a class is destructive and
 * cannot be undone, so it asks for an inline confirmation before calling the
 * API instead of opening a dialog the user could dismiss by accident.
 */
export function MyClasses({
  classes,
  loading,
  error,
  onUnenrolled,
}: MyClassesProps) {
  const [confirmingId, setConfirmingId] = useState<number | null>(null);
  const [leavingId, setLeavingId] = useState<number | null>(null);
  const [leaveError, setLeaveError] = useState<{
    offeringId: number;
    message: string;
  } | null>(null);

  function handleLeave(offeringId: number) {
    setLeavingId(offeringId);
    setLeaveError(null);
    void (async () => {
      try {
        await unenroll(offeringId);
        setConfirmingId(null);
        onUnenrolled(offeringId);
      } catch (apiError) {
        setLeaveError({
          offeringId,
          message:
            apiError instanceof Error
              ? apiError.message
              : "Erro ao sair da turma",
        });
      } finally {
        setLeavingId(null);
      }
    })();
  }

  if (loading) {
    return <p className="text-muted-foreground text-sm">Carregando…</p>;
  }

  if (error) {
    return <p className="text-destructive text-sm">{error}</p>;
  }

  if (classes.length === 0) {
    return (
      <p className="text-muted-foreground text-sm">
        Você ainda não está matriculado em nenhuma turma.
      </p>
    );
  }

  return (
    <ul className="space-y-3">
      {classes.map((offering) => {
        const confirming = confirmingId === offering.offering_id;
        const leaving = leavingId === offering.offering_id;
        const itemError =
          leaveError?.offeringId === offering.offering_id
            ? leaveError.message
            : null;

        return (
          <li
            key={offering.offering_id}
            className="flex flex-wrap items-start justify-between gap-3 rounded-lg border p-3"
          >
            <div className="min-w-0 space-y-1">
              <div className="flex flex-wrap items-center gap-2">
                <p className="font-medium">{offering.name}</p>
                <Badge variant={roleBadgeVariant(offering)}>
                  {roleLabel(offering)}
                </Badge>
              </div>
              {offering.description && (
                <p className="text-muted-foreground text-sm">
                  {offering.description}
                </p>
              )}
              <p className="text-muted-foreground text-xs">
                Professor: {offering.owner_name}
              </p>
              <p className="text-muted-foreground text-xs">
                Disponível até {formatDateTime(offering.end_date)}
              </p>
            </div>

            {!offering.is_owner && (
              <div className="flex flex-col items-end gap-2">
                {confirming ? (
                  <div className="flex flex-wrap justify-end gap-2">
                    <Button
                      size="sm"
                      variant="destructive"
                      disabled={leaving}
                      onClick={() => {
                        handleLeave(offering.offering_id);
                      }}
                    >
                      {leaving ? "Saindo…" : "Confirmar saída"}
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={leaving}
                      onClick={() => {
                        setConfirmingId(null);
                        setLeaveError(null);
                      }}
                    >
                      Cancelar
                    </Button>
                  </div>
                ) : (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      setConfirmingId(offering.offering_id);
                      setLeaveError(null);
                    }}
                  >
                    Sair da turma
                  </Button>
                )}
                {itemError && (
                  <p className="text-destructive text-sm">{itemError}</p>
                )}
              </div>
            )}
          </li>
        );
      })}
    </ul>
  );
}
