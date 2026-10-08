import {
  CheckCircle2Icon,
  CircleAlertIcon,
  ClockIcon,
  DownloadIcon,
  Loader2Icon,
} from "lucide-react";

import { SectionCard } from "@/components/app/SectionCard";
import { StatusBadge } from "@/components/app/StatusBadge";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Separator } from "@/components/ui/separator";
import type {
  CommitStatus,
  SubmissionArtifactEvent,
  SubmissionStatusEvent,
} from "@/lib/api/submissions";
import { formatBytes, formatCpuTime } from "@/lib/format";
import {
  caseStatus,
  commitStatus,
  finishedStatus,
  isPendingStatus,
} from "@/lib/submission-status";
import type {
  CaseResultView,
  CompilationInfo,
  FinalSummary,
} from "@/lib/submission-view";

/** Whether a case's user output may be shown to this viewer. */
export interface CaseVisibility {
  showUserOutput: boolean;
}

export interface LiveResultsProps {
  status: CommitStatus;
  history: SubmissionStatusEvent[];
  compilation: CompilationInfo | null;
  results: CaseResultView[];
  final: FinalSummary | null;
  streamError: string | null;
  artifacts?: SubmissionArtifactEvent[];
  visibility?: Map<number, CaseVisibility>;
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

/** Live judging results: timeline, compilation, per-case output and artifacts. */
export function LiveResults({
  status,
  history,
  compilation,
  results,
  final,
  streamError,
  artifacts = [],
  visibility,
}: LiveResultsProps) {
  const overall = final ? finishedStatus(final.status) : commitStatus(status);
  const pending = final === null && isPendingStatus(status);

  return (
    <div className="space-y-4">
      <SectionCard
        title={
          <span className="flex items-center gap-2">
            {pending ? (
              <Loader2Icon className="size-4 animate-spin" aria-hidden />
            ) : (
              <CheckCircle2Icon className="size-4 text-success" aria-hidden />
            )}
            Resultados da correção
            <StatusBadge value={overall} />
          </span>
        }
        description={
          pending
            ? "A sua submissão está sendo processada. Os resultados aparecem conforme ficam prontos."
            : "Correção finalizada."
        }
      >
        {final ? (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4 sm:max-w-sm">
              <div>
                <p className="text-muted-foreground text-sm">Nota</p>
                <p className="text-3xl font-semibold tabular-nums">
                  {final.score.toFixed(2)}
                </p>
              </div>
              <div>
                <p className="text-muted-foreground text-sm">Casos corretos</p>
                <p className="text-3xl font-semibold tabular-nums">
                  {String(final.numCorrectCases)}
                </p>
              </div>
            </div>

            {final.status === "server_error" ? (
              <Alert variant="destructive">
                <AlertDescription>
                  O serviço de correção falhou ao processar a sua submissão.
                  Tente enviar novamente em alguns instantes.
                </AlertDescription>
              </Alert>
            ) : null}
            {final.status === "timeout" ? (
              <Alert variant="destructive">
                <AlertDescription>
                  A sua submissão excedeu o tempo máximo de execução.
                </AlertDescription>
              </Alert>
            ) : null}
            {final.compilationError ? (
              <pre className="bg-destructive/10 text-destructive overflow-x-auto rounded-lg p-3 text-sm whitespace-pre-wrap">
                {final.compilationError}
              </pre>
            ) : null}
          </div>
        ) : (
          <p className="text-muted-foreground text-sm">
            Assim que a correção terminar, a nota e o número de casos corretos
            aparecem aqui.
          </p>
        )}
      </SectionCard>

      {streamError ? (
        <Alert variant="destructive">
          <CircleAlertIcon aria-hidden />
          <AlertTitle>Falha na transmissão de resultados</AlertTitle>
          <AlertDescription>{streamError}</AlertDescription>
        </Alert>
      ) : null}

      {compilation && (compilation.error || compilation.message) ? (
        <SectionCard title="Compilação">
          {compilation.error ? (
            <pre className="bg-destructive/10 text-destructive overflow-x-auto rounded-lg p-3 text-sm whitespace-pre-wrap">
              {compilation.error}
            </pre>
          ) : (
            <p className="text-sm">{compilation.message}</p>
          )}
        </SectionCard>
      ) : null}

      <SectionCard title="Progresso">
        {history.length === 0 ? (
          <p className="text-muted-foreground flex items-center gap-2 text-sm">
            <ClockIcon className="size-4" aria-hidden />
            Aguardando o início da correção…
          </p>
        ) : (
          <ol className="space-y-2">
            {history.map((event) => (
              <li
                key={event.seq}
                className="flex items-center justify-between gap-4 text-sm"
              >
                <StatusBadge value={commitStatus(event.status)} />
                <span className="text-muted-foreground font-mono text-xs">
                  {formatEventTime(event.at)}
                </span>
              </li>
            ))}
          </ol>
        )}
      </SectionCard>

      {results.length > 0 ? (
        <SectionCard
          title="Casos de teste"
          description={`${String(results.length)} caso(s) reportado(s)`}
        >
          <div className="space-y-4">
            {results.map((result, index) => {
              const value = caseStatus(result.status);
              const canShowOutput =
                visibility?.get(result.testCaseId)?.showUserOutput ?? true;
              return (
                <div key={result.testCaseId} className="space-y-2">
                  {index > 0 ? <Separator /> : null}
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <span className="font-medium">
                      Caso #{String(result.testCaseId)}
                    </span>
                    <span className="flex items-center gap-3">
                      <span className="text-muted-foreground text-xs">
                        CPU {formatCpuTime(result.cpuTime)} · Memória{" "}
                        {formatBytes(result.memUsage)}
                      </span>
                      <StatusBadge value={value} />
                    </span>
                  </div>
                  {result.statusMessage ? (
                    <p className="text-muted-foreground text-sm">
                      {result.statusMessage}
                    </p>
                  ) : null}
                  {result.errorMessage ? (
                    <pre className="bg-destructive/10 text-destructive overflow-x-auto rounded-lg p-3 text-xs whitespace-pre-wrap">
                      {result.errorMessage}
                    </pre>
                  ) : null}
                  {canShowOutput && result.userOutput ? (
                    <details>
                      <summary className="text-muted-foreground cursor-pointer text-sm select-none">
                        Ver saída do programa
                      </summary>
                      <pre className="bg-muted mt-2 max-h-72 overflow-auto rounded-lg p-3 text-xs whitespace-pre-wrap">
                        {result.userOutput}
                      </pre>
                    </details>
                  ) : null}
                </div>
              );
            })}
          </div>
        </SectionCard>
      ) : null}

      {artifacts.length > 0 ? (
        <SectionCard title="Artefatos gerados">
          <ul className="space-y-2">
            {artifacts.map((artifact) => (
              <li key={artifact.seq}>
                <a
                  href={artifact.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-primary inline-flex items-center gap-2 text-sm hover:underline"
                >
                  <DownloadIcon className="size-4" aria-hidden />
                  <span className="font-medium">{artifact.kind}</span>
                </a>
              </li>
            ))}
          </ul>
        </SectionCard>
      ) : null}
    </div>
  );
}
