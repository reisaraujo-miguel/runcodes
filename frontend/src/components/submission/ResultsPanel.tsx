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
  PlayIcon,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { LoadingState } from "@/components/app/states";
import { StatusBadge } from "@/components/app/StatusBadge";
import { Button } from "@/components/ui/button";
import { TabBar, type TabBarItem } from "@/components/ui/tab-bar";
import type { TestCase } from "@/lib/api/exercises";
import type {
  CaseResultStatus,
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
import { cn } from "@/lib/utils";

type PanelTab = "results" | "cases";

export interface ResultsPanelProps {
  commitId: number | null;
  status: CommitStatus;
  history: SubmissionStatusEvent[];
  compilation: CompilationInfo | null;
  results: CaseResultView[];
  final: FinalSummary | null;
  streamError: string | null;
  artifacts: SubmissionArtifactEvent[];
  testCases: TestCase[];
  testCasesLoading: boolean;
  testCasesError: string | null;
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
 * stack of result cards: the submission status, case progress and case-by-case
 * input/output all stay visible right under the editor.
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
  actions,
  actionError,
  collapsed = false,
  onToggleCollapsed,
  className,
}: ResultsPanelProps) {
  const [tabChoice, setTabChoice] = useState<{
    commitId: number | null;
    tab: PanelTab;
  } | null>(null);
  // The tab and the selected case are keyed by the commit: starting a new run
  // brings the result view back without an effect that resets state.
  const tab: PanelTab =
    tabChoice?.commitId === commitId
      ? tabChoice.tab
      : commitId !== null
        ? "results"
        : "cases";

  const [selectedResult, setSelectedResult] = useState<{
    commitId: number | null;
    id: number;
  } | null>(null);
  const selectedResultId =
    selectedResult?.commitId === commitId ? selectedResult.id : null;
  const [selectedTestCaseId, setSelectedTestCaseId] = useState<number | null>(
    null,
  );

  const caseById = useMemo(
    () => new Map(testCases.map((testCase) => [testCase.id, testCase])),
    [testCases],
  );
  const fallbackResult = useMemo(
    () =>
      results.find((result) => result.status !== "correct") ??
      results[0] ??
      null,
    [results],
  );
  const activeResult =
    results.find((result) => result.testCaseId === selectedResultId) ??
    fallbackResult;
  const activeTestCase =
    testCases.find((testCase) => testCase.id === selectedTestCaseId) ??
    testCases[0] ??
    null;
  const activeResultId = activeResult?.testCaseId ?? null;
  const caseStripRef = useRef<HTMLDivElement>(null);

  // Keep the selected case visible when the strip is scrolled horizontally.
  // Only the strip scrolls: the page itself must not jump to the results.
  useEffect(() => {
    const strip = caseStripRef.current;
    const selectedId =
      tab === "results" ? activeResultId : (activeTestCase?.id ?? null);
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
  }, [activeResultId, activeTestCase?.id, tab]);

  const pending =
    commitId !== null && final === null && isPendingStatus(status);
  const overall = final ? finishedStatus(final.status) : commitStatus(status);
  const totalCases = Math.max(testCases.length, results.length);
  const correctCases =
    final?.numCorrectCases ??
    results.filter((result) => result.status === "correct").length;
  const showCaseBar =
    tab === "results" ? commitId !== null : testCases.length > 0;

  const tabs: TabBarItem<PanelTab>[] = [
    { value: "results", label: "Resultados" },
    { value: "cases", label: "Casos de teste" },
  ];

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
      className={cn(
        "bg-card flex min-h-0 min-w-0 flex-col overflow-hidden rounded-xl border",
        className,
      )}
    >
      <TabBar
        items={tabs}
        value={tab}
        onChange={(next) => {
          setTabChoice({ commitId, tab: next });
          // Choosing a tab is a request to see its content; expand the pane
          // when it is collapsed instead of silently changing a hidden tab.
          if (collapsed) onToggleCollapsed?.();
        }}
        label="Resultados da submissão"
        idPrefix="results"
        trailing={
          <div className="flex items-center gap-1">
            {commitId !== null ? (
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
                title={
                  collapsed ? "Expandir resultados" : "Recolher resultados"
                }
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
        }
      />

      <p className="sr-only" role="status" aria-live="polite">
        {liveMessage}
      </p>

      {showCaseBar ? (
        <div
          className={cn(
            "flex h-9 shrink-0 items-center gap-3 border-b px-2",
            collapsed && "xl:hidden",
          )}
        >
          <div
            ref={caseStripRef}
            role="group"
            aria-label={
              tab === "results" ? "Casos desta submissão" : "Casos de teste"
            }
            className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
          >
            {tab === "results"
              ? results.map((result) => (
                  <CaseChip
                    key={result.testCaseId}
                    id={result.testCaseId}
                    status={result.status}
                    active={activeResult?.testCaseId === result.testCaseId}
                    onSelect={() => {
                      setSelectedResult({
                        commitId,
                        id: result.testCaseId,
                      });
                    }}
                  />
                ))
              : testCases.map((testCase) => (
                  <CaseChip
                    key={testCase.id}
                    id={testCase.id}
                    active={activeTestCase?.id === testCase.id}
                    onSelect={() => {
                      setSelectedTestCaseId(testCase.id);
                    }}
                  />
                ))}
          </div>
          {tab === "results" && totalCases > 0 ? (
            <CaseProgress
              results={results}
              total={totalCases}
              correct={correctCases}
            />
          ) : tab === "cases" ? (
            <span className="text-muted-foreground shrink-0 text-[11px] tabular-nums">
              {String(testCases.length)}{" "}
              {testCases.length === 1 ? "caso" : "casos"}
            </span>
          ) : null}
        </div>
      ) : null}

      <div
        role="tabpanel"
        id={`results-panel-${tab}`}
        aria-labelledby={`results-tab-${tab}`}
        className={cn(
          "max-h-[60vh] min-h-0 overflow-y-auto p-2.5 xl:max-h-none xl:flex-1",
          collapsed && "xl:hidden",
        )}
      >
        {tab === "cases" ? (
          <TestCasesBody
            testCase={activeTestCase}
            loading={testCasesLoading}
            error={testCasesError}
          />
        ) : (
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

            {activeResult ? (
              <ResultDetail
                result={activeResult}
                testCase={caseById.get(activeResult.testCaseId)}
              />
            ) : compilation?.error ? null : pending ? (
              <PendingState />
            ) : commitId !== null ? (
              <p className="text-muted-foreground text-sm">
                Nenhum caso de teste foi executado nesta submissão.
              </p>
            ) : (
              <NoSubmissionState />
            )}

            {artifacts.length > 0 ? <Artifacts artifacts={artifacts} /> : null}

            {compilation?.message && !compilation.error ? (
              <CompilationMessage message={compilation.message} />
            ) : null}

            {history.length > 0 ? <RunHistory history={history} /> : null}
          </div>
        )}
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
            {commitId !== null ? (
              <StatusBadge value={overall} />
            ) : (
              <span className="text-muted-foreground text-[11px]">
                Nenhum envio ainda
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

/** A labelled monospace block; `value === null` renders the lock state. */
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
            "border-border bg-muted/50 max-h-40 overflow-auto rounded-md border px-2.5 py-2 font-mono text-xs leading-relaxed break-words whitespace-pre-wrap",
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

/** Input, expected output and the program output for the selected case. */
function ResultDetail({
  result,
  testCase,
}: {
  result: CaseResultView;
  testCase: TestCase | undefined;
}) {
  const inputReleased =
    testCase?.show_input === true && testCase.input_type === "text";
  const expectedReleased =
    testCase?.show_expected_output === true &&
    testCase.expected_output_type === "text";
  const outputReleased = testCase?.show_user_output ?? true;
  const outputAvailable =
    outputReleased &&
    (result.userOutputType === "text" || result.userOutput !== "");

  return (
    <div className="space-y-2">
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

      <div className="grid gap-2 sm:grid-cols-2">
        <CaseField
          label="Entrada"
          value={inputReleased ? testCase.input : null}
          unavailable={
            testCase?.show_input
              ? "Entrada em arquivo, não exibida aqui."
              : "Entrada não liberada para consulta."
          }
        />
        <CaseField
          label="Saída esperada"
          value={expectedReleased ? testCase.expected_output : null}
          unavailable={
            testCase?.show_expected_output
              ? "Saída em arquivo, não exibida aqui."
              : "Saída esperada não liberada para consulta."
          }
        />
      </div>

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

      {result.errorMessage ? (
        <pre className="border-destructive/30 bg-destructive/10 text-destructive max-h-40 overflow-auto rounded-md border px-2.5 py-2 font-mono text-xs leading-relaxed break-words whitespace-pre-wrap">
          {result.errorMessage}
        </pre>
      ) : null}
    </div>
  );
}

function TestCasesBody({
  testCase,
  loading,
  error,
}: {
  testCase: TestCase | null;
  loading: boolean;
  error: string | null;
}) {
  if (loading) {
    return <LoadingState label="Carregando casos de teste…" className="py-8" />;
  }
  if (error) {
    return (
      <p className="text-destructive text-xs">
        Não foi possível carregar os casos de teste: {error}
      </p>
    );
  }
  if (!testCase) {
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

  const showInput = testCase.show_input && testCase.input_type === "text";
  const showExpected =
    testCase.show_expected_output && testCase.expected_output_type === "text";

  return (
    <div className="space-y-3">
      <div className="grid gap-2 sm:grid-cols-2">
        <CaseField
          label="Entrada"
          value={showInput ? testCase.input : null}
          unavailable={
            testCase.show_input
              ? "Entrada em arquivo, não exibida aqui."
              : "Entrada não liberada para consulta."
          }
        />
        <CaseField
          label="Saída esperada"
          value={showExpected ? testCase.expected_output : null}
          unavailable={
            testCase.show_expected_output
              ? "Saída em arquivo, não exibida aqui."
              : "Saída esperada não liberada para consulta."
          }
        />
      </div>
      <p className="text-muted-foreground text-[11px]">
        O professor escolhe quais casos e detalhes ficam disponíveis para
        consulta.
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

function NoSubmissionState() {
  return (
    <div className="flex min-h-24 flex-col items-center justify-center gap-1.5 py-4 text-center">
      <PlayIcon className="text-muted-foreground size-5" aria-hidden />
      <p className="text-sm font-medium">Nenhum resultado ainda</p>
      <p className="text-muted-foreground max-w-xs text-xs">
        Envie a sua solução para ver a nota e a saída de cada caso de teste.
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
      <pre className="border-destructive/30 bg-destructive/10 text-destructive max-h-40 overflow-auto rounded-md border px-2.5 py-2 font-mono text-xs leading-relaxed break-words whitespace-pre-wrap">
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
        <p className="text-muted-foreground mt-1 text-[11px] break-words">
          {message} Recarregue a página para ver o estado atual.
        </p>
      </details>
    </div>
  );
}

function Artifacts({ artifacts }: { artifacts: SubmissionArtifactEvent[] }) {
  return (
    <div className="border-t pt-2">
      <p className="text-muted-foreground mb-1 text-[11px] font-medium">
        Arquivos gerados
      </p>
      <ul className="flex flex-wrap gap-x-4 gap-y-1">
        {artifacts.map((artifact) => (
          <li key={artifact.seq}>
            <a
              href={artifact.url}
              target="_blank"
              rel="noopener noreferrer"
              className="text-primary inline-flex items-center gap-1.5 text-xs hover:underline"
            >
              <DownloadIcon className="size-3.5" aria-hidden />
              {artifact.kind}
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
      <pre className="bg-muted mt-2 max-h-40 overflow-auto rounded-md px-2.5 py-2 font-mono text-xs leading-relaxed break-words whitespace-pre-wrap">
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
