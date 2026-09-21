import { FileUp } from "lucide-react";
import {
  useEffect,
  useMemo,
  useState,
  type ChangeEvent,
  type SubmitEvent,
} from "react";
import { useParams } from "react-router";

import { Footer } from "@/components/Footer";
import { Navbar } from "@/components/Navbar";
import { LiveResults } from "@/components/submission/LiveResults";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  createSubmission,
  getAllowedFileTypes,
  getExercise,
  subscribeSubmissionEvents,
  type AllowedFileType,
  type CommitStatus,
  type Exercise,
  type FinishedStatus,
  type SubmissionStatusEvent,
} from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { isPendingStatus } from "@/lib/submission-status";
import {
  upsertCaseResult,
  viewFromEvent,
  viewFromSnapshot,
  type CaseResultView,
  type CompilationInfo,
  type FinalSummary,
} from "@/lib/submission-view";

const INITIAL_STATUS: CommitStatus = "queued";

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
    // An empty array means the exercise configured no restriction; the
    // backend accepts any file in that case, so skip client-side validation.
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

function isTerminalStatus(status: CommitStatus): status is FinishedStatus {
  return !isPendingStatus(status);
}

/** Student page to submit a solution and watch the live judging results. */
export function SubmitPage() {
  const { exerciseId } = useParams();
  const numericExerciseId = Number(exerciseId);

  const [exercise, setExercise] = useState<Exercise | null>(null);
  const [allowedTypes, setAllowedTypes] = useState<AllowedFileType[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const [commitId, setCommitId] = useState<number | null>(null);
  const [status, setStatus] = useState<CommitStatus>(INITIAL_STATUS);
  const [history, setHistory] = useState<SubmissionStatusEvent[]>([]);
  const [compilation, setCompilation] = useState<CompilationInfo | null>(null);
  const [results, setResults] = useState<CaseResultView[]>([]);
  const [final, setFinal] = useState<FinalSummary | null>(null);
  const [streamError, setStreamError] = useState<string | null>(null);

  const allowedExtensions = useMemo(
    () => resolveAllowedExtensions(exercise, allowedTypes),
    [exercise, allowedTypes],
  );

  useEffect(() => {
    let cancelled = false;

    async function loadExercise() {
      if (!Number.isInteger(numericExerciseId) || numericExerciseId <= 0) {
        setLoadError("Exercício inválido");
        setLoading(false);
        return;
      }

      try {
        const data = await getExercise(numericExerciseId);
        if (cancelled) return;
        setExercise(data);
        setCommitId(readStoredCommit(numericExerciseId));
      } catch {
        if (!cancelled) setLoadError("Exercício não encontrado");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    void loadExercise();

    return () => {
      cancelled = true;
    };
  }, [numericExerciseId]);

  useEffect(() => {
    let cancelled = false;
    void getAllowedFileTypes()
      .then((types) => {
        if (!cancelled) setAllowedTypes(types);
      })
      .catch(() => {
        // Validation against allowed types is best-effort.
      });
    return () => {
      cancelled = true;
    };
  }, []);

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
    setFinal(null);
    setStreamError(null);
  }

  function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.item(0) ?? null;
    setSelectedFile(file);
    setFileError(file ? validateFile(file, allowedExtensions) : null);
  }

  function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!exercise || !selectedFile) return;

    const validation = validateFile(selectedFile, allowedExtensions);
    if (validation) {
      setFileError(validation);
      return;
    }

    setSubmitting(true);
    setSubmitError(null);

    void (async () => {
      try {
        const queued = await createSubmission(exercise.id, selectedFile);
        storeCommit(exercise.id, queued.commit_id);
        resetRunState();
        setCommitId(queued.commit_id);
      } catch (error) {
        setSubmitError(
          error instanceof Error
            ? error.message
            : "Erro ao enviar a submissão. Tente novamente.",
        );
      } finally {
        setSubmitting(false);
      }
    })();
  }

  function handleNewSubmission() {
    if (exercise) storeCommit(exercise.id, null);
    resetRunState();
    setCommitId(null);
    setSelectedFile(null);
    setFileError(null);
    setSubmitError(null);
  }

  if (loading) {
    return (
      <div>
        <Navbar />
        <main className="flex justify-center p-12">
          <div
            aria-label="Carregando"
            className="h-6 w-6 animate-spin rounded-full border-2 border-muted-foreground/30 border-t-muted-foreground"
            role="status"
          />
        </main>
        <Footer />
      </div>
    );
  }

  if (!exercise) {
    return (
      <div>
        <Navbar />
        <main className="mx-auto max-w-3xl p-6">
          <Card>
            <CardHeader>
              <CardTitle>Exercício não encontrado</CardTitle>
              <CardDescription>
                {loadError ?? "Não foi possível carregar este exercício."}
              </CardDescription>
            </CardHeader>
          </Card>
        </main>
        <Footer />
      </div>
    );
  }

  return (
    <div>
      <Navbar />
      <main className="mx-auto max-w-3xl space-y-4 p-6">
        <Card>
          <CardHeader>
            <CardTitle className="text-2xl">{exercise.title}</CardTitle>
            {exercise.description && (
              <CardDescription>{exercise.description}</CardDescription>
            )}
          </CardHeader>
          <CardContent className="flex flex-wrap gap-x-8 gap-y-2 text-sm">
            <div>
              <p className="text-muted-foreground">Abertura</p>
              <p>{formatDateTime(exercise.open_date)}</p>
            </div>
            <div>
              <p className="text-muted-foreground">Prazo</p>
              <p>{formatDateTime(exercise.deadline)}</p>
            </div>
          </CardContent>
        </Card>

        {commitId === null ? (
          <Card>
            <CardHeader>
              <CardTitle>Enviar solução</CardTitle>
              <CardDescription>
                Selecione o arquivo com o código da sua solução e envie para
                correção.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form className="space-y-4" onSubmit={handleSubmit}>
                <div className="space-y-2">
                  <Label htmlFor="submission-file">Arquivo da solução</Label>
                  <Input
                    id="submission-file"
                    type="file"
                    accept={allowedExtensions?.join(",")}
                    onChange={handleFileChange}
                  />
                  {allowedExtensions && (
                    <p className="text-muted-foreground text-xs">
                      Tipos aceitos: {allowedExtensions.join(", ")}
                    </p>
                  )}
                  {fileError && (
                    <p className="text-destructive text-sm">{fileError}</p>
                  )}
                </div>

                {submitError && (
                  <p className="text-destructive text-sm">{submitError}</p>
                )}

                <Button type="submit" disabled={!selectedFile || submitting}>
                  <FileUp />
                  {submitting ? "Enviando…" : "Enviar para correção"}
                </Button>
              </form>
            </CardContent>
          </Card>
        ) : (
          <>
            <LiveResults
              status={status}
              history={history}
              compilation={compilation}
              results={results}
              final={final}
              streamError={streamError}
            />
            {final && (
              <Button variant="outline" onClick={handleNewSubmission}>
                Enviar outra solução
              </Button>
            )}
          </>
        )}
      </main>
      <Footer />
    </div>
  );
}
