import { useState } from "react";

import {
  BellRingIcon,
  CheckIcon,
  InboxIcon,
  RefreshCwIcon,
} from "lucide-react";

import { PageHeader } from "@/components/app/PageHeader";
import { Section } from "@/components/app/Section";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useAsync } from "@/hooks/use-async";
import { alertDetailLine } from "@/lib/alerts";
import {
  ALERT_PAGE_LIMIT,
  adminAcknowledgeAlert,
  adminListAlerts,
  type SystemAlert,
} from "@/lib/api";
import { errorMessage } from "@/lib/errors";
import { formatDateTime } from "@/lib/format";

/**
 * The platform's operational alerts. They are raised by the judge when a run
 * cannot be completed for an infrastructure reason, and every new alert is also
 * emailed to the administrators; this page is the durable record, where an alert
 * is acknowledged once somebody has dealt with it.
 */
export function AdminAlertsPage() {
  const [onlyOpen, setOnlyOpen] = useState(true);
  const [busyId, setBusyId] = useState<number | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const list = useAsync(
    () => adminListAlerts({ onlyOpen }),
    `admin-alerts-${onlyOpen ? "open" : "all"}`,
    "Não foi possível carregar os alertas.",
  );

  const alerts = list.data?.alerts ?? [];
  const openCount = list.data?.open_count ?? 0;

  async function handleAcknowledge(alert: SystemAlert) {
    setBusyId(alert.id);
    setNotice(null);
    setActionError(null);
    try {
      await adminAcknowledgeAlert(alert.id);
      setNotice(`Alerta ${alert.code} reconhecido.`);
      list.reload();
    } catch (error) {
      setActionError(
        errorMessage(error, "Não foi possível reconhecer o alerta."),
      );
    } finally {
      setBusyId(null);
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Alertas"
        description="Falhas de infraestrutura registradas pela plataforma. Cada alerta novo também é enviado por e-mail para todos os administradores."
        actions={
          <>
            <Button
              variant={onlyOpen ? "default" : "outline"}
              size="sm"
              aria-pressed={onlyOpen}
              onClick={() => {
                setOnlyOpen((current) => !current);
              }}
            >
              <InboxIcon />
              {onlyOpen ? "Somente abertos" : "Mostrar todos"}
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={list.loading}
              onClick={list.reload}
            >
              <RefreshCwIcon
                className={list.loading ? "animate-spin" : undefined}
              />
              Atualizar
            </Button>
          </>
        }
      />

      {notice ? (
        <Alert>
          <CheckIcon />
          <AlertDescription>{notice}</AlertDescription>
        </Alert>
      ) : null}

      {actionError ? (
        <Alert variant="destructive">
          <AlertDescription>{actionError}</AlertDescription>
        </Alert>
      ) : null}

      <Section
        title={
          <span className="flex items-center gap-2">
            {onlyOpen ? "Alertas abertos" : "Todos os alertas"}
            {openCount > 0 ? (
              <Badge variant={onlyOpen ? "warning" : "secondary"}>
                {openCount}
              </Badge>
            ) : null}
          </span>
        }
        description="Um alerta aberto é uma falha que ninguém revisou ainda. Reconheça-o depois de tratar a causa."
      >
        {list.loading && !list.data ? (
          <LoadingState label="Carregando alertas…" />
        ) : list.error ? (
          <ErrorState description={list.error} onRetry={list.reload} />
        ) : alerts.length === 0 ? (
          <EmptyState
            icon={BellRingIcon}
            title={
              onlyOpen ? "Nenhum alerta aberto" : "Nenhum alerta registrado"
            }
            description={
              onlyOpen
                ? "Tudo certo: não há falhas de infraestrutura pendentes."
                : "A plataforma ainda não registrou nenhuma falha de infraestrutura."
            }
          />
        ) : (
          <>
            <Table aria-label="Alertas operacionais">
              <TableHeader>
                <TableRow>
                  <TableHead className="pl-4">Severidade</TableHead>
                  <TableHead>Código</TableHead>
                  <TableHead>Mensagem</TableHead>
                  <TableHead className="text-center">Submissão</TableHead>
                  <TableHead>Quando</TableHead>
                  <TableHead className="pr-4 text-right">Ação</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {alerts.map((alert) => (
                  <TableRow key={alert.id}>
                    <TableCell className="pl-4 align-top">
                      <SeverityBadge severity={alert.severity} />
                    </TableCell>
                    <TableCell className="align-top">
                      <code className="bg-muted rounded px-1.5 py-0.5 font-mono text-xs">
                        {alert.code}
                      </code>
                    </TableCell>
                    <TableCell className="max-w-md align-top">
                      <p className="leading-snug">{alert.message}</p>
                      {alert.details ? (
                        <p className="text-muted-foreground mt-1 font-mono text-[11px] break-words">
                          {alertDetailLine(alert.details)}
                        </p>
                      ) : null}
                      {alert.acknowledged_at ? (
                        <p className="text-muted-foreground mt-1 text-[11px]">
                          Reconhecido
                          {alert.acknowledged_by_name
                            ? ` por ${alert.acknowledged_by_name}`
                            : ""}{" "}
                          em {formatDateTime(alert.acknowledged_at)}
                        </p>
                      ) : null}
                    </TableCell>
                    <TableCell className="text-center align-top tabular-nums">
                      {alert.commit_id ?? "—"}
                    </TableCell>
                    <TableCell className="align-top whitespace-nowrap">
                      {formatDateTime(alert.created_at)}
                    </TableCell>
                    <TableCell className="pr-4 text-right align-top">
                      {alert.acknowledged_at ? (
                        <span className="text-muted-foreground text-xs">
                          Reconhecido
                        </span>
                      ) : (
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={busyId === alert.id}
                          onClick={() => {
                            void handleAcknowledge(alert);
                          }}
                        >
                          <CheckIcon />
                          Reconhecer
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            {alerts.length >= ALERT_PAGE_LIMIT ? (
              <p className="text-muted-foreground px-4 pt-3 text-xs">
                Mostrando os {ALERT_PAGE_LIMIT} alertas mais recentes. Reconheça
                os mais antigos para vê-los sair da lista.
              </p>
            ) : null}
          </>
        )}
      </Section>
    </div>
  );
}

/** The severity badge, with the same palette as the submission statuses. */
function SeverityBadge({ severity }: { severity: SystemAlert["severity"] }) {
  if (severity === "critical") {
    return <Badge variant="destructive">Crítico</Badge>;
  }

  return <Badge variant="warning">Aviso</Badge>;
}
