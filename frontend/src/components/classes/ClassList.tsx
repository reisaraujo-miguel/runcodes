import {
  ArrowRightIcon,
  CalendarIcon,
  LogOutIcon,
  UserRoundIcon,
} from "lucide-react";
import { Link } from "react-router";

import { ConfirmButton } from "@/components/app/ConfirmDialog";
import { EnrollmentRoleBadge } from "@/components/app/RoleBadge";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import type { UserOffering } from "@/lib/api";
import { formatDateTime } from "@/lib/format";

/** One class the user belongs to, with the option to leave it. */
function ClassCard({
  offering,
  onLeave,
}: {
  offering: UserOffering;
  onLeave: (offeringId: number) => void | Promise<void>;
}) {
  return (
    <Card className="@container">
      <CardContent className="flex flex-col gap-4 @md:flex-row @md:items-start @md:justify-between">
        <div className="min-w-0 space-y-1.5">
          <div className="flex flex-wrap items-center gap-2">
            <Link
              to={`/classes/${String(offering.offering_id)}`}
              className="truncate font-medium hover:underline"
            >
              {offering.name}
            </Link>
            <EnrollmentRoleBadge role={offering.role} />
            {offering.is_owner ? (
              <Badge variant="outline">Responsável</Badge>
            ) : null}
          </div>
          {offering.description ? (
            <p className="text-muted-foreground line-clamp-2 text-sm">
              {offering.description}
            </p>
          ) : null}
          <div className="text-muted-foreground flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
            <span className="inline-flex items-center gap-1.5">
              <UserRoundIcon className="size-3.5" aria-hidden />
              {offering.owner_name}
            </span>
            <span className="inline-flex items-center gap-1.5">
              <CalendarIcon className="size-3.5" aria-hidden />
              Termina em {formatDateTime(offering.end_date)}
            </span>
          </div>
        </div>

        <div className="flex shrink-0 flex-wrap items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            render={<Link to={`/classes/${String(offering.offering_id)}`} />}
          >
            Abrir turma
            <ArrowRightIcon data-icon="inline-end" />
          </Button>
          {!offering.is_owner ? (
            <ConfirmButton
              variant="outline"
              size="sm"
              destructive
              title="Sair da turma"
              description={`Você vai deixar “${offering.name}”. Para voltar depois, precisará do código de matrícula novamente.`}
              confirmLabel="Sair da turma"
              onConfirm={() => onLeave(offering.offering_id)}
            >
              <LogOutIcon />
              Sair
            </ConfirmButton>
          ) : null}
        </div>
      </CardContent>
    </Card>
  );
}

/** The list of classes the user belongs to. */
export function ClassList({
  classes,
  onLeave,
}: {
  classes: UserOffering[];
  onLeave: (offeringId: number) => void | Promise<void>;
}) {
  return (
    <div className="space-y-3">
      {classes.map((offering) => (
        <ClassCard
          key={offering.offering_id}
          offering={offering}
          onLeave={onLeave}
        />
      ))}
    </div>
  );
}
