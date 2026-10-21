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
  PanelBottomCloseIcon,
  PanelBottomOpenIcon,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { LoadingState } from "@/components/app/states";
import { StatusBadge } from "@/components/app/StatusBadge";
import { Button } from "@/components/ui/button";
import type { TestCase } from "@/lib/api/exercises";
import type {
  CaseResultStatus,
  CommitStatus,
  SubmissionArtifact,
  SubmissionStatusEvent,
} from "@/lib/api/submissions";
import { submissionOutputUrl } from "@/lib/api/submissions";
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
import { cn } from "@/lib/utils";

interface CaseEntry {
  id: number;
  testCase: TestCase | undefined;
  result: CaseResultView | undefined;
}

export interface ResultsPanelProps {
  commitId: number | null;
  status: CommitStatus;
  history: SubmissionStatusEvent[];
  compilation: CompilationInfo | null;
  results: CaseResultView[];
  final: FinalSummary | null;
  streamError: string | null;
  artifacts: SubmissionArtifact[];
  testCases: TestCase[];
  testCasesLoading: boolean;
  testCasesError: string | null;
  /** True while the page is still checking for an existing submission. */
  historyLoading?: boolean;
  /** Why the existing submissions could not be checked, when that failed. */
  historyError?: string | null;
  /** The primary submission action, rendered in the panel footer. */
  actions?: ReactNode;
  /** Submission error shown next to the action. */
  actionError?: string | null;
  collapsed?: boolean;
  onToggleCollapsed?: () => void;
  className?: string;
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

/**
 * The bottom half of the submission workspace. One panel replaces the old
 * stack of result cards: every test case is listed in the strip, and its detail
 * view carries both the released input/expected output and, once a submission
 * exists, the program's result for that case.
 */
export function ResultsPanel({
  commitId,
  status,
  history,
  compilation,
  results,
  final,
  streamError,
  artifacts,
  testCases,
  testCasesLoading,
  testCasesError,
  historyLoading = false,
  historyError = null,
  actions,
  actionError,
  collapsed = false,
  onToggleCollapsed,
  className,
}: ResultsPanelProps) {
  const [caseChoice, setCaseChoice] = useState<{
    commitId: number | null;
    id: number;
  } | null>(null);
  // A selection survives as long as the same run is shown; a new submission
  // falls back to the first failing case again.
  const selectedCaseId =
    caseChoice?.commitId === commitId ? caseChoice.id : null;

  const resultById = useMemo(
    () => new Map(results.map((result) => [result.testCaseId, result])),
    [results],
  );
  const cases = useMemo<CaseEntry[]>(() => {
    const entries: CaseEntry[] = testCases.map((testCase) => ({
      id: testCase.id,
      testCase,
      result: resultById.get(testCase.id),
    }));

    // A result can outlive its test case (for example, a case removed after the
    // run started). Keep it selectable instead of silently dropping it.
    const known = new Set(testCases.map((testCase) => testCase.id));
    for (const result of results) {
      if (!known.has(result.testCaseId)) {
        entries.push({
          id: result.testCaseId,
          testCase: undefined,
          result,
        });
      }
    }

    return entries;
  }, [testCases, results, resultById]);

  const fallbackCaseId =
    results.find((result) => result.status !== "correct")?.testCaseId ??
    results[0]?.testCaseId ??
    testCases[0]?.id ??
    null;
  const activeCaseId =
    selectedCaseId !== null &&
    cases.some((entry) => entry.id === selectedCaseId)
      ? selectedCaseId
      : fallbackCaseId;
  const activeCase = cases.find((entry) => entry.id === activeCaseId) ?? null;
  const caseStripRef = useRef<HTMLDivElement>(null);

  // Keep the selected case visible when the strip is scrolled horizontally.
  // Only the strip scrolls: the page itself must not jump to the results.
  useEffect(() => {
    const strip = caseStripRef.current;
    const selectedId = activeCase?.id ?? null;
    if (!strip || selectedId === null) return;
    const chip = strip.querySelector<HTMLElement>(
      `[data-case-id="${String(selectedId)}"]`,
    );
    if (!chip) return;
    const stripRect = strip.getBoundingClientRect();
    const chipRect = chip.getBoundingClientRect();
    if (chipRect.left < stripRect.left) {
      strip.scrollLeft -= stripRect.left - chipRect.left;
    } else if (chipRect.right > stripRect.right) {
      strip.scrollLeft += chipRect.right - stripRect.right;
    }
  }, [activeCase?.id, cases.length]);

  const pending =
    commitId !== null && final === null && isPendingStatus(status);
  const overall = final ? finishedStatus(final.status) : commitStatus(status);
  const totalCases = cases.length;
  const correctCases =
    final?.numCorrectCases ??
    results.filter((result) => result.status === "correct").length;
  const hasSubmission = commitId !== null;

  const liveMessage =
    commitId === null
      ? ""
      : `Submissão ${String(commitId)}: ${overall.label}.${
          final
            ? ` Nota ${final.score.toFixed(2)}, ${String(final.numCorrectCases)} casos corretos.`
            : ""
        }`;

  return (
    <section
      aria-busy={pending}
      aria-labelledby="results-panel-title"
      className={cn(
        "bg-card flex min-h-0 min-w-0 flex-col overflow-hidden rounded-xl border",
        className,
      )}
    >
      <div className="flex h-10 shrink-0 items-center gap-2 border-b px-3">
        <h2
          id="results-panel-title"
          className="text-foreground text-xs font-semibold"
        >
          Resultados
        </h2>
        <div className="ml-auto flex items-center gap-1">
          {hasSubmission ? (
            <span className="text-muted-foreground hidden text-[11px] tabular-nums sm:inline">
              Submissão #{String(commitId)}
            </span>
          ) : null}
          {onToggleCollapsed ? (
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              className="hidden xl:inline-flex"
              aria-label={
                collapsed ? "Expandir resultados" : "Recolher resultados"
              }
              title={collapsed ? "Expandir resultados" : "Recolher resultados"}
              onClick={onToggleCollapsed}
            >
              {collapsed ? (
                <PanelBottomOpenIcon className="size-4.5" />
              ) : (
                <PanelBottomCloseIcon className="size-4.5" />
              )}
            </Button>
          ) : null}
        </div>
      </div>

      <p className="sr-only" role="status" aria-live="polite">
        {liveMessage}
      </p>

      {totalCases > 0 ? (
        <div
          className={cn(
            "flex h-9 shrink-0 items-center gap-3 border-b px-2",
            collapsed && "xl:hidden",
          )}
        >
          <div
            ref={caseStripRef}
            role="group"
            aria-label="Casos de teste"
            className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto scrollbar-none [&::-webkit-scrollbar]:hidden"
          >
            {cases.map((entry) => (
              <CaseChip
                key={entry.id}
                id={entry.id}
                status={entry.result?.status}
                active={activeCaseId === entry.id}
                onSelect={() => {
                  setCaseChoice({ commitId, id: entry.id });
                }}
              />
            ))}
          </div>
          {hasSubmission ? (
            <CaseProgress
              results={results}
              total={totalCases}
              correct={correctCases}
            />
          ) : (
            <span className="text-muted-foreground shrink-0 text-[11px] tabular-nums">
              {String(totalCases)} {totalCases === 1 ? "caso" : "casos"}
            </span>
          )}
        </div>
      ) : null}

      <div
        className={cn(
          "max-h-[60vh] min-h-0 overflow-y-auto p-2.5 xl:max-h-none xl:flex-1",
          collapsed && "xl:hidden",
        )}
      >
        <div className="space-y-3">
          {streamError ? <StreamError message={streamError} /> : null}
          {final?.status === "server_error" ? (
            <InlineNotice>
              O serviço de correção falhou ao processar a sua submissão. Tente
              enviar novamente em alguns instantes.
            </InlineNotice>
          ) : null}
          {final?.status === "timeout" ? (
            <InlineNotice>
              A sua submissão excedeu o tempo máximo de execução.
            </InlineNotice>
          ) : null}

          {compilation?.error ? (
            <CompilationError message={compilation.error} />
          ) : null}

          {historyError ? (
            <p className="text-destructive text-xs">{historyError}</p>
          ) : null}

          {testCasesError ? (
            <p className="text-destructive text-xs">
              Não foi possível carregar os casos de teste: {testCasesError}
            </p>
          ) : null}

          {testCasesLoading && totalCases === 0 ? (
            <LoadingState label="Carregando casos de teste…" className="py-8" />
          ) : activeCase ? (
            <CaseDetail
              testCase={activeCase.testCase}
              result={activeCase.result}
              pending={pending}
              hasSubmission={hasSubmission}
            />
          ) : pending ? (
            <PendingState />
          ) : testCasesError ? null : (
            <NoCasesState />
          )}

          {commitId !== null && artifacts.length > 0 ? (
            <Artifacts commitId={commitId} artifacts={artifacts} />
          ) : null}

          {compilation?.message && !compilation.error ? (
            <CompilationMessage message={compilation.message} />
          ) : null}

          {history.length > 0 ? <RunHistory history={history} /> : null}
        </div>
      </div>

      <div className="shrink-0 border-t">
        {actionError ? (
          <p
            aria-live="polite"
            className="text-destructive px-2 pt-1.5 text-xs"
          >
            {actionError}
          </p>
        ) : null}
        <div className="flex h-10 items-center justify-between gap-3 px-2">
          <div className="flex min-w-0 items-center gap-2">
            {pending ? (
              <Loader2Icon
                className="text-muted-foreground size-3.5 animate-spin"
                aria-hidden
              />
            ) : null}
            {hasSubmission ? (
              <StatusBadge value={overall} />
            ) : historyLoading ? (
              <span className="text-muted-foreground flex items-center gap-1.5 text-[11px]">
                <Loader2Icon className="size-3 animate-spin" aria-hidden />
                Verificando submissões…
              </span>
            ) : (
              <span className="text-muted-foreground text-[11px]">
                {historyError
                  ? "Submissões anteriores indisponíveis"
                  : "Nenhum envio ainda"}
              </span>
            )}
            {final ? (
              <span className="flex items-baseline gap-1">
                <span className="text-muted-foreground text-[11px]">Nota</span>
                <span className="text-foreground text-xs font-semibold tabular-nums">
                  {final.score.toFixed(2)}
                </span>
              </span>
            ) : null}
          </div>
          {actions ? (
            <div className="flex shrink-0 items-center">{actions}</div>
          ) : null}
        </div>
      </div>
    </section>
  );
}

/** One selectable case in the strip above the case details. */
function CaseChip({
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
function CaseProgress({
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
function CaseDetail({
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

function NoCasesState() {
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

function PendingState() {
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

function CompilationError({ message }: { message: string }) {
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

function InlineNotice({ children }: { children: ReactNode }) {
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

function StreamError({ message }: { message: string }) {
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

function Artifacts({
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

function CompilationMessage({ message }: { message: string }) {
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

function RunHistory({ history }: { history: SubmissionStatusEvent[] }) {
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
