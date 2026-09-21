import { CircleAlert, Clock, Loader2 } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import type {
  CommitStatus,
  SubmissionStatusEvent,
} from "@/lib/api/submissions";
import { formatBytes, formatCpuTime } from "@/lib/format";
import {
  caseStatus,
  commitStatus,
  finishedStatus,
  isPendingStatus,
  type StatusTone,
} from "@/lib/submission-status";
import type {
  CaseResultView,
  CompilationInfo,
  FinalSummary,
} from "@/lib/submission-view";

const TONE_VARIANT: Record<
  StatusTone,
  "secondary" | "destructive" | "success" | "warning" | "info"
> = {
  neutral: "secondary",
  info: "info",
  success: "success",
  warning: "warning",
  danger: "destructive",
};

interface LiveResultsProps {
  status: CommitStatus;
  history: SubmissionStatusEvent[];
  compilation: CompilationInfo | null;
  results: CaseResultView[];
  final: FinalSummary | null;
  streamError: string | null;
}

function formatEventTime(at: string): string {
  const date = new Date(at);
  if (Number.isNaN(date.getTime())) return at;
  return date.toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

/** Live judging results: timeline, compilation, per-case output and score. */
export function LiveResults({
  status,
  history,
  compilation,
  results,
  final,
  streamError,
}: LiveResultsProps) {
  const overall = final ? finishedStatus(final.status) : commitStatus(status);
  const pending = final === null && isPendingStatus(status);

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            {pending && <Loader2 className="size-4 animate-spin" />}
            {!pending && <Clock className="size-4 text-muted-foreground" />}
            Resultados da correção
            <Badge variant={TONE_VARIANT[overall.tone]}>{overall.label}</Badge>
          </CardTitle>
          <CardDescription>
            {pending
              ? "A sua submissão está a ser processada. Os resultados aparecem à medida que ficam prontos."
              : "Correção finalizada."}
          </CardDescription>
        </CardHeader>

        {final && (
          <CardContent className="space-y-3">
            <div className="flex flex-wrap items-baseline gap-x-6 gap-y-2">
              <div>
                <p className="text-muted-foreground text-sm">Nota</p>
                <p className="text-3xl font-semibold">
                  {final.score.toFixed(2)}
                </p>
              </div>
              <div>
                <p className="text-muted-foreground text-sm">Casos corretos</p>
                <p className="text-3xl font-semibold">
                  {String(final.numCorrectCases)}
                </p>
              </div>
            </div>
            {final.status === "server_error" && (
              <p className="text-destructive text-sm">
                O serviço de correção falhou ao processar a sua submissão. Tente
                enviar novamente em alguns instantes.
              </p>
            )}
            {final.status === "timeout" && (
              <p className="text-destructive text-sm">
                A sua submissão excedeu o tempo máximo de execução.
              </p>
            )}
            {final.compilationError && (
              <pre className="bg-destructive/10 text-destructive overflow-x-auto rounded-lg p-3 text-sm whitespace-pre-wrap">
                {final.compilationError}
              </pre>
            )}
          </CardContent>
        )}
      </Card>

      {streamError && (
        <Card>
          <CardHeader>
            <CardTitle className="text-destructive flex items-center gap-2">
              <CircleAlert className="size-4" />
              Falha na transmissão de resultados
            </CardTitle>
            <CardDescription>{streamError}</CardDescription>
          </CardHeader>
        </Card>
      )}

      {compilation && (compilation.error || compilation.message) && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Compilação</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {compilation.error ? (
              <pre className="bg-destructive/10 text-destructive overflow-x-auto rounded-lg p-3 text-sm whitespace-pre-wrap">
                {compilation.error}
              </pre>
            ) : (
              <p className="text-sm">{compilation.message}</p>
            )}
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Progresso</CardTitle>
        </CardHeader>
        <CardContent>
          {history.length === 0 ? (
            <p className="text-muted-foreground text-sm">
              A aguardar o início da correção…
            </p>
          ) : (
            <ol className="space-y-2">
              {history.map((event) => {
                const value = commitStatus(event.status);
                return (
                  <li
                    key={event.seq}
                    className="flex items-center justify-between gap-4 text-sm"
                  >
                    <Badge variant={TONE_VARIANT[value.tone]}>
                      {value.label}
                    </Badge>
                    <span className="text-muted-foreground font-mono text-xs">
                      {formatEventTime(event.at)}
                    </span>
                  </li>
                );
              })}
            </ol>
          )}
        </CardContent>
      </Card>

      {results.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Casos de teste</CardTitle>
            <CardDescription>
              {String(results.length)} caso(s) reportado(s)
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {results.map((result, index) => {
              const value = caseStatus(result.status);
              return (
                <div key={result.testCaseId} className="space-y-2">
                  {index > 0 && <Separator />}
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <span className="font-medium">
                      Caso #{String(result.testCaseId)}
                    </span>
                    <span className="flex items-center gap-3">
                      <span className="text-muted-foreground text-xs">
                        CPU {formatCpuTime(result.cpuTime)} · Memória{" "}
                        {formatBytes(result.memUsage)}
                      </span>
                      <Badge variant={TONE_VARIANT[value.tone]}>
                        {value.label}
                      </Badge>
                    </span>
                  </div>
                  {result.statusMessage && (
                    <p className="text-muted-foreground text-sm">
                      {result.statusMessage}
                    </p>
                  )}
                  {result.errorMessage && (
                    <pre className="bg-destructive/10 text-destructive overflow-x-auto rounded-lg p-3 text-xs whitespace-pre-wrap">
                      {result.errorMessage}
                    </pre>
                  )}
                  {result.userOutput && (
                    <details>
                      <summary className="text-muted-foreground cursor-pointer text-sm select-none">
                        Ver saída do programa
                      </summary>
                      <pre className="bg-muted mt-2 max-h-72 overflow-auto rounded-lg p-3 text-xs whitespace-pre-wrap">
                        {result.userOutput}
                      </pre>
                    </details>
                  )}
                </div>
              );
            })}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
