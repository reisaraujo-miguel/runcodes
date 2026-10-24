import {
  CheckCircle2Icon,
  ChevronDownIcon,
  CircleAlertIcon,
  CircleXIcon,
  DownloadIcon,
  HistoryIcon,
  ListChecksIcon,
  Loader2Icon,
  LockIcon,
} from "lucide-react";
import type { ReactNode } from "react";

import { StatusBadge } from "@/components/app/StatusBadge";
import type { TestCase } from "@/lib/api/exercises";
import type {
  CaseResultStatus,
  SubmissionArtifact,
  SubmissionStatusEvent,
} from "@/lib/api/submissions";
import { submissionOutputUrl } from "@/lib/api/submissions";
import { formatBytes, formatCpuTime } from "@/lib/format";
import { caseStatus, commitStatus } from "@/lib/submission-status";
import type { CaseResultView } from "@/lib/submission-view";
import { cn } from "@/lib/utils";

function formatEventTime(at: string): string {
  const date = new Date(at);
  if (Number.isNaN(date.getTime())) return at;
  return date.toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

export function CaseChip({
  id,
  status,
  active,
  onSelect,
}: {
  id: number;
  status?: CaseResultStatus;
  active: boolean;
  onSelect: () => void;
}) {
  const value = status ? caseStatus(status) : null;
  const icon =
    status === "correct" ? (
      <CheckCircle2Icon className="text-success size-3.5" aria-hidden />
    ) : status === "killed_with_signal" ? (
      <CircleXIcon className="text-destructive size-3.5" aria-hidden />
    ) : status === "bad_formatted_output" ? (
      <CircleAlertIcon className="text-warning size-3.5" aria-hidden />
    ) : null;

  return (
    <button
      type="button"
      data-case-id={id}
      aria-pressed={active}
      onClick={onSelect}
      className={cn(
        "focus-visible:ring-ring/50 inline-flex h-7 shrink-0 items-center gap-1.5 rounded-md border px-2 text-xs font-medium tabular-nums transition-colors outline-none focus-visible:ring-2",
        active
          ? "border-primary/50 bg-primary/10 text-primary"
          : "text-muted-foreground hover:bg-muted hover:text-foreground border-transparent",
      )}
    >
      Caso {String(id)}
      {icon}
      {value ? <span className="sr-only">{value.label}</span> : null}
    </button>
  );
}

/** Compact per-case completion bar. */
export function CaseProgress({
  results,
  total,
  correct,
}: {
  results: CaseResultView[];
  total: number;
  correct: number;
}) {
  const pendingCases = Math.max(0, total - results.length);
  return (
    <div
      role="img"
      aria-label={`${String(correct)} de ${String(total)} casos corretos`}
      className="flex shrink-0 items-center gap-2"
      title={`${String(correct)} de ${String(total)} casos corretos`}
    >
      <div
        aria-hidden
        className="bg-muted flex h-1.5 w-12 items-center gap-px overflow-hidden rounded-full sm:w-16"
      >
        {results.map((result) => (
          <span
            key={result.testCaseId}
            className={cn(
              "h-full min-w-0 flex-1 basis-0",
              result.status === "correct" ? "bg-success" : "bg-destructive",
            )}
          />
        ))}
        {pendingCases > 0 ? (
          <span
            style={{ flexGrow: pendingCases }}
            className="bg-muted-foreground/25 h-full min-w-0 basis-0"
          />
        ) : null}
      </div>
      <span className="text-muted-foreground text-[11px] tabular-nums">
        {String(correct)}/{String(total)}
      </span>
    </div>
  );
}

/** A labeled monospace block; `value === null` renders the lock state. */
function CaseField({
  label,
  value,
  unavailable,
  tone,
  className,
}: {
  label: string;
  value: string | null;
  unavailable: string;
  tone?: "danger";
  className?: string;
}) {
  return (
    <div className={cn("min-w-0", className)}>
      <p className="text-muted-foreground mb-1 text-xs font-medium">{label}</p>
      {value === null ? (
        <div className="border-border text-muted-foreground flex min-h-9 items-start gap-1.5 rounded-md border border-dashed px-2.5 py-2 text-xs">
          <LockIcon className="mt-px size-3.5 shrink-0" aria-hidden />
          <span>{unavailable}</span>
        </div>
      ) : (
        <pre
          className={cn(
            "border-border bg-muted/50 max-h-40 overflow-auto rounded-md border px-2.5 py-2 font-mono text-xs leading-relaxed wrap-break-word whitespace-pre-wrap",
            tone === "danger" &&
              "border-destructive/30 bg-destructive/10 text-destructive",
          )}
        >
          {value === "" ? "(vazio)" : value}
        </pre>
      )}
    </div>
  );
}

/**
 * The released test-case inputs plus, when the current submission has a result
 * for this case, its status, resource usage and program output.
 */
export function CaseDetail({
  testCase,
  result,
  pending,
  hasSubmission,
}: {
  testCase: TestCase | undefined;
  result: CaseResultView | undefined;
  pending: boolean;
  hasSubmission: boolean;
}) {
  const inputReleased =
    testCase?.show_input === true && testCase.input_type === "text";
  const expectedReleased =
    testCase?.show_expected_output === true &&
    testCase.expected_output_type === "text";
  const outputReleased = testCase?.show_user_output ?? true;
  const outputAvailable =
    result !== undefined &&
    outputReleased &&
    (result.userOutputType === "text" || result.userOutput !== "");

  return (
    <div className="space-y-2">
      {result ? (
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <StatusBadge value={caseStatus(result.status)} />
          <span className="text-muted-foreground text-[11px] tabular-nums">
            CPU {formatCpuTime(result.cpuTime)}
          </span>
          <span className="text-muted-foreground text-[11px] tabular-nums">
            Memória {formatBytes(result.memUsage)}
          </span>
          {result.statusMessage ? (
            <span className="text-muted-foreground min-w-0 text-xs">
              {result.statusMessage}
            </span>
          ) : null}
        </div>
      ) : null}

      <div className="grid gap-2 sm:grid-cols-2">
        <CaseField
          label="Entrada"
          value={inputReleased ? testCase.input : null}
          unavailable={
            !testCase
              ? "Caso não disponível para consulta."
              : testCase.show_input
                ? "Entrada em arquivo, não exibida aqui."
                : "Entrada não liberada para consulta."
          }
        />
        <CaseField
          label="Saída esperada"
          value={expectedReleased ? testCase.expected_output : null}
          unavailable={
            !testCase
              ? "Caso não disponível para consulta."
              : testCase.show_expected_output
                ? "Saída em arquivo, não exibida aqui."
                : "Saída esperada não liberada para consulta."
          }
        />
      </div>

      {result ? (
        <CaseField
          label="Saída do programa"
          value={outputAvailable ? result.userOutput : null}
          unavailable={
            outputReleased
              ? "Saída em arquivo, não exibida aqui."
              : "Saída do programa não liberada para consulta."
          }
          tone={result.status === "correct" ? undefined : "danger"}
        />
      ) : (
        <p className="text-muted-foreground flex items-start gap-1.5 text-xs">
          {pending ? (
            <Loader2Icon
              className="mt-px size-3.5 shrink-0 animate-spin"
              aria-hidden
            />
          ) : null}
          <span>
            {pending
              ? "A correção está em andamento. A saída deste caso aparece quando ele terminar."
              : hasSubmission
                ? "Este caso não foi executado nesta submissão."
                : "Envie a sua solução para ver a saída deste caso."}
          </span>
        </p>
      )}

      {result?.errorMessage ? (
        <pre className="border-destructive/30 bg-destructive/10 text-destructive max-h-40 overflow-auto rounded-md border px-2.5 py-2 font-mono text-xs leading-relaxed wrap-break-word whitespace-pre-wrap">
          {result.errorMessage}
        </pre>
      ) : null}

      <p className="text-muted-foreground text-[11px]">
        O professor escolhe quais casos e detalhes ficam disponíveis para
        consulta.
      </p>
    </div>
  );
}

export function NoCasesState() {
  return (
    <div className="text-muted-foreground flex min-h-24 flex-col items-center justify-center gap-1.5 py-4 text-center">
      <ListChecksIcon className="size-5" aria-hidden />
      <p className="text-foreground text-sm font-medium">
        Nenhum caso liberado
      </p>
      <p className="max-w-xs text-xs">
        O professor não liberou casos de teste para consulta.
      </p>
    </div>
  );
}

export function PendingState() {
  return (
    <div
      role="status"
      className="text-muted-foreground flex min-h-24 flex-col items-center justify-center gap-2 py-4 text-center"
    >
      <Loader2Icon className="size-4 animate-spin" aria-hidden />
      <p className="text-xs">
        A correção está em andamento. Os casos aparecem aqui conforme terminam.
      </p>
    </div>
  );
}

export function CompilationError({ message }: { message: string }) {
  return (
    <div className="space-y-1.5">
      <p className="text-destructive flex items-center gap-1.5 text-xs font-medium">
        <CircleAlertIcon className="size-3.5 shrink-0" aria-hidden />
        Erro de compilação
      </p>
      <pre className="border-destructive/30 bg-destructive/10 text-destructive max-h-40 overflow-auto rounded-md border px-2.5 py-2 font-mono text-xs leading-relaxed wrap-break-word whitespace-pre-wrap">
        {message}
      </pre>
    </div>
  );
}

export function InlineNotice({ children }: { children: ReactNode }) {
  return (
    <p
      role="alert"
      className="border-destructive/30 bg-destructive/10 text-destructive flex items-start gap-1.5 rounded-md border px-2.5 py-1.5 text-xs"
    >
      <CircleAlertIcon className="mt-px size-3.5 shrink-0" aria-hidden />
      <span>{children}</span>
    </p>
  );
}

export function StreamError({ message }: { message: string }) {
  return (
    <div className="border-destructive/30 bg-destructive/10 rounded-md border px-2.5 py-1.5">
      <p className="text-destructive flex items-center gap-1.5 text-xs font-medium">
        <CircleAlertIcon className="size-3.5 shrink-0" aria-hidden />
        Falha na transmissão dos resultados
      </p>
      <details className="mt-1">
        <summary className="text-muted-foreground hover:text-foreground cursor-pointer text-[11px] select-none">
          Ver detalhes
        </summary>
        <p className="text-muted-foreground mt-1 text-[11px] wrap-break-word">
          {message} Recarregue a página para ver o estado atual.
        </p>
      </details>
    </div>
  );
}

export function Artifacts({
  commitId,
  artifacts,
}: {
  commitId: number;
  artifacts: SubmissionArtifact[];
}) {
  const href = submissionOutputUrl(commitId);

  return (
    <div className="border-t pt-2">
      <p className="text-muted-foreground mb-1 text-[11px] font-medium">
        Arquivos gerados
      </p>
      <ul className="flex flex-wrap gap-x-4 gap-y-1">
        {artifacts.map((artifact) => (
          <li key={artifact.kind}>
            <a
              href={href}
              className="text-primary inline-flex items-center gap-1.5 text-xs hover:underline"
            >
              <DownloadIcon className="size-3.5" aria-hidden />
              {artifact.kind === "output" ? "Arquivo de saída" : artifact.kind}
            </a>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function CompilationMessage({ message }: { message: string }) {
  return (
    <details className="group border-t pt-2">
      <summary className="text-muted-foreground hover:text-foreground flex cursor-pointer list-none items-center gap-1.5 text-[11px] select-none [&::-webkit-details-marker]:hidden">
        <ChevronDownIcon
          className="size-3 transition-transform group-open:rotate-180"
          aria-hidden
        />
        Mensagem da compilação
      </summary>
      <pre className="bg-muted mt-2 max-h-40 overflow-auto rounded-md px-2.5 py-2 font-mono text-xs leading-relaxed wrap-break-word whitespace-pre-wrap">
        {message}
      </pre>
    </details>
  );
}

export function RunHistory({ history }: { history: SubmissionStatusEvent[] }) {
  return (
    <details className="group border-t pt-2">
      <summary className="text-muted-foreground hover:text-foreground flex cursor-pointer list-none items-center gap-1.5 text-[11px] select-none [&::-webkit-details-marker]:hidden">
        <HistoryIcon className="size-3.5" aria-hidden />
        Andamento da correção
        <ChevronDownIcon
          className="size-3 transition-transform group-open:rotate-180"
          aria-hidden
        />
      </summary>
      <ol className="mt-2 space-y-1">
        {history.map((event) => (
          <li
            key={event.seq}
            className="flex items-center justify-between gap-3 text-[11px]"
          >
            <span className="text-muted-foreground">
              {commitStatus(event.status).label}
            </span>
            <span className="text-muted-foreground font-mono tabular-nums">
              {formatEventTime(event.at)}
            </span>
          </li>
        ))}
      </ol>
    </details>
  );
}
