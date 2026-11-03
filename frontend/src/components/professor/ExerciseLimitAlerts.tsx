import { useState } from "react";

import { AlertTriangleIcon } from "lucide-react";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { useAsync } from "@/hooks/use-async";
import {
  acknowledgeExerciseLimitAlert,
  getExerciseLimitAlerts,
  type ExerciseLimitAlert,
} from "@/lib/api";
import { formatDateTime } from "@/lib/format";

const LIMIT_LABELS: Record<string, string> = {
  timeout: "tempo limite",
  cpu_time: "tempo de CPU",
  memory: "memória",
  stack: "pilha",
  file_size: "tamanho de arquivo",
};

function limitLabel(limit: string): string {
  return LIMIT_LABELS[limit] ?? limit;
}

/**
 * The professor's limit-failure warnings for one exercise: a language that keeps
 * failing submissions because of a limit is shown here until it is acknowledged.
 */
export function ExerciseLimitAlerts({ exerciseId }: { exerciseId: number }) {
  const alerts = useAsync(
    () => getExerciseLimitAlerts(exerciseId),
    `exercise-limit-alerts-${String(exerciseId)}`,
    "Não foi possível carregar os alertas de limites.",
  );
  const [acknowledging, setAcknowledging] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function acknowledge(alert: ExerciseLimitAlert) {
    setAcknowledging(alert.id);
    setError(null);
    try {
      await acknowledgeExerciseLimitAlert(exerciseId, alert.id);
      alerts.reload();
    } catch {
      setError("Não foi possível reconhecer o alerta.");
    } finally {
      setAcknowledging(null);
    }
  }

  const data = alerts.data ?? [];
  if (data.length === 0) return null;

  return (
    <div className="space-y-2">
      {error ? (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}
      {data.map((alert) => (
        <Alert key={alert.id} variant="warning">
          <AlertTriangleIcon />
          <AlertDescription>
            <div className="flex flex-wrap items-start justify-between gap-3">
              <p>
                {alert.failures} submissão(ões) de{" "}
                <strong>
                  {alert.language_name ?? "linguagem não identificada"}
                </strong>{" "}
                falharam por causa do limite de{" "}
                <strong>{limitLabel(alert.limit_name)}</strong> entre{" "}
                {formatDateTime(alert.window_started_at)} e{" "}
                {formatDateTime(alert.window_ended_at)}. Verifique se o limite
                está apertado demais na aba <strong>Limites</strong>.
              </p>
              <Button
                size="sm"
                variant="outline"
                disabled={acknowledging === alert.id}
                onClick={() => {
                  void acknowledge(alert);
                }}
              >
                {acknowledging === alert.id ? <Spinner /> : null}
                Reconhecer
              </Button>
            </div>
          </AlertDescription>
        </Alert>
      ))}
    </div>
  );
}
