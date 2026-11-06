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
  SubmissionOutputFile,
  SubmissionStatusEvent,
} from "@/lib/api/submissions";
import {
  submissionOutputFileUrl,
  submissionOutputUrl,
} from "@/lib/api/submissions";
import {
  artifactUnavailableMessage,
  type ArtifactStatus,
} from "@/lib/artifact-progress";
import { formatBytes, formatCpuTime } from "@/lib/format";
import { filesForTestCase } from "@/lib/output-files";
import {
  caseReasonLabel,
  caseStatus,
  commitStatus,
} from "@/lib/submission-status";
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
  number,
  status,
  running,
  active,
  onSelect,
}: {
  id: number;
  /** 1-based position in the exercise; null when the case was removed. */
  number: number | null;
  status?: CaseResultStatus;
  /** The suite is running and this is the case being executed. */
  running?: boolean;
  active: boolean;
  onSelect: () => void;
}) {
  const value = status ? caseStatus(status) : null;
  const icon =
    status === "correct" ? (
      <CheckCircle2Icon className="size-3.5 text-success" aria-hidden />
    ) : status === "killed_with_signal" ? (
      <CircleXIcon className="size-3.5 text-destructive" aria-hidden />
    ) : status === "bad_formatted_output" ? (
      <CircleAlertIcon className="size-3.5 text-warning" aria-hidden />
    ) : null;

  return (
    <button
      type="button"
      data-case-id={id}
      aria-pressed={active}
      onClick={onSelect}
      className={cn(
        "inline-flex h-7 shrink-0 items-center gap-1.5 rounded-md border px-2 text-xs font-medium tabular-nums transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
        active
          ? "border-primary/50 bg-primary/10 text-primary"
          : "border-transparent text-muted-foreground hover:bg-muted hover:text-foreground",
      )}
    >
      {number === null ? "Caso removido" : `Caso ${String(number)}`}
      {running && !status ? (
        <>
          <span
            className="size-2 animate-pulse rounded-full bg-primary"
            aria-hidden
          />
          <span className="sr-only">Executando</span>
        </>
      ) : null}
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
        className="flex h-1.5 w-12 items-center gap-px overflow-hidden rounded-full bg-muted sm:w-16"
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
            className="h-full min-w-0 basis-0 bg-muted-foreground/25"
          />
        ) : null}
      </div>
      <span className="text-[11px] text-muted-foreground tabular-nums">
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
      <p className="mb-1 text-xs font-medium text-muted-foreground">{label}</p>
      {value === null ? (
        <div className="flex min-h-9 items-start gap-1.5 rounded-md border border-dashed border-border px-2.5 py-2 text-xs text-muted-foreground">
          <LockIcon className="mt-px size-3.5 shrink-0" aria-hidden />
          <span>{unavailable}</span>
        </div>
      ) : (
        <pre
          className={cn(
            "max-h-40 overflow-auto rounded-md border border-border bg-muted/50 px-2.5 py-2 font-mono text-xs leading-relaxed wrap-break-word whitespace-pre-wrap",
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
          <span className="text-[11px] text-muted-foreground tabular-nums">
            CPU {formatCpuTime(result.cpuTime)}
          </span>
          <span className="text-[11px] text-muted-foreground tabular-nums">
            Memória {formatBytes(result.memUsage)}
          </span>
          {caseReasonLabel(result.reason) ? (
            <span className="min-w-0 text-xs font-medium text-destructive">
              {caseReasonLabel(result.reason)}
            </span>
          ) : null}
          {result.statusMessage ? (
            <span className="min-w-0 text-xs text-muted-foreground">
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
        <p className="flex items-start gap-1.5 text-xs text-muted-foreground">
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
        <pre className="max-h-40 overflow-auto rounded-md border border-destructive/30 bg-destructive/10 px-2.5 py-2 font-mono text-xs leading-relaxed wrap-break-word whitespace-pre-wrap text-destructive">
          {result.errorMessage}
        </pre>
      ) : null}

      <p className="text-[11px] text-muted-foreground">
        O professor escolhe quais casos e detalhes ficam disponíveis para
        consulta.
      </p>
    </div>
  );
}

export function NoCasesState() {
  return (
    <div className="flex min-h-24 flex-col items-center justify-center gap-1.5 py-4 text-center text-muted-foreground">
      <ListChecksIcon className="size-5" aria-hidden />
      <p className="text-sm font-medium text-foreground">
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
      className="flex min-h-24 flex-col items-center justify-center gap-2 py-4 text-center text-muted-foreground"
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
      <p className="flex items-center gap-1.5 text-xs font-medium text-destructive">
        <CircleAlertIcon className="size-3.5 shrink-0" aria-hidden />
        Erro de compilação
      </p>
      <pre className="max-h-40 overflow-auto rounded-md border border-destructive/30 bg-destructive/10 px-2.5 py-2 font-mono text-xs leading-relaxed wrap-break-word whitespace-pre-wrap text-destructive">
        {message}
      </pre>
    </div>
  );
}

export function InlineNotice({ children }: { children: ReactNode }) {
  return (
    <p
      role="alert"
      className="flex items-start gap-1.5 rounded-md border border-destructive/30 bg-destructive/10 px-2.5 py-1.5 text-xs text-destructive"
    >
      <CircleAlertIcon className="mt-px size-3.5 shrink-0" aria-hidden />
      <span>{children}</span>
    </p>
  );
}

export function StreamError({ message }: { message: string }) {
  return (
    <div className="rounded-md border border-destructive/30 bg-destructive/10 px-2.5 py-1.5">
      <p className="flex items-center gap-1.5 text-xs font-medium text-destructive">
        <CircleAlertIcon className="size-3.5 shrink-0" aria-hidden />
        Falha na transmissão dos resultados
      </p>
      <details className="mt-1">
        <summary className="cursor-pointer text-[11px] text-muted-foreground select-none hover:text-foreground">
          Ver detalhes
        </summary>
        <p className="mt-1 text-[11px] wrap-break-word text-muted-foreground">
          {message} Recarregue a página para ver o estado atual.
        </p>
      </details>
    </div>
  );
}

/**
 * Labels one generated file for display. The tab already identifies the case, so
 * only the path inside the case is shown: the judge's `test_<id>/` prefix would
 * repeat the tab's own title.
 */
function generatedFileLabel(filePath: string): string {
  const parts = filePath.split("/");
  const rest = parts.slice(1);
  if (rest.length === 0) return parts[0] ?? filePath;
  if (parts[0] === "outputfiles" || /^test_\d+$/.test(parts[0] ?? "")) {
    return rest.join("/");
  }

  return filePath;
}

/**
 * The files one test case generated, with a download per file. Each case has its
 * own tab, so a case shows only what it produced: a file's origin is the
 * `test_<case number>/` prefix the judge stored it under.
 *
 * The listing is pushed by the output-files WebSocket. When an upload fails, the
 * section shows why there is no download instead of a spinner.
 */
export function Artifacts({
  commitId,
  testCaseNumber,
  files,
  pending,
  status,
}: {
  commitId: number;
  /** The case's 1-based position in the exercise. */
  testCaseNumber: number;
  /** Every generated file of the commit, as pushed by the WebSocket. */
  files: readonly SubmissionOutputFile[];
  /** True while this case's files are still being uploaded. */
  pending: boolean;
  /** The upload outcome, when the judge has reported one. */
  status?: ArtifactStatus;
}) {
  const caseFiles = filesForTestCase(files, testCaseNumber);
  if (caseFiles.length === 0) {
    const unavailable = artifactUnavailableMessage(status);
    if (unavailable) {
      return (
        <div className="border-t pt-2">
          <p className="mb-1 text-[11px] font-medium text-muted-foreground">
            Arquivos gerados
          </p>
          <p
            className="text-xs wrap-break-word text-muted-foreground"
            aria-live="polite"
          >
            {unavailable}
          </p>
        </div>
      );
    }
    if (!pending) return null;

    return (
      <div className="border-t pt-2">
        <p className="mb-1 text-[11px] font-medium text-muted-foreground">
          Arquivos gerados
        </p>
        <p
          className="inline-flex items-center gap-1.5 text-xs text-muted-foreground"
          aria-live="polite"
        >
          <Loader2Icon className="size-3.5 animate-spin" aria-hidden />
          Aguardando o envio do arquivo…
        </p>
      </div>
    );
  }

  // The zip holds every case's files, so it is only offered when that is more
  // than this tab already lists.
  const hasOtherFiles = files.length > caseFiles.length;

  return (
    <div className="border-t pt-2">
      <p className="mb-1 text-[11px] font-medium text-muted-foreground">
        Arquivos gerados
      </p>
      <ul className="flex flex-col gap-1">
        {caseFiles.map((file) => (
          <li
            key={file.path}
            className="flex items-baseline justify-between gap-3"
          >
            <a
              href={submissionOutputFileUrl(commitId, file.path)}
              className="inline-flex min-w-0 items-center gap-1.5 text-xs text-primary hover:underline"
            >
              <DownloadIcon className="size-3.5 shrink-0" aria-hidden />
              <span className="truncate">{generatedFileLabel(file.path)}</span>
            </a>
            <span className="shrink-0 text-[11px] text-muted-foreground tabular-nums">
              {formatBytes(file.size)}
            </span>
          </li>
        ))}
      </ul>
      {hasOtherFiles ? (
        <a
          href={submissionOutputUrl(commitId)}
          className="mt-1.5 inline-flex items-center gap-1.5 text-[11px] text-muted-foreground hover:text-foreground hover:underline"
        >
          <DownloadIcon className="size-3" aria-hidden />
          Baixar os arquivos de todos os casos (.zip)
        </a>
      ) : null}
    </div>
  );
}

export function CompilationMessage({ message }: { message: string }) {
  return (
    <details className="group border-t pt-2">
      <summary className="flex cursor-pointer list-none items-center gap-1.5 text-[11px] text-muted-foreground select-none hover:text-foreground [&::-webkit-details-marker]:hidden">
        <ChevronDownIcon
          className="size-3 transition-transform group-open:rotate-180"
          aria-hidden
        />
        Mensagem da compilação
      </summary>
      <pre className="mt-2 max-h-40 overflow-auto rounded-md bg-muted px-2.5 py-2 font-mono text-xs leading-relaxed wrap-break-word whitespace-pre-wrap">
        {message}
      </pre>
    </details>
  );
}

export function RunHistory({ history }: { history: SubmissionStatusEvent[] }) {
  return (
    <details className="group border-t pt-2">
      <summary className="flex cursor-pointer list-none items-center gap-1.5 text-[11px] text-muted-foreground select-none hover:text-foreground [&::-webkit-details-marker]:hidden">
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
            <span className="font-mono text-muted-foreground tabular-nums">
              {formatEventTime(event.at)}
            </span>
          </li>
        ))}
      </ol>
    </details>
  );
}
