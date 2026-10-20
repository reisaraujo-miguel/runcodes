import {
  CheckCircle2Icon,
  CodeIcon,
  DownloadIcon,
  FileUpIcon,
  Loader2Icon,
  PaperclipIcon,
} from "lucide-react";
import {
  useEffect,
  useMemo,
  useState,
  type ChangeEvent,
  type SubmitEvent,
} from "react";
import { Link, useParams } from "react-router";

import { PageHeader } from "@/components/app/PageHeader";
import { SectionCard } from "@/components/app/SectionCard";
import { ErrorState, LoadingState } from "@/components/app/states";
import { StatusBadge } from "@/components/app/StatusBadge";
import { CodeEditor } from "@/components/editor/CodeEditor";
import {
  LiveResults,
  type CaseVisibility,
} from "@/components/submission/LiveResults";
import { Markdown } from "@/components/submission/Markdown";
import { TestCasesPreview } from "@/components/submission/TestCasesPreview";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { useAsync } from "@/hooks/use-async";
import { useCodeEditorPreference } from "@/hooks/use-code-editor-preference";
import {
  attachedFileDownloadUrl,
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
import { editorLanguagesFor } from "@/lib/editor/languages";
import {
  extensionOf,
  normalizeExtension,
  validateFileType,
} from "@/lib/file-types";
import { formatDateTime } from "@/lib/format";
import {
  commitStatus,
  finishedStatus,
  isPendingStatus,
  isTerminalStatus,
} from "@/lib/submission-status";
import {
  upsertCaseResult,
  viewFromEvent,
  viewFromSnapshot,
  type CaseResultView,
  type CompilationInfo,
  type FinalSummary,
} from "@/lib/submission-view";

const INITIAL_STATUS: CommitStatus = "queued";

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

function isPdf(filename: string): boolean {
  return extensionOf(filename) === ".pdf";
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

  // Honor each test case's visibility flags in the student's result view.
  const visibility = useMemo(() => {
    const map = new Map<number, CaseVisibility>();
    for (const testCase of testCases.data ?? []) {
      map.set(testCase.id, {
        showUserOutput: testCase.show_user_output,
        showInput: testCase.show_input,
        showExpectedOutput: testCase.show_expected_output,
        input: testCase.input,
        inputType: testCase.input_type,
        expectedOutput: testCase.expected_output,
        expectedOutputType: testCase.expected_output_type,
      });
    }
    return map;
  }, [testCases.data]);

  useEffect(() => {
    if (commitId === null) return;

    const unsubscribe = subscribeSubmissionEvents(commitId, {
      onSnapshot: (event) => {
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
        setStatus(event.status);
        setHistory((previous) => [...previous, event]);
      },
      onCompilation: (event) => {
        setCompilation({
          compiled: event.compiled,
          message: event.message,
          error: event.error,
        });
      },
      onCaseResult: (event) => {
        setResults((previous) =>
          upsertCaseResult(previous, viewFromEvent(event)),
        );
      },
      onArtifact: (event) => {
        setArtifacts((previous) => [...previous, event]);
      },
      onFinished: (event) => {
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
  const pdfMaterials = materials.filter((file) => isPdf(file.filename));
  const primaryPdf = pdfMaterials[0];
  const editorAvailable = languages.length > 0;

  const pending =
    commitId !== null && final === null && isPendingStatus(status);
  const overall = final ? finishedStatus(final.status) : commitStatus(status);
  const canSubmit =
    !submitting &&
    !pending &&
    (effectiveMode === "editor"
      ? editorAvailable && code.trim() !== ""
      : selectedFile !== null);

  return (
    <div className="space-y-6">
      <PageHeader
        title={current.title}
        breadcrumb={
          <Link
            to="/exercises"
            className="text-muted-foreground text-sm hover:underline"
          >
            ← Exercícios
          </Link>
        }
        actions={
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
        }
      />

      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)]">
        <div className="space-y-4 lg:sticky lg:top-24 lg:max-h-[calc(100svh-7rem)] lg:overflow-y-auto lg:pr-1">
          <SectionCard
            title="Enunciado"
            description={`Prazo: ${formatDateTime(current.deadline)}`}
          >
            {current.description.trim() ? (
              <Markdown>{current.description}</Markdown>
            ) : (
              <p className="text-muted-foreground text-sm">
                Este exercício não possui descrição em texto.
              </p>
            )}
          </SectionCard>

          {primaryPdf ? (
            <SectionCard
              title="Enunciado em PDF"
              description={primaryPdf.filename}
              action={
                <Button
                  size="sm"
                  variant="outline"
                  render={
                    <a
                      href={attachedFileDownloadUrl(current.id, primaryPdf.id)}
                      target="_blank"
                      rel="noopener noreferrer"
                    />
                  }
                >
                  <DownloadIcon />
                  Abrir
                </Button>
              }
            >
              <iframe
                title={`Enunciado em PDF: ${primaryPdf.filename}`}
                src={attachedFileDownloadUrl(current.id, primaryPdf.id)}
                className="border-border h-[65vh] w-full rounded-lg border"
              />
            </SectionCard>
          ) : null}

          <SectionCard
            title="Materiais de apoio"
            description="Arquivos anexados pelo professor."
          >
            {attached.loading ? (
              <LoadingState className="py-6" />
            ) : materials.length === 0 ? (
              <p className="text-muted-foreground text-sm">
                Nenhum material anexado a este exercício.
              </p>
            ) : (
              <ul className="space-y-2">
                {materials.map((file) => (
                  <li key={file.id} className="flex items-center gap-2 text-sm">
                    <PaperclipIcon
                      className="text-muted-foreground size-4 shrink-0"
                      aria-hidden
                    />
                    <a
                      href={attachedFileDownloadUrl(current.id, file.id)}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-primary min-w-0 truncate font-mono hover:underline"
                    >
                      {file.filename}
                    </a>
                  </li>
                ))}
              </ul>
            )}
          </SectionCard>
        </div>

        <div className="space-y-4">
          <SectionCard
            title="Sua solução"
            description="Escreva o código no editor ou envie um arquivo pronto."
            action={
              editorAvailable && preference.enabled ? (
                <div className="flex rounded-lg border p-0.5">
                  <Button
                    size="xs"
                    variant={effectiveMode === "editor" ? "secondary" : "ghost"}
                    onClick={() => {
                      setModeChoice("editor");
                    }}
                  >
                    <CodeIcon />
                    Editor
                  </Button>
                  <Button
                    size="xs"
                    variant={effectiveMode === "upload" ? "secondary" : "ghost"}
                    onClick={() => {
                      setModeChoice("upload");
                    }}
                  >
                    <FileUpIcon />
                    Arquivo
                  </Button>
                </div>
              ) : undefined
            }
          >
            <form
              className="space-y-4"
              onSubmit={(event) => void handleSubmit(event)}
            >
              {effectiveMode === "editor" && selectedLanguage ? (
                <>
                  <div className="flex flex-wrap items-end gap-3">
                    <div className="min-w-44 flex-1 space-y-2">
                      <Label htmlFor="editor-language">Linguagem</Label>
                      <Select
                        id="editor-language"
                        value={selectedLanguage.id}
                        onChange={(event) => {
                          setSelectedLanguageId(event.target.value);
                          setFilenameDraft(null);
                        }}
                      >
                        {languages.map((language) => (
                          <option key={language.id} value={language.id}>
                            {language.label}
                          </option>
                        ))}
                      </Select>
                    </div>
                    <div className="min-w-48 flex-1 space-y-2">
                      <Label htmlFor="editor-filename">Nome do arquivo</Label>
                      <Input
                        id="editor-filename"
                        value={filename}
                        onChange={(event) => {
                          setFilenameDraft(event.target.value);
                        }}
                        className="font-mono"
                        spellCheck={false}
                      />
                    </div>
                  </div>

                  <CodeEditor
                    value={code}
                    onChange={setCode}
                    language={selectedLanguage}
                    className="h-[26rem]"
                  />
                </>
              ) : effectiveMode === "editor" ? (
                <Alert>
                  <AlertDescription>
                    Este exercício não possui linguagens de programação
                    configuradas para o editor. Envie um arquivo.
                  </AlertDescription>
                </Alert>
              ) : (
                <div className="space-y-2">
                  <Label htmlFor="submission-file">Arquivo da solução</Label>
                  <Input
                    id="submission-file"
                    type="file"
                    accept={allowedExtensions?.join(",")}
                    onChange={handleFileChange}
                    className="h-auto py-2"
                  />
                  {allowedExtensions ? (
                    <p className="text-muted-foreground text-xs">
                      Tipos aceitos: {allowedExtensions.join(", ")}
                    </p>
                  ) : (
                    <p className="text-muted-foreground text-xs">
                      Nenhuma restrição de tipo configurada neste exercício.
                    </p>
                  )}
                </div>
              )}

              {fileError ? (
                <p className="text-destructive text-sm">{fileError}</p>
              ) : null}

              {submitError ? (
                <Alert variant="destructive">
                  <AlertDescription>{submitError}</AlertDescription>
                </Alert>
              ) : null}

              {preference.error ? (
                <Alert variant="destructive">
                  <AlertDescription>{preference.error}</AlertDescription>
                </Alert>
              ) : null}

              <div className="flex flex-wrap items-center justify-between gap-3">
                <Button type="submit" disabled={!canSubmit}>
                  {submitting ? <Spinner className="size-4" /> : <FileUpIcon />}
                  {submitting
                    ? "Enviando…"
                    : pending
                      ? "Correção em andamento"
                      : "Enviar para correção"}
                </Button>

                {editorAvailable ? (
                  <label className="text-muted-foreground flex cursor-pointer items-center gap-2 text-xs">
                    <Checkbox
                      checked={preference.enabled}
                      disabled={preference.saving || preference.loading}
                      onCheckedChange={(checked) => {
                        setModeChoice(null);
                        void preference.setEnabled(checked);
                      }}
                    />
                    Usar editor de código
                    {preference.saving ? (
                      <Loader2Icon
                        className="size-3 animate-spin"
                        aria-label="Salvando preferência"
                      />
                    ) : null}
                  </label>
                ) : null}
              </div>
            </form>
          </SectionCard>

          {commitId !== null ? (
            <>
              <div
                role="status"
                aria-live="polite"
                className="bg-card flex flex-wrap items-center justify-between gap-3 rounded-lg border px-3 py-2"
              >
                <div className="flex items-center gap-2">
                  {pending ? (
                    <Loader2Icon
                      className="text-muted-foreground size-4 animate-spin"
                      aria-hidden
                    />
                  ) : (
                    <CheckCircle2Icon
                      className="text-success size-4"
                      aria-hidden
                    />
                  )}
                  <span className="text-sm font-medium">
                    Submissão #{String(commitId)}
                  </span>
                  <StatusBadge value={overall} />
                </div>
                {final ? (
                  <span className="text-muted-foreground text-sm">
                    Nota{" "}
                    <span className="text-foreground font-medium tabular-nums">
                      {final.score.toFixed(2)}
                    </span>{" "}
                    · {String(final.numCorrectCases)} caso(s) correto(s)
                  </span>
                ) : null}
              </div>

              <LiveResults
                status={status}
                history={history}
                compilation={compilation}
                results={results}
                final={final}
                streamError={streamError}
                artifacts={artifacts}
                visibility={visibility}
              />

              {results.length === 0 ? (
                <TestCasesPreview
                  testCases={testCases.data ?? []}
                  loading={testCases.loading}
                />
              ) : null}
            </>
          ) : (
            <TestCasesPreview
              testCases={testCases.data ?? []}
              loading={testCases.loading}
            />
          )}
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
