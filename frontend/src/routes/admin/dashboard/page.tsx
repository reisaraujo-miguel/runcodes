import {
  BookOpenIcon,
  GraduationCapIcon,
  InfoIcon,
  TicketIcon,
  UsersIcon,
} from "lucide-react";

import { PreviewControls } from "@/components/admin/PreviewControls";
import { PageHeader } from "@/components/app/PageHeader";
import { StatCard } from "@/components/app/StatCard";
import { ErrorState, LoadingState } from "@/components/app/states";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { useAsync } from "@/hooks/use-async";
import {
  ADMIN_PAGE_LIMIT,
  adminListOfferings,
  adminListUsers,
} from "@/lib/api";

/** Whether an ISO timestamp is still in the future (an unparseable one is not). */
function isFuture(value: string): boolean {
  const time = new Date(value).getTime();
  return Number.isFinite(time) && time > Date.now();
}

/**
 * The admin overview. There is no statistics endpoint, so the metrics are
 * derived from the first page of the user and class listings; the page says so
 * rather than presenting the totals as complete.
 */
export function AdminDashboard() {
  const overview = useAsync(
    async () => {
      const [users, offerings] = await Promise.all([
        adminListUsers(),
        adminListOfferings(),
      ]);
      return { users, offerings };
    },
    "admin-dashboard",
    "Não foi possível carregar o painel.",
  );

  const users = overview.data?.users ?? [];
  const offerings = overview.data?.offerings ?? [];
  const openClasses = offerings.filter(
    (offering) => offering.visible_to_enroll && isFuture(offering.end_date),
  ).length;
  const members = offerings.reduce(
    (total, offering) => total + offering.member_count,
    0,
  );

  return (
    <div className="space-y-6">
      <PageHeader
        title="Painel"
        description="Visão geral da plataforma a partir das contas e turmas."
      />

      <PreviewControls />

      {overview.loading ? (
        <LoadingState />
      ) : overview.error ? (
        <ErrorState description={overview.error} onRetry={overview.reload} />
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <StatCard
              icon={UsersIcon}
              label="Usuários"
              value={users.length}
              hint="Contas carregadas"
            />
            <StatCard
              icon={GraduationCapIcon}
              label="Turmas"
              value={offerings.length}
              hint="Turmas carregadas"
              tone="info"
            />
            <StatCard
              icon={BookOpenIcon}
              label="Turmas abertas"
              value={openClasses}
              hint="Matrícula aberta e prazo futuro"
              tone="success"
            />
            <StatCard
              icon={TicketIcon}
              label="Matrículas"
              value={members}
              hint="Somadas nas turmas carregadas"
              tone="warning"
            />
          </div>

          <Alert variant="info">
            <InfoIcon />
            <AlertDescription>
              Os números refletem apenas a primeira página de cada listagem (até{" "}
              {ADMIN_PAGE_LIMIT} registros), não o total da plataforma. Use as
              telas de Usuários e Turmas para consultar todos os cadastros.
            </AlertDescription>
          </Alert>
        </>
      )}
    </div>
  );
}
