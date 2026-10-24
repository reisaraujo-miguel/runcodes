import {
  Loader2Icon,
  PanelBottomCloseIcon,
  PanelBottomOpenIcon,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { LoadingState } from "@/components/app/states";
import { StatusBadge } from "@/components/app/StatusBadge";
import { Button } from "@/components/ui/button";
import type { TestCase } from "@/lib/api/exercises";
import type {
  CommitStatus,
  SubmissionArtifact,
  SubmissionStatusEvent,
} from "@/lib/api/submissions";
import {
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

import {
  Artifacts,
  CaseChip,
  CaseDetail,
  CaseProgress,
  CompilationError,
  CompilationMessage,
  InlineNotice,
  NoCasesState,
  PendingState,
  RunHistory,
  StreamError,
} from "./ResultsPanelParts";

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
