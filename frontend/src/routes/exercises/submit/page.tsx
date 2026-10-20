import {
  ArrowLeftIcon,
  CalendarClockIcon,
  CircleAlertIcon,
  CodeIcon,
  FileCode2Icon,
  FileUpIcon,
  PanelTopCloseIcon,
  PanelTopOpenIcon,
  SendIcon,
} from "lucide-react";
import {
  lazy,
  Suspense,
  useEffect,
  useMemo,
  useState,
  type ChangeEvent,
  type CSSProperties,
  type SubmitEvent,
} from "react";
import { Link, useParams } from "react-router";

import { ErrorState, LoadingState } from "@/components/app/states";
import { ResultsPanel } from "@/components/submission/ResultsPanel";
import { StatementPanel } from "@/components/submission/StatementPanel";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DockSplitter } from "@/components/ui/dock-splitter";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { useAsync } from "@/hooks/use-async";
import { useCodeEditorPreference } from "@/hooks/use-code-editor-preference";
import { useDockLayout } from "@/hooks/use-dock-layout";
import { useElementSize } from "@/hooks/use-element-size";
import {
  createSubmission,
  getAllowedFileTypes,
  getExercise,
  getExerciseTestCases,
  listAttachedFiles,
  subscribeSubmissionEvents,
  type AllowedFileType,
  type CommitStatus,
  type Exercise,
  type SubmissionArtifactEvent,
  type SubmissionStatusEvent,
} from "@/lib/api";
import { ApiRequestError } from "@/lib/api/client";
import { deadlineInfo } from "@/lib/deadline";
import {
  DOCK_SPLITTER_SIZE,
  dockBounds,
  MIN_EDITOR_HEIGHT,
  MIN_RESULTS_HEIGHT,
  MIN_RIGHT_WIDTH,
  MIN_STATEMENT_WIDTH,
  STATEMENT_RAIL_WIDTH,
} from "@/lib/dock-layout";
import { editorLanguagesFor } from "@/lib/editor/languages";
import { normalizeExtension, validateFileType } from "@/lib/file-types";
import { formatDateTime } from "@/lib/format";
import { isPendingStatus, isTerminalStatus } from "@/lib/submission-status";
import {
  upsertCaseResult,
  viewFromEvent,
  viewFromSnapshot,
  type CaseResultView,
  type CompilationInfo,
  type FinalSummary,
} from "@/lib/submission-view";
import { clamp, cn } from "@/lib/utils";

const INITIAL_STATUS: CommitStatus = "queued";
const FORM_ID = "submission-form";

// CodeMirror and its grammars are large, so they load only when the editor is
// actually shown. The grammar for the selected language is fetched separately.
const CodeEditor = lazy(async () => {
  const { CodeEditor: Editor } = await import("@/components/editor/CodeEditor");
  return { default: Editor };
});

/** Translates a refused submission, keyed on the HTTP status. */
function submitErrorMessage(error: unknown): string {
  if (error instanceof ApiRequestError) {
    switch (error.status) {
      case 400:
        return "Tipo de arquivo não permitido para este exercício.";
      case 403:
        return "Você não está matriculado nesta turma.";
      case 422:
        return "O prazo deste exercício já passou.";
      case 503:
        return "O corretor está indisponível no momento. Tente novamente em alguns instantes.";
    }
  }
  return error instanceof Error
    ? error.message
    : "Erro ao enviar a submissão. Tente novamente.";
}

function commitStorageKey(exerciseId: number): string {
  return `runcodes:submission:${String(exerciseId)}`;
}

function readStoredCommit(exerciseId: number): number | null {
  try {
    const stored = sessionStorage.getItem(commitStorageKey(exerciseId));
    if (!stored) return null;
    const parsed = Number(stored);
    return Number.isInteger(parsed) && parsed > 0 ? parsed : null;
  } catch {
    return null;
  }
}

function storeCommit(exerciseId: number, commitId: number | null): void {
  try {
    if (commitId === null) {
      sessionStorage.removeItem(commitStorageKey(exerciseId));
    } else {
      sessionStorage.setItem(commitStorageKey(exerciseId), String(commitId));
    }
  } catch {
    // Storage unavailable (private mode) — reload will simply not resume.
  }
}

