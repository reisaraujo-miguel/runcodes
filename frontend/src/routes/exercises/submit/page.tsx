import { FileUpIcon, PaperclipIcon } from "lucide-react";
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
import {
  LiveResults,
  type CaseVisibility,
} from "@/components/submission/LiveResults";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { useAsync } from "@/hooks/use-async";
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
import { formatDateTime } from "@/lib/format";
import { isTerminalStatus } from "@/lib/submission-status";
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

function normalizeExtension(extension: string): string {
  const trimmed = extension.trim().toLowerCase();
  if (!trimmed) return "";
  return trimmed.startsWith(".") ? trimmed : `.${trimmed}`;
}

function extensionOf(filename: string): string {
  const dot = filename.lastIndexOf(".");
  return dot === -1 ? "" : filename.slice(dot);
}

/**
 * Resolves the extensions the exercise accepts: the exercise's own allowed
 * types when the endpoint embeds them, otherwise the platform's available
 * types. Returns `null` when nothing is known (skip client-side validation).
 */
function resolveAllowedExtensions(
  exercise: Exercise | null,
  allTypes: AllowedFileType[],
): string[] | null {
  if (!exercise) return null;

  const own = exercise.allowed_file_types;
  if (own) {
    if (own.length === 0) return null;
    const extensions = [
      ...new Set(own.map((type) => normalizeExtension(type.extension))),
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

function validateFile(file: File, allowed: string[] | null): string | null {
  if (!allowed) return null;
  const extension = normalizeExtension(extensionOf(file.name));
  if (extension !== "" && allowed.includes(extension)) return null;
  return `Extensão não permitida. Tipos aceitos: ${allowed.join(", ")}.`;
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

  const allowedExtensions = useMemo(
    () => resolveAllowedExtensions(exercise.data, allowedTypes.data ?? []),
    [exercise.data, allowedTypes.data],
  );

  // Honor each test case's visibility flags in the student's result view.
  const visibility = useMemo(() => {
    const map = new Map<number, CaseVisibility>();
    for (const testCase of testCases.data ?? []) {
      map.set(testCase.id, { showUserOutput: testCase.show_user_output });
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
    setFileError(file ? validateFile(file, allowedExtensions) : null);
  }

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!exercise.data || !selectedFile) return;

    const validation = validateFile(selectedFile, allowedExtensions);
    if (validation) {
      setFileError(validation);
      return;
    }

    setSubmitting(true);
    setSubmitError(null);
    try {
      const queued = await createSubmission(exercise.data.id, selectedFile);
      storeCommit(exercise.data.id, queued.commit_id);
      resetRunState();
      setCommitId(queued.commit_id);
    } catch (error) {
      setSubmitError(submitErrorMessage(error));
    } finally {
      setSubmitting(false);
    }
  }

  function handleNewSubmission() {
    if (exercise.data) storeCommit(exercise.data.id, null);
    resetRunState();
    setCommitId(null);
    setSelectedFile(null);
    setFileError(null);
    setSubmitError(null);
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

  return (
    <div className="space-y-6">
      <PageHeader
        title={current.title}
        description={current.description || undefined}
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

      <div className="grid gap-6 lg:grid-cols-3">
        <div className="space-y-4 lg:col-span-2">
          {commitId === null ? (
            <SectionCard
              title="Enviar solução"
              description="Selecione o arquivo com o código da sua solução e envie para correção."
            >
              <form
                className="space-y-4"
                onSubmit={(event) => void handleSubmit(event)}
              >
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
                  {fileError ? (
                    <p className="text-destructive text-sm">{fileError}</p>
                  ) : null}
                </div>

                {submitError ? (
                  <Alert variant="destructive">
                    <AlertDescription>{submitError}</AlertDescription>
                  </Alert>
                ) : null}

                <Button type="submit" disabled={!selectedFile || submitting}>
                  {submitting ? <Spinner className="size-4" /> : <FileUpIcon />}
                  {submitting ? "Enviando…" : "Enviar para correção"}
                </Button>
              </form>
            </SectionCard>
          ) : (
            <>
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
              {final ? (
                <Button variant="outline" onClick={handleNewSubmission}>
                  Enviar outra solução
                </Button>
              ) : null}
            </>
          )}
        </div>

        <aside className="space-y-6">
          <SectionCard title="Informações">
            <dl className="space-y-3 text-sm">
              <div>
                <dt className="text-muted-foreground">Abertura</dt>
                <dd>{formatDateTime(current.open_date)}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Prazo</dt>
                <dd>{formatDateTime(current.deadline)}</dd>
              </div>
            </dl>
          </SectionCard>

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
                    <span className="truncate font-mono">{file.filename}</span>
                  </li>
                ))}
              </ul>
            )}
          </SectionCard>
        </aside>
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