/**
 * Resolves the types an exercise accepts on the server: its own allowed types
 * when configured, otherwise every available platform type.
 */
function resolveAllowedFileTypes(
  exercise: Exercise | null,
  allTypes: AllowedFileType[],
): AllowedFileType[] {
  if (!exercise) return [];

  const own = exercise.allowed_file_types;
  if (own && own.length > 0) {
    const available = own.filter((type) => type.is_available);
    if (available.length > 0) return available;
  }

  const ids = exercise.allowed_file_type_ids;
  if (ids && ids.length > 0) {
    return allTypes.filter(
      (type) => ids.includes(type.id) && type.is_available,
    );
  }

  // An explicitly empty exercise list means "no restriction": offer everything
  // the platform can compile so the editor still has languages to choose from.
  return allTypes.filter((type) => type.is_available);
}

/**
 * Resolves the extensions the exercise accepts for client-side validation.
 * Returns `null` when nothing is restricted (skip validation).
 */
function resolveAllowedExtensions(
  exercise: Exercise | null,
  allTypes: AllowedFileType[],
): string[] | null {
  if (!exercise) return null;

  const own = exercise.allowed_file_types;
  if (own) {
    // An empty list means no restriction; a list whose entries are all
    // unavailable is treated the same way by the backend's validation.
    const available = own.filter((type) => type.is_available);
    if (own.length === 0 || available.length === 0) return null;
    const extensions = [
      ...new Set(available.map((type) => normalizeExtension(type.extension))),
    ].filter((extension) => extension !== "");
    return extensions.length > 0 ? extensions : null;
  }

  const ids = exercise.allowed_file_type_ids;
  const source =
    ids && ids.length > 0
      ? allTypes.filter((type) => ids.includes(type.id))
      : allTypes.filter((type) => type.is_available);
  const extensions = [
    ...new Set(source.map((type) => normalizeExtension(type.extension))),
  ].filter((extension) => extension !== "");
  return extensions.length > 0 ? extensions : null;
}

/** The submission view for one exercise; remounted when the exercise changes. */
function SubmitView({ exerciseId }: { exerciseId: string | undefined }) {
  const numericExerciseId = Number(exerciseId);
  const validId = Number.isInteger(numericExerciseId) && numericExerciseId > 0;

  const exercise = useAsync(
    () =>
      validId
        ? getExercise(numericExerciseId)
        : Promise.reject(new Error("Exercício inválido.")),
    `exercise-${exerciseId ?? ""}`,
    "Exercício não encontrado.",
  );
  const allowedTypes = useAsync(() => getAllowedFileTypes(), "allowed-types");
  const testCases = useAsync(
    () =>
      validId ? getExerciseTestCases(numericExerciseId) : Promise.resolve([]),
    `test-cases-${exerciseId ?? ""}`,
  );
  const attached = useAsync(
    () =>
      validId ? listAttachedFiles(numericExerciseId) : Promise.resolve([]),
    `attached-${exerciseId ?? ""}`,
  );
  const preference = useCodeEditorPreference();
  const [gridRef, gridSize] = useElementSize();
  const [columnRef, columnSize] = useElementSize();
  const dockLayout = useDockLayout();

  const [modeChoice, setModeChoice] = useState<"editor" | "upload" | null>(
    null,
  );
  const [code, setCode] = useState("");
  const [filenameDraft, setFilenameDraft] = useState<string | null>(null);
  const [selectedLanguageId, setSelectedLanguageId] = useState<string | null>(
    null,
  );

  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const [commitId, setCommitId] = useState<number | null>(() =>
    validId ? readStoredCommit(numericExerciseId) : null,
  );
  const [status, setStatus] = useState<CommitStatus>(INITIAL_STATUS);
  const [history, setHistory] = useState<SubmissionStatusEvent[]>([]);
  const [compilation, setCompilation] = useState<CompilationInfo | null>(null);
  const [results, setResults] = useState<CaseResultView[]>([]);
  const [artifacts, setArtifacts] = useState<SubmissionArtifactEvent[]>([]);
  const [final, setFinal] = useState<FinalSummary | null>(null);
  const [streamError, setStreamError] = useState<string | null>(null);

  const allowedFileTypes = useMemo(
    () => resolveAllowedFileTypes(exercise.data, allowedTypes.data ?? []),
    [exercise.data, allowedTypes.data],
  );
  const allowedExtensions = useMemo(
    () => resolveAllowedExtensions(exercise.data, allowedTypes.data ?? []),
    [exercise.data, allowedTypes.data],
  );
  const languages = useMemo(
    () => editorLanguagesFor(allowedFileTypes),
    [allowedFileTypes],
  );
  const selectedLanguage = useMemo(
    () =>
      languages.find((item) => item.id === selectedLanguageId) ?? languages[0],
    [languages, selectedLanguageId],
  );
  const filename = filenameDraft ?? selectedLanguage?.defaultFilename ?? "";

  // An explicit choice only lives for this visit; otherwise the stored
  // preference decides the default input mode.
  const effectiveMode =
    languages.length === 0
      ? "upload"
      : (modeChoice ?? (preference.enabled ? "editor" : "upload"));

  useEffect(() => {
    if (commitId === null) return;

    const unsubscribe = subscribeSubmissionEvents(commitId, {
      onSnapshot: (event) => {
        // A frame only arrives on a live connection, so any earlier stream
        // warning is stale.
        setStreamError(null);
        setStatus(event.commit.status);
        setHistory([]);
        setCompilation({
          compiled: event.commit.compiled,
          message: event.commit.compilation_message,
          error: event.commit.compilation_error,
        });
        setResults(event.results.map(viewFromSnapshot));
        if (isTerminalStatus(event.commit.status)) {
          setFinal({
            status: event.commit.status,
            numCorrectCases: event.commit.num_correct_cases,
            score: event.commit.score,
            compilationMessage: event.commit.compilation_message,
            compilationError: event.commit.compilation_error,
          });
        } else {
          setFinal(null);
        }
      },
      onStatus: (event) => {
        setStreamError(null);
        setStatus(event.status);
        setHistory((previous) => [...previous, event]);
      },
      onCompilation: (event) => {
        setStreamError(null);
        setCompilation({
          compiled: event.compiled,
          message: event.message,
          error: event.error,
        });
      },
      onCaseResult: (event) => {
        setStreamError(null);
        setResults((previous) =>
          upsertCaseResult(previous, viewFromEvent(event)),
        );
      },
      onArtifact: (event) => {
        setStreamError(null);
        setArtifacts((previous) => [...previous, event]);
      },
      onFinished: (event) => {
        setStreamError(null);
        setStatus(event.status);
        setFinal({
          status: event.status,
          numCorrectCases: event.num_correct_cases,
          score: event.score,
          compilationMessage: event.compilation_message,
          compilationError: event.compilation_error,
        });
        setCompilation({
          compiled: event.compilation_error === "",
          message: event.compilation_message,
          error: event.compilation_error,
        });
      },
      onError: (event) => {
        setStreamError(event.message);
      },
      onConnectionError: () => {
        setStreamError(
          "Não foi possível obter os resultados em tempo real. Recarregue a página para tentar novamente.",
        );
      },
    });

    return unsubscribe;
  }, [commitId]);

  function resetRunState() {
    setStatus(INITIAL_STATUS);
    setHistory([]);
    setCompilation(null);
    setResults([]);
    setArtifacts([]);
    setFinal(null);
    setStreamError(null);
  }

  function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.item(0) ?? null;
    setSelectedFile(file);
    setFileError(file ? validateFileType(file, allowedExtensions) : null);
  }

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!exercise.data) return;

    let file: File;
    if (effectiveMode === "editor") {
      if (!selectedLanguage || code.trim() === "") return;
      file = new File(
        [code],
        filename.trim() || selectedLanguage.defaultFilename,
        { type: "text/plain" },
      );
    } else {
      if (!selectedFile) return;
      file = selectedFile;
    }

    const validation = validateFileType(file, allowedExtensions);
    if (validation) {
      setFileError(validation);
      return;
    }

    setFileError(null);
    setSubmitting(true);
    setSubmitError(null);
    try {
      const queued = await createSubmission(exercise.data.id, file);
      storeCommit(exercise.data.id, queued.commit_id);
      resetRunState();
      setCommitId(queued.commit_id);
    } catch (error) {
      setSubmitError(submitErrorMessage(error));
    } finally {
      setSubmitting(false);
    }
  }

  if (exercise.loading) return <LoadingState />;
  if (!exercise.data) {
    return (
      <ErrorState
        description={
          exercise.error ?? "Não foi possível carregar este exercício."
        }
      />
    );
  }

  const current = exercise.data;
  const info = deadlineInfo(current.deadline);
  const materials = attached.data ?? [];
  const editorAvailable = languages.length > 0;
  const pending =
    commitId !== null && final === null && isPendingStatus(status);
  const canSubmit =
    !submitting &&
    !pending &&
    (effectiveMode === "editor"
      ? editorAvailable && code.trim() !== ""
      : selectedFile !== null);

  const statementCollapsed = dockLayout.layout.collapsed.statement;
  const editorCollapsed = dockLayout.layout.collapsed.editor;
  const resultsCollapsed = dockLayout.layout.collapsed.results;

  const horizontalBounds = dockBounds(
    gridSize.width - DOCK_SPLITTER_SIZE,
    MIN_STATEMENT_WIDTH,
    MIN_RIGHT_WIDTH,
  );
  const leftRatio = clamp(
    dockLayout.layout.horizontal,
    horizontalBounds.min,
    horizontalBounds.max,
  );
  const verticalBounds = dockBounds(
    columnSize.height - DOCK_SPLITTER_SIZE,
    MIN_EDITOR_HEIGHT,
    MIN_RESULTS_HEIGHT,
  );
  const editorRatio = clamp(
    dockLayout.layout.vertical,
    verticalBounds.min,
    verticalBounds.max,
  );

  const dockColumns = statementCollapsed
    ? `${String(STATEMENT_RAIL_WIDTH)}px minmax(0, 1fr)`
    : `${leftRatio.toFixed(4)}fr ${String(DOCK_SPLITTER_SIZE)}px ${(1 - leftRatio).toFixed(4)}fr`;
  const showHorizontalSplitter = !statementCollapsed;
  const showVerticalSplitter = !editorCollapsed && !resultsCollapsed;
  const editorGrow = editorCollapsed ? 0 : resultsCollapsed ? 1 : editorRatio;
  const resultsGrow = resultsCollapsed
    ? 0
    : editorCollapsed
      ? 1
      : 1 - editorRatio;

  const submitAction = (
    <Button
      type="submit"
      form={FORM_ID}
      size="sm"
      disabled={!canSubmit}
      aria-busy={submitting}
      title="Enviar a solução para correção"
    >
      {submitting ? <Spinner className="size-4" /> : <SendIcon />}
      {submitting ? "Enviando…" : "Enviar solução"}
    </Button>
  );

  return (
    <div className="flex flex-col gap-3 xl:h-full">
      <header className="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-2">
        <Link
          to="/exercises"
          className="text-muted-foreground hover:text-foreground inline-flex shrink-0 items-center gap-1 text-xs font-medium"
        >
          <ArrowLeftIcon className="size-3.5" aria-hidden />
          Exercícios
        </Link>
        <span className="bg-border hidden h-4 w-px sm:block" aria-hidden />
        <h1 className="min-w-0 text-lg font-semibold tracking-tight text-balance">
          {current.title}
        </h1>
        <Badge
          variant={
            info.expired
              ? "destructive"
              : info.tone === "warning"
                ? "warning"
                : "outline"
          }
        >
          {info.label}
        </Badge>
        <span className="text-muted-foreground order-last flex w-full items-center gap-1.5 text-xs sm:order-0 sm:ml-auto sm:w-auto">
          <CalendarClockIcon className="size-3.5 shrink-0" aria-hidden />
          Prazo: {formatDateTime(current.deadline)}
        </span>
      </header>

      <div
        ref={gridRef}
        style={{ "--dock-columns": dockColumns } as CSSProperties}
        className={cn(
          "grid min-h-0 grid-cols-1 gap-3 xl:flex-1 xl:grid-cols-(--dock-columns)",
          statementCollapsed ? "xl:gap-3" : "xl:gap-0",
        )}
      >
        <StatementPanel
          exerciseId={current.id}
          description={current.description}
          materials={materials}
          materialsLoading={attached.loading}
          materialsError={attached.error}
          collapsed={statementCollapsed}
          onToggleCollapsed={dockLayout.toggleStatement}
        />

        {showHorizontalSplitter ? (
          <DockSplitter
            orientation="vertical"
            value={leftRatio}
            min={horizontalBounds.min}
            max={horizontalBounds.max}
            onChange={dockLayout.setHorizontal}
            onReset={dockLayout.resetHorizontal}
            label="Redimensionar enunciado e editor"
            className="hidden xl:flex"
          />
        ) : null}

        <div
          ref={columnRef}
          style={
            {
              "--editor-grow": String(editorGrow),
              "--results-grow": String(resultsGrow),
            } as CSSProperties
          }
          className={cn(
            "flex min-h-0 min-w-0 flex-col gap-3",
            showVerticalSplitter ? "xl:gap-0" : "xl:gap-3",
          )}
        >
          <section
            className={cn(
              "bg-card flex min-h-0 min-w-0 flex-col overflow-hidden rounded-xl border",
              editorCollapsed
                ? "xl:basis-auto xl:grow-0 xl:shrink-0"
                : "xl:basis-0 xl:grow-(--editor-grow)",
            )}
          >
            <form
              id={FORM_ID}
              className="flex min-h-0 flex-col xl:flex-1"
              onSubmit={(event) => void handleSubmit(event)}
            >
              <div className="flex min-h-11 shrink-0 flex-wrap items-center gap-2 border-b px-2 py-1.5">
                {editorAvailable ? (
                  <div className="bg-muted/50 flex shrink-0 items-center rounded-md border p-0.5">
                    <Button
                      type="button"
                      size="xs"
                      variant={
                        effectiveMode === "editor" ? "secondary" : "ghost"
                      }
                      aria-pressed={effectiveMode === "editor"}
                      onClick={() => {
                        setModeChoice("editor");
                        setFileError(null);
                      }}
                    >
                      <CodeIcon />
                      Editor
                    </Button>
                    <Button
                      type="button"
                      size="xs"
                      variant={
                        effectiveMode === "upload" ? "secondary" : "ghost"
                      }
                      aria-pressed={effectiveMode === "upload"}
                      onClick={() => {
                        setModeChoice("upload");
                        setFileError(null);
                      }}
                    >
                      <FileUpIcon />
                      Arquivo
                    </Button>
                  </div>
                ) : (
                  <p className="text-muted-foreground shrink-0 px-1 text-xs font-medium">
                    Arquivo da solução
                  </p>
                )}

                {effectiveMode === "editor" && selectedLanguage ? (
                  <>
                    <Select
                      aria-label="Linguagem"
                      value={selectedLanguage.id}
                      onChange={(event) => {
                        setSelectedLanguageId(event.target.value);
                        setFilenameDraft(null);
                      }}
                      className="border-border/70 h-7 w-auto min-w-28 cursor-pointer rounded-md bg-transparent px-2 py-0 text-xs shadow-none"
                    >
                      {languages.map((language) => (
                        <option key={language.id} value={language.id}>
                          {language.label}
                        </option>
                      ))}
                    </Select>
                    <div className="flex items-center gap-1.5">
                      <FileCode2Icon
                        className="text-muted-foreground size-3.5 shrink-0"
                        aria-hidden
                      />
                      <Input
                        aria-label="Nome do arquivo"
                        value={filename}
                        onChange={(event) => {
                          setFilenameDraft(event.target.value);
                        }}
                        className="border-border/70 h-7 w-36 rounded-md bg-transparent px-2 py-0 font-mono text-xs shadow-none"
                        spellCheck={false}
                      />
                    </div>
                  </>
                ) : null}

                <div className="ml-auto flex min-w-0 items-center gap-2">
                  {effectiveMode === "upload" && selectedFile ? (
                    <span className="text-muted-foreground min-w-0 truncate text-[11px]">
                      {selectedFile.name}
                    </span>
                  ) : null}
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    className="hidden xl:inline-flex"
                    aria-label={
                      editorCollapsed ? "Expandir editor" : "Recolher editor"
                    }
                    title={
                      editorCollapsed ? "Expandir editor" : "Recolher editor"
                    }
                    onClick={dockLayout.toggleEditor}
                  >
                    {editorCollapsed ? (
                      <PanelTopOpenIcon className="size-4.5" />
                    ) : (
                      <PanelTopCloseIcon className="size-4.5" />
                    )}
                  </Button>
                </div>
              </div>

              {fileError ? (
                <div
                  aria-live="polite"
                  className={cn(
                    "border-destructive/30 bg-destructive/10 border-b px-3 py-1.5",
                    editorCollapsed && "xl:hidden",
                  )}
                >
                  <p className="text-destructive flex items-start gap-1.5 text-xs">
                    <CircleAlertIcon
                      className="mt-px size-3.5 shrink-0"
                      aria-hidden
                    />
                    {fileError}
                  </p>
                </div>
              ) : null}

              {effectiveMode === "editor" && selectedLanguage ? (
                <Suspense
                  fallback={
                    <div
                      aria-hidden
                      className={cn(
                        "bg-muted/30 h-80 w-full xl:h-auto xl:min-h-0 xl:flex-1",
                        editorCollapsed && "xl:hidden",
                      )}
                    />
                  }
                >
                  <CodeEditor
                    value={code}
                    onChange={setCode}
                    language={selectedLanguage}
                    className={cn(
                      "h-80 w-full rounded-none border-0 xl:h-auto xl:min-h-0 xl:flex-1",
                      editorCollapsed && "xl:hidden",
                    )}
                  />
                </Suspense>
              ) : (
                <div
                  className={cn(
                    "flex min-h-64 flex-1 flex-col items-center justify-center gap-3 px-6 py-8 text-center xl:min-h-0",
                    editorCollapsed && "xl:hidden",
                  )}
                >
                  <div className="bg-muted text-muted-foreground flex size-10 items-center justify-center rounded-full">
                    <FileUpIcon className="size-5" aria-hidden />
                  </div>
                  <div className="space-y-1">
                    <p className="text-sm font-medium">
                      Envie o arquivo da sua solução
                    </p>
                    <p className="text-muted-foreground text-xs">
                      {allowedExtensions
                        ? `Tipos aceitos: ${allowedExtensions.join(", ")}`
                        : "Nenhuma restrição de tipo configurada neste exercício."}
                    </p>
                  </div>
                  <Input
                    id="submission-file"
                    type="file"
                    accept={allowedExtensions?.join(",")}
                    onChange={handleFileChange}
                    aria-label="Arquivo da solução"
                    className="bg-background h-auto max-w-sm cursor-pointer py-1.5 text-xs file:mr-3 file:text-xs"
                  />
                </div>
              )}
            </form>
          </section>

          {showVerticalSplitter ? (
            <DockSplitter
              orientation="horizontal"
              value={editorRatio}
              min={verticalBounds.min}
              max={verticalBounds.max}
              onChange={dockLayout.setVertical}
              onReset={dockLayout.resetVertical}
              label="Redimensionar editor e resultados"
              className="hidden xl:flex"
            />
          ) : null}

          <ResultsPanel
            commitId={commitId}
            status={status}
            history={history}
            compilation={compilation}
            results={results}
            final={final}
            streamError={streamError}
            artifacts={artifacts}
            testCases={testCases.data ?? []}
            testCasesLoading={testCases.loading}
            testCasesError={testCases.error}
            actions={submitAction}
            actionError={submitError}
            collapsed={resultsCollapsed}
            onToggleCollapsed={dockLayout.toggleResults}
            className={cn(
              "xl:mt-auto",
              resultsCollapsed
                ? "xl:basis-auto xl:grow-0 xl:shrink-0"
                : "xl:basis-0 xl:grow-(--results-grow)",
            )}
          />
        </div>
      </div>
    </div>
  );
}

/** Student page to submit a solution and watch the live judging results. */
export function SubmitPage() {
  const { exerciseId } = useParams();
  // Keyed by exercise so navigating between exercises resets the run state and
  // re-reads the submission stored for the new exercise.
  return <SubmitView key={exerciseId} exerciseId={exerciseId} />;
}
